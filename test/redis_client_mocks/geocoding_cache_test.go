package redis_client_mocks

import (
	"github.com/alicebob/miniredis/v2"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"net/url"
	"strings"
	"testing"
)

func TestGeoCodingCache(t *testing.T) {
	// set up
	mockServer, err := miniredis.Run()
	if err != nil {
		panic(err)
	}
	defer mockServer.Close()

	redisClient := iowrappers.RedisClient{}
	redisUrl := "redis://" + mockServer.Addr()
	redisURL, _ := url.Parse(redisUrl)
	redisClient.Init(redisURL)

	geoCodeQuery := iowrappers.GeocodeQuery{
		City:    "New York City",
		Country: "US",
	}
	expectedLat := 40.7128
	expectedLng := -74.0060

	redisClient.SetGeocode(geoCodeQuery, expectedLat, expectedLng, geoCodeQuery)

	lat, lng, exist := redisClient.GetGeocode(&geoCodeQuery)

	if !exist || lat != expectedLat || lng != expectedLng {
		t.Errorf("geo-coding for %s fails",
			strings.Join([]string{geoCodeQuery.City, geoCodeQuery.Country}, ","))
	}
}
