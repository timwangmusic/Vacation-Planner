package redis_client_mocks

import (
	"strings"
	"testing"

	"github.com/weihesdlegend/Vacation-planner/iowrappers"
)

func TestGeoCodingCache(t *testing.T) {
	geoCodeQuery := iowrappers.GeocodeQuery{
		City:              "New York City",
		AdminAreaLevelOne: "New York",
		Country:           "US",
	}
	expectedLat := 40.7128
	expectedLng := -74.0060

	RedisClient.SetGeocode(RedisContext, geoCodeQuery, expectedLat, expectedLng, geoCodeQuery)

	lat, lng, geocodeMissingErr := RedisClient.Geocode(RedisContext, &geoCodeQuery)

	if geocodeMissingErr != nil {
		t.Errorf("geo-coding for %s fails, error is %s",
			strings.Join([]string{geoCodeQuery.City, geoCodeQuery.Country}, ","), geocodeMissingErr.Error())
		return
	}

	if lat != expectedLat {
		t.Errorf("expected lat %.4f got %.4f", expectedLat, lat)
		return
	}

	if lng != expectedLng {
		t.Errorf("expected lng %.4f got %.4f", expectedLng, lng)
	}
}
