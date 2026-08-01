package iowrappers

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/weihesdlegend/Vacation-planner/POI"
)

// newReverseGeocodeFixture builds a RedisClient backed by its own miniredis instance, mirroring
// add_searched_place_test.go's own-server-per-test harness.
func newReverseGeocodeFixture(t *testing.T) (*RedisClient, *miniredis.Miniredis, context.Context) {
	t.Helper()
	svr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	t.Cleanup(svr.Close)

	redisURL, _ := url.Parse("redis://" + svr.Addr())
	if err := CreateLogger(); err != nil {
		t.Fatalf("CreateLogger: %v", err)
	}
	return CreateRedisClient(redisURL), svr, context.Background()
}

func TestReverseGeocodeCacheRoundTrip(t *testing.T) {
	r, _, ctx := newReverseGeocodeFixture(t)
	want := GeocodeQuery{City: "Mountain View", AdminAreaLevelOne: "CA", Country: "US"}

	r.SetReverseGeocode(ctx, 37.4001, -122.0801, want)

	// Nearby coordinates land in the same ~8 km search cell and must hit.
	got, err := r.ReverseGeocode(ctx, 37.4002, -122.0802)
	if err != nil {
		t.Fatalf("ReverseGeocode after Set: %v", err)
	}
	if *got != want {
		t.Fatalf("ReverseGeocode = %+v, want %+v", *got, want)
	}
}

func TestReverseGeocodeCacheMiss(t *testing.T) {
	r, _, ctx := newReverseGeocodeFixture(t)

	if _, err := r.ReverseGeocode(ctx, 37.4, -122.08); err == nil {
		t.Fatal("ReverseGeocode on an empty cache must return an error")
	}

	// A hit in one cell must not leak into another cell.
	r.SetReverseGeocode(ctx, 37.4, -122.08, GeocodeQuery{City: "Mountain View", Country: "US"})
	if _, err := r.ReverseGeocode(ctx, 40.7, -74.0); err == nil {
		t.Fatal("ReverseGeocode for a different cell must miss")
	}
}

func TestReverseGeocodeCacheExpires(t *testing.T) {
	r, svr, ctx := newReverseGeocodeFixture(t)
	r.SetReverseGeocode(ctx, 37.4, -122.08, GeocodeQuery{City: "Mountain View", Country: "US"})

	svr.FastForward(ReverseGeocodeExpiration + time.Hour)

	if _, err := r.ReverseGeocode(ctx, 37.4, -122.08); err == nil {
		t.Fatal("ReverseGeocode must miss after the cache entry expires")
	}
}

// A cached cell must satisfy PoiSearcher.ReverseGeocode without any Google call: the fixture's
// mapsClient is nil, so reaching for Google would panic — returning the cached value is the proof
// the cache is consulted first. This is the call every warm nearby scan makes.
func TestPoiSearcherReverseGeocodeServesFromCache(t *testing.T) {
	r, _, ctx := newReverseGeocodeFixture(t)
	s := &PoiSearcher{redisClient: r}
	want := GeocodeQuery{City: "Los Altos", AdminAreaLevelOne: "CA", Country: "US"}
	r.SetReverseGeocode(ctx, 37.379, -122.117, want)

	got, err := s.ReverseGeocode(ctx, 37.379, -122.117)
	if err != nil {
		t.Fatalf("ReverseGeocode: %v", err)
	}
	if *got != want {
		t.Fatalf("ReverseGeocode = %+v, want %+v", *got, want)
	}
}

// processLocation's precise-location branch is the nearby-scan entry point for the reverse
// geocode — it must go through the searcher's cached path, not straight to the maps client.
func TestProcessLocationPreciseUsesCachedReverseGeocode(t *testing.T) {
	r, _, ctx := newReverseGeocodeFixture(t)
	s := &PoiSearcher{redisClient: r}
	r.SetReverseGeocode(ctx, 37.379, -122.117, GeocodeQuery{City: "Los Altos", AdminAreaLevelOne: "CA", Country: "US"})

	req := &PlaceSearchRequest{
		Location:           POI.Location{Latitude: 37.379, Longitude: -122.117},
		UsePreciseLocation: true,
	}
	if err := s.processLocation(ctx, req); err != nil {
		t.Fatalf("processLocation: %v", err)
	}
	if req.Location.City != "Los Altos" || req.Location.Country != "US" {
		t.Fatalf("processLocation resolved %+v, want cached Los Altos/US", req.Location)
	}
}
