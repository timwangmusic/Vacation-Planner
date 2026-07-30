package test

import (
	"testing"

	"github.com/weihesdlegend/Vacation-planner/POI"
	"googlemaps.github.io/maps"
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
//
// university/airport/real_estate_agency/doctor/casino/train_station/campground/
// physiotherapist are legal legacy Places API types (maps.ParsePlaceType accepts all of
// them) that this service simply has no use for yet. They pin the refusal list the
// text-search insert endpoint's 422 path depends on: an unmapped-but-legal type must stay
// refused, not get swept in by a future overly-broad edit to placeTypeToCategory.
func TestGetPlaceCategoryRejectsUnknownTypes(t *testing.T) {
	unknown := []POI.LocationType{
		POI.LocationType("fast_food_restaurant"),
		POI.LocationType("food_court"),
		POI.LocationType("lodging_but_not_really"),
		POI.LocationType(""),
		POI.LocationType("university"),
		POI.LocationType("airport"),
		POI.LocationType("real_estate_agency"),
		POI.LocationType("doctor"),
		POI.LocationType("casino"),
		POI.LocationType("train_station"),
		POI.LocationType("campground"),
		POI.LocationType("physiotherapist"),
	}
	for _, placeType := range unknown {
		if got, ok := POI.GetPlaceCategory(placeType); ok {
			t.Errorf("GetPlaceCategory(%q) = (%q, true), want ok=false", placeType, got)
		}
	}
}

// TestGetPlaceCategoryKnownTypes pins that every mapped type still resolves, including the
// 25 primary-type entries Task 1 added on top of the original 18 searched types.
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

		// New entries: Eatery
		POI.LocationTypeMealDelivery: POI.PlaceCategoryEatery,
		POI.LocationTypeNightClub:    POI.PlaceCategoryEatery,

		// New entries: Visit
		POI.LocationTypeTouristAttraction: POI.PlaceCategoryVisit,
		POI.LocationTypeZoo:               POI.PlaceCategoryVisit,
		POI.LocationTypeAquarium:          POI.PlaceCategoryVisit,
		POI.LocationTypeMovieTheater:      POI.PlaceCategoryVisit,
		POI.LocationTypeStadium:           POI.PlaceCategoryVisit,
		POI.LocationTypeBowlingAlley:      POI.PlaceCategoryVisit,

		// New entries: Shopping
		POI.LocationTypeGroceryOrSupermarket: POI.PlaceCategoryShopping,
		POI.LocationTypeConvenienceStore:     POI.PlaceCategoryShopping,
		POI.LocationTypeHardwareStore:        POI.PlaceCategoryShopping,
		POI.LocationTypeHomeGoodsStore:       POI.PlaceCategoryShopping,
		POI.LocationTypeElectronicsStore:     POI.PlaceCategoryShopping,
		POI.LocationTypeFurnitureStore:       POI.PlaceCategoryShopping,
		POI.LocationTypeBookStore:            POI.PlaceCategoryShopping,
		POI.LocationTypeShoeStore:            POI.PlaceCategoryShopping,
		POI.LocationTypeJewelryStore:         POI.PlaceCategoryShopping,
		POI.LocationTypePetStore:             POI.PlaceCategoryShopping,
		POI.LocationTypeBicycleStore:         POI.PlaceCategoryShopping,
		POI.LocationTypeFlorist:              POI.PlaceCategoryShopping,
		POI.LocationTypeLiquorStore:          POI.PlaceCategoryShopping,
		POI.LocationTypeGasStation:           POI.PlaceCategoryShopping,

		// New entries: Wellness
		POI.LocationTypeDrugstore:   POI.PlaceCategoryWellness,
		POI.LocationTypeBeautySalon: POI.PlaceCategoryWellness,
		POI.LocationTypeHairCare:    POI.PlaceCategoryWellness,
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

// TestGetPlaceCategoryKeysAreGoogleTypes is the structural guard against another
// fast_food_restaurant-style typo entering placeTypeToCategory: every mapped key must be a
// legal legacy Places API type per the SDK's own ParsePlaceType, except the documented
// types[]-only allowlist.
func TestGetPlaceCategoryKeysAreGoogleTypes(t *testing.T) {
	// grocery_or_supermarket appears in a place's Types[] but is not a legal ?type= search
	// value (maps.ParsePlaceType rejects it) — see the constant's comment in POI/categories.go.
	allowlist := map[POI.LocationType]bool{
		POI.LocationTypeGroceryOrSupermarket: true,
	}
	for _, lt := range POI.MappedLocationTypes() {
		if allowlist[lt] {
			continue
		}
		if _, err := maps.ParsePlaceType(string(lt)); err != nil {
			t.Errorf("MappedLocationTypes() contains %q, which is not a legal Places API type: %v", lt, err)
		}
	}
}

// TestGetPlaceTypesSubsetOfCategoryMap makes the superset relation explicit: every type a
// category actively searches for (GetPlaceTypes) must map back to that category via the
// broader placeTypeToCategory map (GetPlaceCategory). This strengthens TestPlaceCategoryRoundTrip
// by pinning the relationship the docstrings now describe — placeTypeToCategory is a superset
// of GetPlaceTypes' inverse, not an exact inverse.
func TestGetPlaceTypesSubsetOfCategoryMap(t *testing.T) {
	for _, cat := range POI.AllPlaceCategories {
		for _, placeType := range POI.GetPlaceTypes(cat) {
			got, ok := POI.GetPlaceCategory(placeType)
			if !ok {
				t.Errorf("category %s: searched type %q has no entry in GetPlaceCategory", cat, placeType)
				continue
			}
			if got != cat {
				t.Errorf("category %s: searched type %q maps to %s via GetPlaceCategory", cat, placeType, got)
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

	t.Run("meal_delivery is kept in Eatery and re-tagged", func(t *testing.T) {
		p := POI.Place{LocationType: POI.LocationTypeRestaurant, Types: []string{"meal_delivery", "restaurant", "food"}}
		rp, keep := POI.ReclassifyForCategory(p, POI.PlaceCategoryEatery)
		if !keep {
			t.Fatal("expected a meal_delivery place to be kept in Eatery")
		}
		if rp.LocationType != POI.LocationType("meal_delivery") {
			t.Errorf("LocationType = %q, want meal_delivery", rp.LocationType)
		}
	})

	t.Run("night_club is kept in Eatery", func(t *testing.T) {
		p := POI.Place{LocationType: POI.LocationTypeBar, Types: []string{"night_club", "bar", "point_of_interest"}}
		if _, keep := POI.ReclassifyForCategory(p, POI.PlaceCategoryEatery); !keep {
			t.Error("expected a night_club place to be kept in Eatery")
		}
	})

	t.Run("convenience_store is kept in Shopping", func(t *testing.T) {
		p := POI.Place{LocationType: POI.LocationTypeStore, Types: []string{"convenience_store", "store", "food"}}
		if _, keep := POI.ReclassifyForCategory(p, POI.PlaceCategoryShopping); !keep {
			t.Error("expected a convenience_store place to be kept in Shopping")
		}
	})

	t.Run("grocery_or_supermarket is kept in Shopping", func(t *testing.T) {
		p := POI.Place{LocationType: POI.LocationTypeSupermarket, Types: []string{"grocery_or_supermarket", "food", "store"}}
		if _, keep := POI.ReclassifyForCategory(p, POI.PlaceCategoryShopping); !keep {
			t.Error("expected a grocery_or_supermarket place to be kept in Shopping")
		}
	})

	t.Run("drugstore is kept in Wellness", func(t *testing.T) {
		p := POI.Place{LocationType: POI.LocationTypePharmacy, Types: []string{"drugstore", "point_of_interest"}}
		if _, keep := POI.ReclassifyForCategory(p, POI.PlaceCategoryWellness); !keep {
			t.Error("expected a drugstore place to be kept in Wellness")
		}
	})

	t.Run("tourist_attraction is kept in Visit", func(t *testing.T) {
		p := POI.Place{LocationType: POI.LocationTypePark, Types: []string{"tourist_attraction", "point_of_interest"}}
		if _, keep := POI.ReclassifyForCategory(p, POI.PlaceCategoryVisit); !keep {
			t.Error("expected a tourist_attraction place to be kept in Visit")
		}
	})

	t.Run("university is dropped from Eatery", func(t *testing.T) {
		p := POI.Place{LocationType: POI.LocationTypeRestaurant, Types: []string{"university", "point_of_interest", "establishment"}}
		if _, keep := POI.ReclassifyForCategory(p, POI.PlaceCategoryEatery); keep {
			t.Error("expected a university to be dropped from the Eatery category")
		}
	})

	t.Run("movie_theater is dropped from Eatery and kept in Visit", func(t *testing.T) {
		p := POI.Place{LocationType: POI.LocationTypeRestaurant, Types: []string{"movie_theater", "point_of_interest", "establishment"}}
		if _, keep := POI.ReclassifyForCategory(p, POI.PlaceCategoryEatery); keep {
			t.Error("expected a movie_theater to be dropped from the Eatery category")
		}
		rp, keep := POI.ReclassifyForCategory(p, POI.PlaceCategoryVisit)
		if !keep {
			t.Fatal("expected a movie_theater to be kept in Visit")
		}
		if rp.LocationType != POI.LocationType("movie_theater") {
			t.Errorf("LocationType = %q, want movie_theater", rp.LocationType)
		}
	})
}

// TestReclassifyForCategoryKeepsAllFormerlySearchedTypes is the monotonicity guarantee the
// task brief requires: because placeTypeToCategory is a superset of GetPlaceTypes' inverse,
// widening it must never cause ReclassifyForCategory to drop a place that the OLD
// primary-in-GetPlaceTypes(cat) rule would have kept. For every category and every type it
// actively searches for, a place whose primary type is that search type must still be kept.
func TestReclassifyForCategoryKeepsAllFormerlySearchedTypes(t *testing.T) {
	for _, cat := range POI.AllPlaceCategories {
		for _, placeType := range POI.GetPlaceTypes(cat) {
			p := POI.Place{LocationType: placeType, Types: []string{string(placeType)}}
			if _, keep := POI.ReclassifyForCategory(p, cat); !keep {
				t.Errorf("category %s: a place searched-and-primary-typed %q must be kept, was dropped", cat, placeType)
			}
		}
	}
}
