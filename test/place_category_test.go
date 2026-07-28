package test

import (
	"testing"

	"github.com/weihesdlegend/Vacation-planner/POI"
)

// TestGetPlaceTypesByCategory pins the Google Maps place types each category expands to.
func TestGetPlaceTypesByCategory(t *testing.T) {
	cases := map[POI.PlaceCategory][]POI.LocationType{
		POI.PlaceCategoryEatery: {POI.LocationTypeCafe, POI.LocationTypeRestaurant},
		POI.PlaceCategoryShopping: {
			POI.LocationTypeShoppingMall, POI.LocationTypeDepartmentStore,
			POI.LocationTypeSupermarket, POI.LocationTypeClothingStore, POI.LocationTypeStore,
		},
		POI.PlaceCategoryLodging:  {POI.LocationTypeLodging},
		POI.PlaceCategoryWellness: {POI.LocationTypeGym, POI.LocationTypeSpa, POI.LocationTypePharmacy},
	}
	for category, want := range cases {
		got := POI.GetPlaceTypes(category)
		if len(got) != len(want) {
			t.Fatalf("category %s: got %d place types %v, want %d %v", category, len(got), got, len(want), want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("category %s: place type[%d] = %s, want %s", category, i, got[i], want[i])
			}
		}
	}
}

// TestPlaceCategoryRoundTrip is the cache-correctness invariant: every place type a category
// searches for must map back to that same category via GetPlaceCategory. If it does not, the
// nearby-search cache writes places under one Redis key and reads them under another, so the
// category can never cache-hit. Covers all searchable categories, new and existing.
func TestPlaceCategoryRoundTrip(t *testing.T) {
	categories := []POI.PlaceCategory{
		POI.PlaceCategoryVisit, POI.PlaceCategoryEatery,
		POI.PlaceCategoryShopping, POI.PlaceCategoryLodging, POI.PlaceCategoryWellness,
	}
	for _, category := range categories {
		for _, placeType := range POI.GetPlaceTypes(category) {
			if got := POI.GetPlaceCategory(placeType); got != category {
				t.Errorf("round-trip broken: GetPlaceCategory(%q) = %q, want %q", placeType, got, category)
			}
		}
	}
}

func TestParsePlaceCategory(t *testing.T) {
	valid := []string{"Visit", "Eatery", "Shopping", "Lodging", "Wellness"}
	for _, s := range valid {
		if parsed, ok := POI.ParsePlaceCategory(s); !ok || string(parsed) != s {
			t.Errorf("ParsePlaceCategory(%q) = (%q, %v), want (%q, true)", s, parsed, ok, s)
		}
	}
	for _, s := range []string{"", "shopping", "Restaurant", "food"} {
		if _, ok := POI.ParsePlaceCategory(s); ok {
			t.Errorf("ParsePlaceCategory(%q) accepted an unknown category", s)
		}
	}
}

// TestEncodeNearbySearchRedisKeyDistinct guards against two categories sharing a Redis geo
// bucket, which would cross-contaminate their cached results.
func TestEncodeNearbySearchRedisKeyDistinct(t *testing.T) {
	categories := []POI.PlaceCategory{
		POI.PlaceCategoryVisit, POI.PlaceCategoryEatery,
		POI.PlaceCategoryShopping, POI.PlaceCategoryLodging, POI.PlaceCategoryWellness,
	}
	seen := make(map[string]POI.PlaceCategory)
	for _, category := range categories {
		key := POI.EncodeNearbySearchRedisKey(category, POI.PriceLevelDefault)
		if other, dup := seen[key]; dup {
			t.Errorf("categories %s and %s share Redis key %q", other, category, key)
		}
		seen[key] = category
	}
}
