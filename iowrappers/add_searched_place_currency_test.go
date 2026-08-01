package iowrappers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/weihesdlegend/Vacation-planner/POI"
	"googlemaps.github.io/maps"
)

// These tests pin when a confirm buys a Place Details call. A Details call is the single most
// expensive Google request the service makes (max field tier), and a re-confirm of a place whose
// stored record already carries current Details-sourced fields holds all the data the confirm
// needs — the same placeDetailsAreCurrent rule the nearby-search path already trusts.

func countingEnricher(calls *int) placeDetailsEnricher {
	return func(ctx context.Context, placeID string) (maps.PlaceDetailsResult, error) {
		*calls++
		return maps.PlaceDetailsResult{}, errors.New("no network in this test")
	}
}

func TestConfirmSkipsDetailsWhenCachedRecordIsCurrent(t *testing.T) {
	s, ctx := newAddSearchedPlaceFixture(t)

	placeID := "museum-current"
	stored := POI.Place{
		ID:            placeID,
		Name:          "City History Museum",
		LocationType:  POI.LocationType("museum"),
		Location:      POI.Location{Latitude: 37.4, Longitude: -122.1},
		URL:           "https://maps.google.com/?cid=42", // proof a Details call landed (detailsSourcedFields)
		LastUpdatedAt: time.Now().Format(time.RFC3339),
	}
	s.redisClient.SetPlacesAddGeoLocations(ctx, []POI.Place{stored})

	stashCandidate(t, s, ctx, POI.Place{
		ID:       placeID,
		Name:     "City History Museum",
		Types:    []string{"museum", "point_of_interest", "establishment"},
		Location: POI.Location{Latitude: 37.4, Longitude: -122.1},
	})

	calls := 0
	result, err := s.addSearchedPlaceToCache(ctx, placeID, countingEnricher(&calls))
	if err != nil {
		t.Fatalf("addSearchedPlaceToCache: %v", err)
	}
	if calls != 0 {
		t.Errorf("enricher called %d times for a current cached record, want 0 — this is a billed Place Details call", calls)
	}
	if !result.AlreadyCached {
		t.Error("AlreadyCached = false, want true")
	}
	if result.Place.URL != stored.URL {
		t.Errorf("Place.URL = %q, want the cached record's %q restored", result.Place.URL, stored.URL)
	}
}

func TestConfirmBuysDetailsWhenCachedRecordIsStale(t *testing.T) {
	s, ctx := newAddSearchedPlaceFixture(t)

	placeID := "museum-stale"
	stale := time.Now().Add(-(PlaceDetailsRefreshDuration + 24*time.Hour))
	s.redisClient.SetPlacesAddGeoLocations(ctx, []POI.Place{{
		ID:            placeID,
		Name:          "Old Museum",
		LocationType:  POI.LocationType("museum"),
		Location:      POI.Location{Latitude: 37.4, Longitude: -122.1},
		URL:           "https://maps.google.com/?cid=43",
		LastUpdatedAt: stale.Format(time.RFC3339),
	}})

	stashCandidate(t, s, ctx, POI.Place{
		ID:       placeID,
		Name:     "Old Museum",
		Types:    []string{"museum", "point_of_interest", "establishment"},
		Location: POI.Location{Latitude: 37.4, Longitude: -122.1},
	})

	calls := 0
	if _, err := s.addSearchedPlaceToCache(ctx, placeID, countingEnricher(&calls)); err != nil {
		t.Fatalf("addSearchedPlaceToCache: %v", err)
	}
	if calls != 1 {
		t.Errorf("enricher called %d times for a stale cached record, want 1", calls)
	}
}

func TestConfirmBuysDetailsWhenNotCached(t *testing.T) {
	s, ctx := newAddSearchedPlaceFixture(t)

	placeID := "museum-uncached"
	stashCandidate(t, s, ctx, POI.Place{
		ID:       placeID,
		Name:     "Brand New Museum",
		Types:    []string{"museum", "point_of_interest", "establishment"},
		Location: POI.Location{Latitude: 37.4, Longitude: -122.1},
	})

	calls := 0
	if _, err := s.addSearchedPlaceToCache(ctx, placeID, countingEnricher(&calls)); err != nil {
		t.Fatalf("addSearchedPlaceToCache: %v", err)
	}
	if calls != 1 {
		t.Errorf("enricher called %d times for an uncached place, want 1", calls)
	}
}
