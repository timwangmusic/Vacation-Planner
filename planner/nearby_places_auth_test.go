package planner

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/weihesdlegend/Vacation-planner/test/redis_client_mocks"
	"github.com/weihesdlegend/Vacation-planner/user"
)

// The /v1/nearby-places handler must reject unauthenticated requests before
// doing any work: each request can fan out a reverse geocode plus up to 25
// Google Maps searches, so the endpoint cannot be open.
func TestGetNearbyPlacesRequiresAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	p := &MyPlanner{RedisClient: redis_client_mocks.RedisClient}
	router := gin.New()
	router.POST("/v1/nearby-places", p.getNearbyPlaces)

	// zero location: with valid auth this fails request validation (400),
	// proving the request got past the auth check without needing geocoding
	body := `{"brands": ["Dunkin'"], "location": {"latitude": 0, "longitude": 0}}`

	post := func(authorization string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/v1/nearby-places", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if authorization != "" {
			req.Header.Set("Authorization", authorization)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}

	t.Run("no credentials", func(t *testing.T) {
		if w := post(""); w.Code != http.StatusUnauthorized {
			t.Errorf("expected %d without credentials, got %d (%s)", http.StatusUnauthorized, w.Code, w.Body.String())
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		if w := post("Bearer not-a-real-token"); w.Code != http.StatusUnauthorized {
			t.Errorf("expected %d with an invalid token, got %d (%s)", http.StatusUnauthorized, w.Code, w.Body.String())
		}
	})

	t.Run("valid PAT reaches request validation", func(t *testing.T) {
		userView, err := redis_client_mocks.RedisClient.CreateUser(
			redis_client_mocks.RedisContext,
			user.View{Username: "nearby_places_svc", Email: "nearby_places_svc@example.com", Password: "pwd", UserLevel: user.LevelStringRegular},
			false,
		)
		if err != nil {
			t.Fatalf("failed to create test user: %v", err)
		}
		pat, err := redis_client_mocks.RedisClient.NewPAT(
			redis_client_mocks.RedisContext, "nearby-places-test", userView.ID, "nearby-places-test-token", time.Hour,
		)
		if err != nil {
			t.Fatalf("failed to create test PAT: %v", err)
		}

		if w := post("Bearer " + pat.TokenHash); w.Code != http.StatusBadRequest {
			t.Errorf("expected %d (past auth, zero location rejected), got %d (%s)", http.StatusBadRequest, w.Code, w.Body.String())
		}
	})
}
