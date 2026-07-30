package planner

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"github.com/weihesdlegend/Vacation-planner/test/redis_client_mocks"
	"github.com/weihesdlegend/Vacation-planner/user"
)

// The reclassify-buckets migration deletes members from the shared place cache that the
// trip-planning path reads, and a deleted bucket row is not re-created until the city's
// 14-day MapsLastSearchTime marker expires. `dryRun := ctx.Query("apply") != "true"` is
// therefore the last line of defence on the endpoint: every value other than the exact
// string "true" — including the truthy-looking "TRUE" and "1" — must keep the run read-only.
func TestReclassifyBucketsMigrationDryRunDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// point a real PoiSearcher at the same mock Redis the shared RedisClient uses, so the
	// handler exercises the whole path down to RemoveMisclassifiedPlacesFromCategoryBuckets
	redisURL, err := url.Parse("redis://" + redis_client_mocks.RedisMockSvr.Addr())
	if err != nil {
		t.Fatalf("failed to parse mock redis URL: %v", err)
	}
	p := &MyPlanner{
		RedisClient: redis_client_mocks.RedisClient,
		Solver:      Solver{Searcher: iowrappers.CreatePoiSearcher("test-maps-api-key", redisURL)},
	}

	router := gin.New()
	router.GET("/v1/migrate/reclassify-buckets", p.reclassifyBucketsMigrationHandler)

	adminToken := newAdminPAT(t, "reclassify_buckets_admin", "reclassify-buckets-admin-token")

	get := func(t *testing.T, query string) (int, map[string]any) {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/v1/migrate/reclassify-buckets"+query, nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		var body map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("failed to parse response %q: %v", w.Body.String(), err)
		}
		return w.Code, body
	}

	tests := []struct {
		name       string
		query      string
		wantDryRun bool
	}{
		{name: "apply absent", query: "?category=Eatery", wantDryRun: true},
		{name: "apply empty", query: "?category=Eatery&apply=", wantDryRun: true},
		{name: "apply uppercase TRUE", query: "?category=Eatery&apply=TRUE", wantDryRun: true},
		{name: "apply 1", query: "?category=Eatery&apply=1", wantDryRun: true},
		{name: "apply yes", query: "?category=Eatery&apply=yes", wantDryRun: true},
		{name: "no query at all", query: "", wantDryRun: true},
		// the one and only spelling that is allowed to delete
		{name: "apply exactly true", query: "?category=Eatery&apply=true", wantDryRun: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, body := get(t, tt.query)
			if code != http.StatusOK {
				t.Fatalf("status = %d, want %d (body: %+v)", code, http.StatusOK, body)
			}
			dryRun, ok := body["dry_run"].(bool)
			if !ok {
				t.Fatalf("response has no boolean dry_run field: %+v", body)
			}
			if dryRun != tt.wantDryRun {
				t.Errorf("dry_run = %v for %q, want %v", dryRun, tt.query, tt.wantDryRun)
			}
		})
	}
}

// TestReclassifyBucketsMigrationRequiresAdmin pins that the destructive endpoint is not
// reachable without admin credentials, so the dry-run default is not its only protection.
func TestReclassifyBucketsMigrationRequiresAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	redisURL, err := url.Parse("redis://" + redis_client_mocks.RedisMockSvr.Addr())
	if err != nil {
		t.Fatalf("failed to parse mock redis URL: %v", err)
	}
	p := &MyPlanner{
		RedisClient: redis_client_mocks.RedisClient,
		Solver:      Solver{Searcher: iowrappers.CreatePoiSearcher("test-maps-api-key", redisURL)},
	}

	router := gin.New()
	router.GET("/v1/migrate/reclassify-buckets", p.reclassifyBucketsMigrationHandler)

	get := func(authorization string) int {
		req := httptest.NewRequest(http.MethodGet, "/v1/migrate/reclassify-buckets?category=Eatery&apply=true", nil)
		if authorization != "" {
			req.Header.Set("Authorization", authorization)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w.Code
	}

	t.Run("no credentials", func(t *testing.T) {
		if code := get(""); code != http.StatusUnauthorized {
			t.Errorf("status = %d without credentials, want %d", code, http.StatusUnauthorized)
		}
	})

	t.Run("regular user PAT", func(t *testing.T) {
		token := newRegularPAT(t, "reclassify_buckets_regular", "reclassify-buckets-regular-token")
		if code := get("Bearer " + token); code != http.StatusUnauthorized {
			t.Errorf("status = %d for a non-admin PAT, want %d", code, http.StatusUnauthorized)
		}
	})
}

func newAdminPAT(t *testing.T, username, rawToken string) string {
	t.Helper()
	return newPATForLevel(t, username, rawToken, user.LevelStringAdmin)
}

func newRegularPAT(t *testing.T, username, rawToken string) string {
	t.Helper()
	return newPATForLevel(t, username, rawToken, user.LevelStringRegular)
}

func newPATForLevel(t *testing.T, username, rawToken, level string) string {
	t.Helper()
	userView, err := redis_client_mocks.RedisClient.CreateUser(
		redis_client_mocks.RedisContext,
		user.View{Username: username, Email: username + "@example.com", Password: "pwd", UserLevel: level},
		false,
	)
	if err != nil {
		t.Fatalf("failed to create %s test user: %v", level, err)
	}
	pat, err := redis_client_mocks.RedisClient.NewPAT(
		redis_client_mocks.RedisContext, username+"-pat", userView.ID, rawToken, time.Hour,
	)
	if err != nil {
		t.Fatalf("failed to create test PAT: %v", err)
	}
	return pat.TokenHash
}
