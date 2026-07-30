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
			got, ok := POI.GetPlaceCategory(placeType)
			if !ok {
				t.Errorf("round-trip broken: GetPlaceCategory(%q) has no category, want %q", placeType, category)
				continue
			}
			if got != category {
				t.Errorf("round-trip broken: GetPlaceCategory(%q) = %q, want %q", placeType, got, category)
			}
		}
	}
}

// TestGetPlaceCategoryRejectsUnknownTypes pins the fix for the fast_food_restaurant
// incident: GetPlaceCategory must NOT silently absorb unmapped types into Eatery.
// A default-to-Eatery branch made TestPlaceCategoryRoundTrip un-failable, so two
// Places-API-(New)-only types were added to GetPlaceTypes(Eatery) and hotels were
// written into the eatery geo buckets.
func TestGetPlaceCategoryRejectsUnknownTypes(t *testing.T) {
	unknown := []POI.LocationType{
		POI.LocationType("fast_food_restaurant"),
		POI.LocationType("food_court"),
		POI.LocationType("lodging_but_not_really"),
		POI.LocationType(""),
	}
	for _, placeType := range unknown {
		if got, ok := POI.GetPlaceCategory(placeType); ok {
			t.Errorf("GetPlaceCategory(%q) = (%q, true), want ok=false", placeType, got)
		}
	}
}

// TestGetPlaceCategoryKnownTypes pins that every mapped type still resolves.
func TestGetPlaceCategoryKnownTypes(t *testing.T) {
	cases := map[POI.LocationType]POI.PlaceCategory{
		POI.LocationTypeCafe:         POI.PlaceCategoryEatery,
		POI.LocationTypeRestaurant:   POI.PlaceCategoryEatery,
		POI.LocationTypeBar:          POI.PlaceCategoryEatery,
		POI.LocationTypeBakery:       POI.PlaceCategoryEatery,
		POI.LocationTypeMealTakeaway: POI.PlaceCategoryEatery,
		POI.LocationTypePark:         POI.PlaceCategoryVisit,
		POI.LocationTypeMuseum:       POI.PlaceCategoryVisit,
		POI.LocationTypeStore:        POI.PlaceCategoryShopping,
		POI.LocationTypeLodging:      POI.PlaceCategoryLodging,
		POI.LocationTypeGym:          POI.PlaceCategoryWellness,
	}
	for placeType, want := range cases {
		got, ok := POI.GetPlaceCategory(placeType)
		if !ok {
			t.Errorf("GetPlaceCategory(%q) returned ok=false, want %q", placeType, want)
			continue
		}
		if got != want {
			t.Errorf("GetPlaceCategory(%q) = %q, want %q", placeType, got, want)
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
	seen := make(map[string]POI.PlaceCategory)
	for _, category := range POI.AllPlaceCategories {
		key := POI.EncodeNearbySearchRedisKey(category)
		if other, dup := seen[key]; dup {
			t.Errorf("categories %s and %s share Redis key %q", other, category, key)
		}
		seen[key] = category
	}
}

// TestEncodeLastSearchTimeFieldMatchesSearchVariant pins the marker's scoping rule. The original
// defect was a marker scoped differently from what it guarded: only Visit was special-cased to
// drop the price segment, so Shopping/Lodging/Wellness carried a price-scoped marker over a
// price-agnostic bucket. The rule now is that the field identifies the external SEARCH variant.
func TestEncodeLastSearchTimeFieldMatchesSearchVariant(t *testing.T) {
	const lat, lng = 37.38, -122.11

	t.Run("non-eatery categories never carry a price segment", func(t *testing.T) {
		for _, cat := range POI.AllPlaceCategories {
			if cat == POI.PlaceCategoryEatery {
				continue
			}
			want := POI.EncodeLastSearchTimeField(cat, POI.PriceLevelZero, lat, lng)
			for _, level := range POI.AllPriceLevels {
				got := POI.EncodeLastSearchTimeField(cat, level, lat, lng)
				if got != want {
					t.Errorf("category %s at price level %d: got %q, want %q", cat, level, got, want)
				}
			}
		}
	})

	// Levels 0-2 produce an identical, unfiltered Google request, so they must share one marker
	// or two of every three fan-outs are redundant.
	t.Run("eatery levels 0-2 share one field", func(t *testing.T) {
		want := POI.EncodeLastSearchTimeField(POI.PlaceCategoryEatery, POI.PriceLevelZero, lat, lng)
		for _, level := range []POI.PriceLevel{POI.PriceLevelZero, POI.PriceLevelOne, POI.PriceLevelTwo} {
			if got := POI.EncodeLastSearchTimeField(POI.PlaceCategoryEatery, level, lat, lng); got != want {
				t.Errorf("eatery level %d: got %q, want %q", level, got, want)
			}
		}
	})

	// PriceyEatery makes Google apply a real price filter at four times the radius, so a fresh
	// generic marker must not suppress it.
	t.Run("eatery levels 3 and 4 each get their own field", func(t *testing.T) {
		generic := POI.EncodeLastSearchTimeField(POI.PlaceCategoryEatery, POI.PriceLevelZero, lat, lng)
		three := POI.EncodeLastSearchTimeField(POI.PlaceCategoryEatery, POI.PriceLevelThree, lat, lng)
		four := POI.EncodeLastSearchTimeField(POI.PlaceCategoryEatery, POI.PriceLevelFour, lat, lng)
		for _, pair := range [][2]string{{generic, three}, {generic, four}, {three, four}} {
			if pair[0] == pair[1] {
				t.Errorf("fields must differ, both are %q", pair[0])
			}
		}
	})
}

func TestEncodeSearchCell(t *testing.T) {
	const lat, lng = 37.38, -122.11
	base := POI.EncodeSearchCell(lat, lng)

	// ~1 km north: well inside a cell sized to the ~8 km cold-search radius, so a second request
	// nearby must reuse the first one's freshness rather than re-searching.
	if near := POI.EncodeSearchCell(lat+0.009, lng); near != base {
		t.Errorf("a point ~1 km away landed in cell %q, want %q", near, base)
	}

	// ~22 km north: beyond anything the first search populated, so it must be its own cell. This
	// is the case a city-scoped marker got wrong — claiming coverage over ground no search reached.
	if far := POI.EncodeSearchCell(lat+0.2, lng); far == base {
		t.Errorf("a point ~22 km away shares cell %q", far)
	}

	if crossed := POI.EncodeSearchCell(-lat, -lng); crossed == base {
		t.Errorf("the opposite hemisphere shares cell %q", crossed)
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
