package iowrappers

import (
	"testing"

	"github.com/weihesdlegend/Vacation-planner/POI"
)

func placeAt(id string, lat, lng float64) POI.Place {
	var p POI.Place
	p.SetID(id)
	p.SetLocationCoordinates([2]float64{lat, lng})
	return p
}

// TestSortPlacesByDistance pins that truncation keeps the NEAREST places. The fresh
// (Google) path returns results grouped by place type in prominence order, so slicing
// without sorting first drops whole place types and can rank a 3km result above a 250m one.
func TestSortPlacesByDistance(t *testing.T) {
	// State Street Market, Los Altos
	lat, lng := 37.38006, -122.11612

	places := []POI.Place{
		placeAt("far-sunnyvale", 37.3688, -122.0363),    // ~7km east
		placeAt("mid-mountainview", 37.3861, -122.0839), // ~3km east
		placeAt("near-state-st", 37.38025, -122.11655),  // ~40m away
	}

	SortPlacesByDistance(places, lat, lng)

	want := []string{"near-state-st", "mid-mountainview", "far-sunnyvale"}
	for i, id := range want {
		if places[i].GetID() != id {
			t.Errorf("position %d = %q, want %q (full order: %v)", i, places[i].GetID(), id, placeIDs(places))
		}
	}
}

// TestSortPlacesByDistanceEmpty pins that the no-result case does not panic.
func TestSortPlacesByDistanceEmpty(t *testing.T) {
	var places []POI.Place
	SortPlacesByDistance(places, 37.38006, -122.11612)
	if len(places) != 0 {
		t.Errorf("got %d places, want 0", len(places))
	}
}

func placeIDs(places []POI.Place) []string {
	ids := make([]string, 0, len(places))
	for _, p := range places {
		ids = append(ids, p.GetID())
	}
	return ids
}
