package test

import (
	"testing"

	"github.com/weihesdlegend/Vacation-planner/POI"
)

// TestGetPlaceTypesByCategory pins the Google Maps place types each category expands to.
func TestGetPlaceTypesByCategory(t *testing.T) {
	cases := map[POI.PlaceCategory][]POI.LocationType{
		POI.PlaceCategoryEatery: {
			POI.LocationTypeCafe, POI.LocationTypeRestaurant,
			POI.LocationTypeBar, POI.LocationTypeBakery, POI.LocationTypeMealTakeaway,
		},
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

func TestPrimaryLocationType(t *testing.T) {
	cases := []struct {
		name  string
		types []string
		want  POI.LocationType
	}{
		{"restaurant first", []string{"restaurant", "food", "point_of_interest", "establishment"}, POI.LocationTypeRestaurant},
		{"supermarket before secondary restaurant", []string{"supermarket", "grocery_or_supermarket", "food", "store", "point_of_interest"}, POI.LocationTypeSupermarket},
		{"skips leading umbrella types", []string{"food", "point_of_interest", "cafe", "establishment"}, POI.LocationTypeCafe},
		{"cinema", []string{"movie_theater", "point_of_interest", "establishment"}, POI.LocationType("movie_theater")},
		{"empty -> unknown", nil, POI.LocationType("")},
		{"all umbrella -> unknown", []string{"point_of_interest", "establishment"}, POI.LocationType("")},
	}
	for _, tc := range cases {
		if got := POI.PrimaryLocationType(tc.types); got != tc.want {
			t.Errorf("%s: PrimaryLocationType(%v) = %q, want %q", tc.name, tc.types, got, tc.want)
		}
	}
}

func TestReclassifyForCategory(t *testing.T) {
	t.Run("supermarket returned by the food search is dropped from Eatery", func(t *testing.T) {
		p := POI.Place{LocationType: POI.LocationTypeRestaurant, Types: []string{"supermarket", "grocery_or_supermarket", "food", "store"}}
		if _, keep := POI.ReclassifyForCategory(p, POI.PlaceCategoryEatery); keep {
			t.Error("expected a supermarket to be dropped from the Eatery category")
		}
	})

	t.Run("same supermarket is kept under Shopping, tagged supermarket", func(t *testing.T) {
		p := POI.Place{LocationType: POI.LocationTypeSupermarket, Types: []string{"supermarket", "grocery_or_supermarket", "food", "store"}}
		rp, keep := POI.ReclassifyForCategory(p, POI.PlaceCategoryShopping)
		if !keep {
			t.Fatal("expected a supermarket to be kept under Shopping")
		}
		if rp.LocationType != POI.LocationTypeSupermarket {
			t.Errorf("LocationType = %q, want supermarket", rp.LocationType)
		}
	})

	t.Run("real restaurant found via cafe search is re-tagged restaurant and kept", func(t *testing.T) {
		p := POI.Place{LocationType: POI.LocationTypeCafe, Types: []string{"restaurant", "food", "point_of_interest"}}
		rp, keep := POI.ReclassifyForCategory(p, POI.PlaceCategoryEatery)
		if !keep {
			t.Fatal("expected a restaurant to be kept in Eatery")
		}
		if rp.LocationType != POI.LocationTypeRestaurant {
			t.Errorf("LocationType = %q, want restaurant", rp.LocationType)
		}
	})

	t.Run("cinema is dropped from Eatery (no matching category type)", func(t *testing.T) {
		p := POI.Place{LocationType: POI.LocationTypeRestaurant, Types: []string{"movie_theater", "point_of_interest", "establishment"}}
		if _, keep := POI.ReclassifyForCategory(p, POI.PlaceCategoryEatery); keep {
			t.Error("expected a cinema to be dropped from the Eatery category")
		}
	})

	t.Run("place with no Types is kept unchanged (older cache records)", func(t *testing.T) {
		p := POI.Place{LocationType: POI.LocationTypeRestaurant}
		rp, keep := POI.ReclassifyForCategory(p, POI.PlaceCategoryEatery)
		if !keep {
			t.Fatal("expected a Types-less place to be kept")
		}
		if rp.LocationType != POI.LocationTypeRestaurant {
			t.Errorf("LocationType = %q, want restaurant (unchanged)", rp.LocationType)
		}
	})
}
