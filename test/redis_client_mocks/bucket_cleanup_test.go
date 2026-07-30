package redis_client_mocks

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
)

// bucketCleanupFixtureIDs lists every place ID this test file writes into Redis.
// RedisClient/RedisMockSvr are process-wide fixtures shared with every other test in this
// package (some of which seed cities and places once via their own package init()), so
// resetBucketCleanupFixtures below clears only these specific keys between runs rather than
// flushing the whole mock server, which would also erase those unrelated fixtures.
var bucketCleanupFixtureIDs = []string{"hotel-1", "cafe-1", "hotel-2", "cafe-2", "legacy-1"}

// resetBucketCleanupFixtures gives each test in this file a clean slate for its own fixture
// IDs without disturbing state other test files depend on.
func resetBucketCleanupFixtures(t *testing.T) {
	t.Helper()
	detailKeys := make([]string, 0, len(bucketCleanupFixtureIDs))
	for _, id := range bucketCleanupFixtureIDs {
		detailKeys = append(detailKeys, iowrappers.PlaceDetailsRedisKeyPrefix+id)
	}
	if err := RedisClient.RemoveKeys(RedisContext, detailKeys); err != nil {
		t.Fatalf("RemoveKeys: %v", err)
	}
	for _, lvl := range POI.AllPriceLevels {
		key := POI.EncodeNearbySearchRedisKey(POI.PlaceCategoryEatery, lvl)
		if !RedisMockSvr.Exists(key) {
			continue
		}
		for _, id := range bucketCleanupFixtureIDs {
			if _, err := RedisMockSvr.ZRem(key, id); err != nil && err != miniredis.ErrKeyNotFound {
				t.Fatalf("ZRem(%s, %s): %v", key, id, err)
			}
		}
	}
}

// TestRemoveMisclassifiedPlacesDryRun pins that a dry run reports the hotels that the
// fast_food_restaurant incident wrote into placeIDs:eatery:level* without deleting them.
func TestRemoveMisclassifiedPlacesDryRun(t *testing.T) {
	hotel := newPlaceWithTypes("hotel-1", "Residence Inn by Marriott Palo Alto",
		POI.LocationType("fast_food_restaurant"), []string{"lodging", "point_of_interest", "establishment"})
	cafe := newPlaceWithTypes("cafe-1", "Peet's Coffee",
		POI.LocationTypeCafe, []string{"cafe", "food", "point_of_interest", "establishment"})
	RedisClient.SetPlacesAddGeoLocations(RedisContext, []POI.Place{cafe})
	seedGeoBucket(t, POI.PlaceCategoryEatery, hotel)

	report, err := RedisClient.RemoveMisclassifiedPlacesFromCategoryBuckets(RedisContext, POI.PlaceCategoryEatery, true)
	if err != nil {
		t.Fatalf("RemoveMisclassifiedPlacesFromCategoryBuckets error: %v", err)
	}
	if report.Misclassified != 1 {
		t.Errorf("Misclassified = %d, want 1 (report: %+v)", report.Misclassified, report)
	}
	if report.Removed != 0 {
		t.Errorf("dry run Removed = %d, want 0", report.Removed)
	}
	if len(report.RemovedIDs) != 1 || report.RemovedIDs[0] != "hotel-1" {
		t.Errorf("RemovedIDs = %v, want [hotel-1]", report.RemovedIDs)
	}
	// the hotel must still be present after a dry run
	if got := countInEateryBuckets(t, "hotel-1"); got == 0 {
		t.Error("dry run deleted hotel-1, want it retained")
	}
}

// TestRemoveMisclassifiedPlacesApply pins that a real run removes only the hotel.
func TestRemoveMisclassifiedPlacesApply(t *testing.T) {
	resetBucketCleanupFixtures(t)

	hotel := newPlaceWithTypes("hotel-2", "The Westin Palo Alto",
		POI.LocationType("fast_food_restaurant"), []string{"lodging", "point_of_interest", "establishment"})
	cafe := newPlaceWithTypes("cafe-2", "Red Rock Coffee",
		POI.LocationTypeCafe, []string{"cafe", "food", "point_of_interest", "establishment"})
	RedisClient.SetPlacesAddGeoLocations(RedisContext, []POI.Place{cafe})
	seedGeoBucket(t, POI.PlaceCategoryEatery, hotel)

	report, err := RedisClient.RemoveMisclassifiedPlacesFromCategoryBuckets(RedisContext, POI.PlaceCategoryEatery, false)
	if err != nil {
		t.Fatalf("RemoveMisclassifiedPlacesFromCategoryBuckets error: %v", err)
	}
	if report.Removed != 1 {
		t.Errorf("Removed = %d, want 1 (report: %+v)", report.Removed, report)
	}
	if got := countInEateryBuckets(t, "hotel-2"); got != 0 {
		t.Errorf("hotel-2 still in %d eatery buckets, want 0", got)
	}
	if got := countInEateryBuckets(t, "cafe-2"); got == 0 {
		t.Error("cafe-2 was removed, want it retained")
	}
}

// TestRemoveMisclassifiedPlacesKeepsUntypedRecords pins that older cached records with
// no Types list are left alone, matching ReclassifyForCategory's keep-on-unknown rule.
func TestRemoveMisclassifiedPlacesKeepsUntypedRecords(t *testing.T) {
	resetBucketCleanupFixtures(t)

	legacy := newPlaceWithTypes("legacy-1", "Old Cached Diner", POI.LocationTypeRestaurant, nil)
	RedisClient.SetPlacesAddGeoLocations(RedisContext, []POI.Place{legacy})

	report, err := RedisClient.RemoveMisclassifiedPlacesFromCategoryBuckets(RedisContext, POI.PlaceCategoryEatery, false)
	if err != nil {
		t.Fatalf("RemoveMisclassifiedPlacesFromCategoryBuckets error: %v", err)
	}
	if report.Removed != 0 {
		t.Errorf("Removed = %d, want 0 — records without Types must be kept", report.Removed)
	}
}

func newPlaceWithTypes(id, name string, locationType POI.LocationType, types []string) POI.Place {
	var p POI.Place
	p.SetID(id)
	p.SetName(name)
	p.SetType(locationType)
	p.SetStatus(string(POI.Operational))
	p.SetPriceLevel(POI.PriceLevelDefault)
	p.SetUserRatingsTotal(100)
	p.SetLocationCoordinates([2]float64{37.38006, -122.11612})
	p.Types = types
	return p
}

// seedGeoBucket writes a place record plus its eatery geo-bucket membership directly,
// bypassing SetPlacesAddGeoLocations, which after Task 1 refuses unmapped types.
func seedGeoBucket(t *testing.T, cat POI.PlaceCategory, place POI.Place) {
	t.Helper()
	if err := RedisClient.SetPlace(RedisContext, place); err != nil {
		t.Fatalf("SetPlace(%s): %v", place.GetID(), err)
	}
	key := POI.EncodeNearbySearchRedisKey(cat, place.PriceLevel)
	if err := RedisClient.AddGeoLocation(RedisContext, key, place); err != nil {
		t.Fatalf("AddGeoLocation(%s): %v", key, err)
	}
}

func countInEateryBuckets(t *testing.T, placeID string) int {
	t.Helper()
	count := 0
	for _, lvl := range POI.AllPriceLevels {
		key := POI.EncodeNearbySearchRedisKey(POI.PlaceCategoryEatery, lvl)
		if RedisMockSvr.Exists(key) {
			members, err := RedisMockSvr.ZMembers(key)
			if err != nil {
				continue
			}
			for _, m := range members {
				if m == placeID {
					count++
				}
			}
		}
	}
	return count
}
