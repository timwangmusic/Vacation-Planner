package redis_client_mocks

import (
	"github.com/alicebob/miniredis/v2"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"net/url"
	"testing"
	"time"
)

func TestMapsLastSearchTime (t *testing.T) {
	// set up mock server
	mockServer, err := miniredis.Run()
	if err != nil {
		panic(err)
	}
	defer mockServer.Close()

	// client initialization
	redisClient := iowrappers.RedisClient{}
	redisUrl := "redis://" + mockServer.Addr()
	redisURL, _ := url.Parse(redisUrl)
	redisClient.Init(redisURL)

	request := iowrappers.PlaceSearchRequest{
		Location:      "Beijing,China",
		PlaceCat:      "Visit",
	}

	currentTime := time.Now().Format(time.RFC3339)
	_ = redisClient.CacheMapsLastSearchTime(request, currentTime)

	cachedTime, err := redisClient.GetMapsLastSearchTime(request)

	if err != nil || currentTime != cachedTime.Format(time.RFC3339) {
		t.Error("maps cached time retrieval failure")
		if err != nil {
			t.Error(err)
		} else {
			t.Errorf("expected cached time %s, got %s", currentTime, cachedTime.Format(time.RFC3339))
		}
	}
}
