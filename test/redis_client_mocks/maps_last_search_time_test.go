package redis_client_mocks

import (
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"testing"
	"time"
)

func TestMapsLastSearchTime(t *testing.T) {
	request := iowrappers.PlaceSearchRequest{
		Location: POI.Location{City: "Beijing", Country: "China"},
		PlaceCat: "Visit",
	}

	currentTime := time.Now().Format(time.RFC3339)
	_ = RedisClient.SetMapsLastSearchTime(RedisContext, request.Location, request.PlaceCat, currentTime)

	cachedTime, err := RedisClient.GetMapsLastSearchTime(RedisContext, request.Location, request.PlaceCat)

	if err != nil || currentTime != cachedTime.Format(time.RFC3339) {
		t.Error("maps cached time retrieval failure")
		if err != nil {
			t.Error(err)
		} else {
			t.Errorf("expected cached time %s, got %s", currentTime, cachedTime.Format(time.RFC3339))
		}
	}
}
