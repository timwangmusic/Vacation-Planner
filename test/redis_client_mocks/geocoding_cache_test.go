package redis_client_mocks

import (
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"strings"
	"testing"
)

func TestGeoCodingCache(t *testing.T) {
	geoCodeQuery := iowrappers.GeocodeQuery{
		City:    "New York City",
		Country: "US",
	}
	expectedLat := 40.7128
	expectedLng := -74.0060

	_ = iowrappers.CreateLogger()

	RedisClient.SetGeocode(RedisContext, geoCodeQuery, expectedLat, expectedLng, geoCodeQuery)

	lat, lng, geocodeMissingErr := RedisClient.GetGeocode(RedisContext, &geoCodeQuery)

	if geocodeMissingErr != nil || lat != expectedLat || lng != expectedLng {
		t.Errorf("geo-coding for %s fails",
			strings.Join([]string{geoCodeQuery.City, geoCodeQuery.Country}, ","))
	}
}
