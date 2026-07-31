package planner

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"github.com/weihesdlegend/Vacation-planner/test/redis_client_mocks"
)

// newPlaceSearchTestPlanner builds a MyPlanner wired to the shared mock Redis, with a real
// PoiSearcher pointed at the same backing miniredis instance (see
// TestReclassifyBucketsMigrationDryRunDefault for the established pattern). No real Google Maps
// calls are exercised by any test in this file: every case either fails auth/validation before the
// handler would call out, or exercises AddSearchedPlaceToCache purely against the stashed-candidate
// Redis path.
func newPlaceSearchTestPlanner(t *testing.T) *MyPlanner {
	t.Helper()
	redisURL, err := url.Parse("redis://" + redis_client_mocks.RedisMockSvr.Addr())
	if err != nil {
		t.Fatalf("failed to parse mock redis URL: %v", err)
	}
	return &MyPlanner{
		RedisClient: redis_client_mocks.RedisClient,
		Solver:      Solver{Searcher: iowrappers.CreatePoiSearcher("test-maps-api-key", redisURL)},
	}
}

func newPlaceSearchTestRouter(p *MyPlanner) *gin.Engine {
	router := gin.New()
	router.POST("/v1/place-search", p.searchPlacesByText)
	router.POST("/v1/place-search/confirm", p.confirmSearchedPlace)
	return router
}

func doPlaceSearchRequest(router *gin.Engine, method, path, authorization, body string) (int, map[string]any) {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var respBody map[string]any
	// Not every case (e.g. malformed-auth 401s) is guaranteed to emit JSON in every possible gin
	// configuration, but every handler path in this file always calls ctx.JSON, so this should
	// always succeed; ignore unmarshal errors here and let individual assertions on w.Code/body
	// surface any real problem.
	_ = json.Unmarshal(w.Body.Bytes(), &respBody)
	return w.Code, respBody
}

// TestPlaceSearchRoutesRequireAuth pins that both routes reject unauthenticated and
// garbage-token requests before doing any work (case 1 and case 2 from the task brief).
func TestPlaceSearchRoutesRequireAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	p := newPlaceSearchTestPlanner(t)
	router := newPlaceSearchTestRouter(p)

	routes := []struct {
		name string
		path string
		body string
	}{
		{name: "search", path: "/v1/place-search", body: `{"query":"konjoe"}`},
		{name: "confirm", path: "/v1/place-search/confirm", body: `{"placeId":"some-id"}`},
	}

	for _, rt := range routes {
		t.Run(rt.name+"/no credentials", func(t *testing.T) {
			if code, body := doPlaceSearchRequest(router, http.MethodPost, rt.path, "", rt.body); code != http.StatusUnauthorized {
				t.Errorf("expected %d without credentials, got %d (%v)", http.StatusUnauthorized, code, body)
			}
		})
		t.Run(rt.name+"/garbage token", func(t *testing.T) {
			if code, body := doPlaceSearchRequest(router, http.MethodPost, rt.path, "Bearer not-a-real-token", rt.body); code != http.StatusUnauthorized {
				t.Errorf("expected %d with an invalid token, got %d (%v)", http.StatusUnauthorized, code, body)
			}
		})
	}
}

