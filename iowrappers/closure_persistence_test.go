package iowrappers

import (
	"testing"

	"github.com/weihesdlegend/Vacation-planner/POI"
)

// Pins that a cold search PERSISTS permanently-closed places while still excluding them from the
// response. The previous behavior filtered non-Operational results BEFORE the cache write, which
// discarded the closure signal entirely: the stale record kept Status OPERATIONAL forever and the
// place kept being served from cache. Persisting the closure lets the read-side Operational
// filters (RedisClient.NearbySearch) retire the place everywhere, at zero extra API cost.
func TestColdSearchPersistsClosuresButExcludesThemFromResponse(t *testing.T) {
	s, ctx := newAddSearchedPlaceFixture(t)

	open := POI.Place{
		ID:           "open-1",
		Name:         "Open Diner",
		LocationType: POI.LocationType("restaurant"),
		Status:       POI.Operational,
		Location:     POI.Location{Latitude: 37.4, Longitude: -122.1},
	}
	closed := POI.Place{
		ID:           "closed-1",
		Name:         "Shuttered Grill",
		LocationType: POI.LocationType("restaurant"),
		Status:       POI.ClosedPermanently,
		Location:     POI.Location{Latitude: 37.41, Longitude: -122.11},
	}
	req := &PlaceSearchRequest{
		PlaceCat:       POI.PlaceCategoryEatery,
		BusinessStatus: POI.Operational,
		Location:       POI.Location{Latitude: 37.4, Longitude: -122.1},
	}

	got := s.persistAndFilterSearchResults(ctx, req, []POI.Place{open, closed})

	if len(got) != 1 || got[0].ID != "open-1" {
		t.Fatalf("response = %+v, want only open-1 (closed places excluded from the response)", got)
	}

	cached, err := s.redisClient.CachedPlaces(ctx, []string{"open-1", "closed-1"})
	if err != nil {
		t.Fatalf("CachedPlaces: %v", err)
	}
	if _, ok := cached["open-1"]; !ok {
		t.Error("open-1 was not persisted")
	}
	stored, ok := cached["closed-1"]
	if !ok {
		t.Fatal("closed-1 was not persisted — the closure signal was discarded, the pre-write-filter bug")
	}
	if stored.Status != POI.ClosedPermanently {
		t.Errorf("closed-1 Status = %q, want %q recorded", stored.Status, POI.ClosedPermanently)
	}

	// The read path must retire it: an Operational-filtered cache read excludes the closed place.
	readReq := &PlaceSearchRequest{
		PlaceCat:       POI.PlaceCategoryEatery,
		BusinessStatus: POI.Operational,
		Location:       POI.Location{Latitude: 37.4, Longitude: -122.1},
		Radius:         1000,
		MinNumResults:  1,
	}
	fromCache, err := s.redisClient.NearbySearch(ctx, readReq)
	if err != nil {
		t.Fatalf("RedisClient.NearbySearch: %v", err)
	}
	for _, p := range fromCache {
		if p.ID == "closed-1" {
			t.Error("closed-1 served from an Operational-filtered cache read")
		}
	}
}

func TestPersistAndFilterWithoutStatusFilterReturnsEverything(t *testing.T) {
	s, ctx := newAddSearchedPlaceFixture(t)

	places := []POI.Place{
		{ID: "a", Name: "A", LocationType: POI.LocationType("restaurant"), Status: POI.Operational, Location: POI.Location{Latitude: 37.4, Longitude: -122.1}},
		{ID: "b", Name: "B", LocationType: POI.LocationType("restaurant"), Status: POI.ClosedPermanently, Location: POI.Location{Latitude: 37.41, Longitude: -122.11}},
	}
	req := &PlaceSearchRequest{
		PlaceCat: POI.PlaceCategoryEatery,
		Location: POI.Location{Latitude: 37.4, Longitude: -122.1},
	}

	got := s.persistAndFilterSearchResults(ctx, req, places)
	if len(got) != 2 {
		t.Fatalf("response has %d places, want 2 — no BusinessStatus filter was requested", len(got))
	}
}
