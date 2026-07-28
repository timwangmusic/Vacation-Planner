package iowrappers

import (
	"testing"

	"github.com/weihesdlegend/Vacation-planner/POI"
)

func TestNearbySearchRedisKeys(t *testing.T) {
	t.Run("eatery with AllPriceLevels unions every price bucket", func(t *testing.T) {
		got := nearbySearchRedisKeys(&PlaceSearchRequest{
			PlaceCat:       POI.PlaceCategoryEatery,
			AllPriceLevels: true,
		})
		want := []string{
			"placeIDs:eatery:level0",
			"placeIDs:eatery:level1",
			"placeIDs:eatery:level2",
			"placeIDs:eatery:level3",
			"placeIDs:eatery:level4",
		}
		if len(got) != len(want) {
			t.Fatalf("got %d keys %v, want %d %v", len(got), got, len(want), want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("key[%d] = %q, want %q", i, got[i], want[i])
			}
		}
	})

	t.Run("eatery without AllPriceLevels reads a single bucket", func(t *testing.T) {
		got := nearbySearchRedisKeys(&PlaceSearchRequest{
			PlaceCat:   POI.PlaceCategoryEatery,
			PriceLevel: POI.PriceLevelTwo,
		})
		if len(got) != 1 || got[0] != "placeIDs:eatery:level2" {
			t.Errorf("got %v, want [placeIDs:eatery:level2]", got)
		}
	})

	t.Run("non-eatery categories are single-bucket even with AllPriceLevels", func(t *testing.T) {
		for _, cat := range []POI.PlaceCategory{
			POI.PlaceCategoryShopping, POI.PlaceCategoryLodging, POI.PlaceCategoryWellness,
		} {
			got := nearbySearchRedisKeys(&PlaceSearchRequest{PlaceCat: cat, AllPriceLevels: true})
			want := POI.EncodeNearbySearchRedisKey(cat, POI.PriceLevelZero)
			if len(got) != 1 || got[0] != want {
				t.Errorf("category %s: got %v, want [%s]", cat, got, want)
			}
		}
	})

	t.Run("keyword search uses the brand bucket, ignoring AllPriceLevels", func(t *testing.T) {
		got := nearbySearchRedisKeys(&PlaceSearchRequest{
			Keyword:        "Dunkin'",
			PlaceCat:       POI.PlaceCategoryEatery,
			AllPriceLevels: true,
		})
		want := POI.EncodeBrandNearbySearchRedisKey("Dunkin'")
		if len(got) != 1 || got[0] != want {
			t.Errorf("got %v, want [%s]", got, want)
		}
	})
}
