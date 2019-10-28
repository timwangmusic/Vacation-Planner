package redis_client_mocks

import (
	"Vacation-planner/iowrappers"
	"github.com/alicebob/miniredis/v2"
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
	redisClient.Init(mockServer.Addr(), "", 0)

	geoCodeQuery := iowrappers.GeocodeQuery{
		City:    "New York City",
		Country: "US",
	}
	expectedLat := 40.7128
	expectedLng := -74.0060

	redisClient.SetGeocode(geoCodeQuery, expectedLat, expectedLng)

	lat, lng, exist := redisClient.GetGeocode(geoCodeQuery)

	if !exist || lat != expectedLat || lng != expectedLng {
		t.Errorf("geo-coding for %s fails",
			strings.Join([]string{geoCodeQuery.City, geoCodeQuery.Country}, ","))
	}
}