// TestSearchPlacesByTextValidation exercises the /v1/place-search request validation, all with a
// valid regular-user PAT: binding failures (too-short query, missing query) and the zero-location
// rejection (case 3, 4, 5 from the task brief). The zero-location case doubles as proof the
// handler validates before ever reaching TextSearchPlaces / a Google call.
func TestSearchPlacesByTextValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	p := newPlaceSearchTestPlanner(t)
	router := newPlaceSearchTestRouter(p)
	token := newRegularPAT(t, "place_search_regular", "place-search-regular-token")
	auth := "Bearer " + token

	tests := []struct {
		name string
		body string
	}{
		{name: "query too short", body: `{"query":"x"}`},
		{name: "missing query", body: `{"location":{"latitude":37.0,"longitude":-122.0}}`},
		{name: "zero location", body: `{"query":"konjoe","location":{"latitude":0,"longitude":0}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, body := doPlaceSearchRequest(router, http.MethodPost, "/v1/place-search", auth, tt.body)
			if code != http.StatusBadRequest {
				t.Errorf("expected %d, got %d (%v)", http.StatusBadRequest, code, body)
			}
			if _, ok := body["error"]; !ok {
				t.Errorf("expected an \"error\" field in response body, got %v", body)
			}
		})
	}
}

// TestConfirmSearchedPlaceValidation exercises /v1/place-search/confirm: a candidate ID that was
// never stashed (case 6), a missing placeId (case 7), and the 422 unsupported-place-type refusal
// end to end (case 8). All three run past authentication with a valid regular-user PAT.
func TestConfirmSearchedPlaceValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	p := newPlaceSearchTestPlanner(t)
	router := newPlaceSearchTestRouter(p)
	token := newRegularPAT(t, "place_search_confirm_regular", "place-search-confirm-regular-token")
	auth := "Bearer " + token

	t.Run("candidate never stashed", func(t *testing.T) {
		code, body := doPlaceSearchRequest(router, http.MethodPost, "/v1/place-search/confirm", auth,
			`{"placeId":"nope-never-stashed"}`)
		if code != http.StatusNotFound {
			t.Fatalf("expected %d, got %d (%v)", http.StatusNotFound, code, body)
		}
		if body["code"] != "candidate_expired" {
			t.Errorf(`expected code "candidate_expired", got %v (full body: %v)`, body["code"], body)
		}
		if _, ok := body["error"]; !ok {
			t.Errorf("expected an \"error\" field in response body, got %v", body)
		}
	})

	t.Run("missing placeId", func(t *testing.T) {
		code, body := doPlaceSearchRequest(router, http.MethodPost, "/v1/place-search/confirm", auth, `{}`)
		if code != http.StatusBadRequest {
			t.Errorf("expected %d, got %d (%v)", http.StatusBadRequest, code, body)
		}
	})

	t.Run("unsupported place type", func(t *testing.T) {
		const placeID = "place-search-confirm-unsupported-type"
		// "roofing_contractor" is a real Google Maps place type that maps to no
		// POI.PlaceCategory (see POI/categories.go's placeTypeToCategory) and is not an
		// umbrella type PrimaryLocationType would skip over, so it is a faithful stand-in
		// for "the candidate's primary type is unmapped."
		candidate := POI.Place{
			ID:       placeID,
			Name:     "Roofing Co",
			Types:    []string{"roofing_contractor", "point_of_interest", "establishment"},
			Location: POI.Location{Latitude: 37.0, Longitude: -122.0},
		}
		if err := redis_client_mocks.RedisClient.SetPlaceSearchCandidate(redis_client_mocks.RedisContext, candidate, iowrappers.PlaceSearchCandidateTTL); err != nil {
			t.Fatalf("failed to stash test candidate: %v", err)
		}
		t.Cleanup(func() {
			_ = redis_client_mocks.RedisClient.RemoveKeys(redis_client_mocks.RedisContext,
				[]string{iowrappers.PlaceSearchCandidateRedisKeyPrefix + placeID})
		})

		code, body := doPlaceSearchRequest(router, http.MethodPost, "/v1/place-search/confirm", auth,
			`{"placeId":"`+placeID+`"}`)
		if code != http.StatusUnprocessableEntity {
			t.Fatalf("expected %d, got %d (%v)", http.StatusUnprocessableEntity, code, body)
		}
		if body["code"] != "unsupported_place_type" {
			t.Errorf(`expected code "unsupported_place_type", got %v (full body: %v)`, body["code"], body)
		}
		if body["placeType"] != "roofing_contractor" {
			t.Errorf(`expected placeType "roofing_contractor", got %v (full body: %v)`, body["placeType"], body)
		}
		if _, ok := body["error"]; !ok {
			t.Errorf("expected an \"error\" field in response body, got %v", body)
		}
	})
}
