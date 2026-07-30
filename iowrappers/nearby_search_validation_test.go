package iowrappers

import (
	"strings"
	"testing"

	"github.com/weihesdlegend/Vacation-planner/POI"
)

// TestCreateMapSearchRequestRejectsUnknownPlaceType guards the fast_food_restaurant
// incident at the request boundary. The SDK casts POI.LocationType straight to
// maps.PlaceType and forwards it as ?type=, so an unknown value silently disables
// the filter server-side instead of erroring. Validate before spending the call.
func TestCreateMapSearchRequestRejectsUnknownPlaceType(t *testing.T) {
	req := &PlaceSearchRequest{
		Location:   POI.Location{Latitude: 37.38006, Longitude: -122.11612},
		PlaceCat:   POI.PlaceCategoryEatery,
		Radius:     8000,
		PriceLevel: POI.PriceLevelTwo,
	}
	for _, placeType := range []POI.LocationType{
		POI.LocationType("fast_food_restaurant"),
		POI.LocationType("food_court"),
		POI.LocationType("not_a_google_type"),
	} {
		if _, err := CreateMapSearchRequest(req, placeType, ""); err == nil {
			t.Errorf("CreateMapSearchRequest(%q) returned nil error, want validation failure", placeType)
		} else if !strings.Contains(err.Error(), string(placeType)) {
			t.Errorf("CreateMapSearchRequest(%q) error %q should name the offending type", placeType, err)
		}
	}
}

// TestCreateMapSearchRequestAcceptsKnownPlaceTypes pins that every type the
// categories actually search for still builds a request.
func TestCreateMapSearchRequestAcceptsKnownPlaceTypes(t *testing.T) {
	req := &PlaceSearchRequest{
		Location:   POI.Location{Latitude: 37.38006, Longitude: -122.11612},
		PlaceCat:   POI.PlaceCategoryEatery,
		Radius:     8000,
		PriceLevel: POI.PriceLevelTwo,
	}
	categories := []POI.PlaceCategory{
		POI.PlaceCategoryVisit, POI.PlaceCategoryEatery,
		POI.PlaceCategoryShopping, POI.PlaceCategoryLodging, POI.PlaceCategoryWellness,
	}
	for _, category := range categories {
		for _, placeType := range POI.GetPlaceTypes(category) {
			got, err := CreateMapSearchRequest(req, placeType, "")
			if err != nil {
				t.Errorf("CreateMapSearchRequest(%q) in category %q: unexpected error %v", placeType, category, err)
				continue
			}
			if string(got.Type) != string(placeType) {
				t.Errorf("CreateMapSearchRequest(%q) set Type=%q, want %q", placeType, got.Type, placeType)
			}
		}
	}
}

// TestCreateMapSearchRequestAcceptsAnyType pins that keyword (brand) searches,
// which intentionally leave the type unset, are not rejected.
func TestCreateMapSearchRequestAcceptsAnyType(t *testing.T) {
	req := &PlaceSearchRequest{
		Location: POI.Location{Latitude: 37.38006, Longitude: -122.11612},
		PlaceCat: POI.PlaceCategoryEatery,
		Radius:   8000,
		Keyword:  "Dunkin'",
	}
	got, err := CreateMapSearchRequest(req, POI.LocationTypeAny, "")
	if err != nil {
		t.Fatalf("CreateMapSearchRequest(LocationTypeAny) returned error %v, want nil", err)
	}
	if got.Type != "" {
		t.Errorf("CreateMapSearchRequest(LocationTypeAny) set Type=%q, want empty", got.Type)
	}
	if got.Keyword != "Dunkin'" {
		t.Errorf("CreateMapSearchRequest kept Keyword=%q, want %q", got.Keyword, "Dunkin'")
	}
}
