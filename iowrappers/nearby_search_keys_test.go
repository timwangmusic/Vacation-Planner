package iowrappers

import (
	"testing"

	"github.com/weihesdlegend/Vacation-planner/POI"
)

func TestNearbySearchRedisKey(t *testing.T) {
	t.Run("every category reads one price-agnostic bucket", func(t *testing.T) {
		want := map[POI.PlaceCategory]string{
			POI.PlaceCategoryVisit:    "placeIDs:visit",
			POI.PlaceCategoryEatery:   "placeIDs:eatery",
			POI.PlaceCategoryShopping: "placeIDs:shopping",
			POI.PlaceCategoryLodging:  "placeIDs:lodging",
			POI.PlaceCategoryWellness: "placeIDs:wellness",
		}
		for cat, wantKey := range want {
			if got := nearbySearchRedisKey(&PlaceSearchRequest{PlaceCat: cat}); got != wantKey {
				t.Errorf("category %s: got %q, want %q", cat, got, wantKey)
			}
		}
	})

	// The regression guard for the original defect: the eatery bucket key must not vary with
	// price level, or a search at one level cannot see places stored at another.
	t.Run("the bucket key is identical across every price level", func(t *testing.T) {
		for _, cat := range POI.AllPlaceCategories {
			want := nearbySearchRedisKey(&PlaceSearchRequest{PlaceCat: cat, PriceLevel: POI.PriceLevelZero})
			for _, level := range POI.AllPriceLevels {
				got := nearbySearchRedisKey(&PlaceSearchRequest{PlaceCat: cat, PriceLevel: level})
				if got != want {
					t.Errorf("category %s at price level %d: got %q, want %q", cat, level, got, want)
				}
			}
		}
	})

	t.Run("keyword search uses the brand bucket", func(t *testing.T) {
		got := nearbySearchRedisKey(&PlaceSearchRequest{
			Keyword:  "Dunkin'",
			PlaceCat: POI.PlaceCategoryEatery,
		})
		if want := POI.EncodeBrandNearbySearchRedisKey("Dunkin'"); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}
