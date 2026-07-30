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
var bucketCleanupFixtureIDs = []string{
	"hotel-1", "cafe-1", "hotel-2", "cafe-2", "legacy-1",
	"tt-lodging", "tt-supermarket", "tt-meal-delivery", "tt-night-club", "tt-no-types", "tt-cafe",
	"tt-orphan",
}

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

// TestRemoveMisclassifiedPlacesPrimaryTypeTruthTable pins the exact rule the cleanup applies:
// a bucket member is removed only when its PRIMARY Google type positively maps to a DIFFERENT
// category. An unmapped primary type is NOT evidence of misclassification — the fixed write
// path keys on the stamped LocationType, so it would legitimately file a delivery-first
// restaurant ("meal_delivery" first in Types) or a bar/club ("night_club" first) under Eatery.
// Purging those would make the migration delete rows the write path immediately re-creates,
// while shrinking the trip-planning candidate pool for up to MinMapsResultRefreshDuration.
func TestRemoveMisclassifiedPlacesPrimaryTypeTruthTable(t *testing.T) {
	resetBucketCleanupFixtures(t)
	t.Cleanup(func() { resetBucketCleanupFixtures(t) })

	cases := []struct {
		id         string
		name       string
		stamped    POI.LocationType
		types      []string
		wantRemove bool
		why        string
	}{
		{
			id: "tt-lodging", name: "Residence Inn by Marriott Palo Alto",
			// the incident: searched as fast_food_restaurant, truthfully a hotel
			stamped: POI.LocationType("fast_food_restaurant"),
			types:   []string{"lodging", "point_of_interest", "establishment"},
			// GetPlaceCategory("lodging") == (Lodging, true) != Eatery
			wantRemove: true, why: "primary type lodging maps to Lodging",
		},
		{
			id: "tt-supermarket", name: "Whole Foods Market",
			stamped: POI.LocationTypeRestaurant,
			types:   []string{"supermarket", "grocery_or_supermarket", "food", "store"},
			// GetPlaceCategory("supermarket") == (Shopping, true) != Eatery
			wantRemove: true, why: "primary type supermarket maps to Shopping",
		},
		{
			id: "tt-meal-delivery", name: "Wok This Way Delivery",
			stamped: POI.LocationTypeRestaurant,
			types:   []string{"meal_delivery", "restaurant", "food", "point_of_interest"},
			// GetPlaceCategory("meal_delivery") == ("", false): legal legacy type, unmapped
			wantRemove: false, why: "primary type meal_delivery maps to no category",
		},
		{
			id: "tt-night-club", name: "The Basement",
			stamped: POI.LocationTypeBar,
			types:   []string{"night_club", "bar", "point_of_interest", "establishment"},
			// GetPlaceCategory("night_club") == ("", false): legal legacy type, unmapped
			wantRemove: false, why: "primary type night_club maps to no category",
		},
		{
			id: "tt-no-types", name: "Old Cached Diner",
			stamped: POI.LocationTypeRestaurant,
			types:   nil,
			// PrimaryLocationType(nil) == "" -> GetPlaceCategory("") == ("", false)
			wantRemove: false, why: "no Types at all (record cached before Types was captured)",
		},
		{
			id: "tt-cafe", name: "Peet's Coffee",
			stamped: POI.LocationTypeCafe,
			types:   []string{"cafe", "food", "point_of_interest", "establishment"},
			// GetPlaceCategory("cafe") == (Eatery, true) == the bucket's category
			wantRemove: false, why: "primary type cafe maps to Eatery",
		},
	}

	for _, tc := range cases {
		seedGeoBucket(t, POI.PlaceCategoryEatery, newPlaceWithTypes(tc.id, tc.name, tc.stamped, tc.types))
	}

	report, err := RedisClient.RemoveMisclassifiedPlacesFromCategoryBuckets(RedisContext, POI.PlaceCategoryEatery, false)
	if err != nil {
		t.Fatalf("RemoveMisclassifiedPlacesFromCategoryBuckets error: %v", err)
	}

	removed := make(map[string]bool, len(report.RemovedIDs))
	for _, id := range report.RemovedIDs {
		removed[id] = true
	}

	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			if removed[tc.id] != tc.wantRemove {
				t.Errorf("reported removal of %s = %v, want %v (%s); report: %+v",
					tc.id, removed[tc.id], tc.wantRemove, tc.why, report)
			}
			inBuckets := countInEateryBuckets(t, tc.id)
			if tc.wantRemove && inBuckets != 0 {
				t.Errorf("%s still in %d eatery buckets after apply, want 0 (%s)", tc.id, inBuckets, tc.why)
			}
			if !tc.wantRemove && inBuckets == 0 {
				t.Errorf("%s was deleted from the eatery buckets, want it retained (%s)", tc.id, tc.why)
			}
		})
	}
}

// TestRemoveMisclassifiedPlacesToleratesMissingRecords pins that a bucket member with no
// backing place_details record is counted, skipped, and left in place rather than failing the
// run or being deleted. Orphaned members are RemovePlaces' job, not this migration's. This
// guards the batched read path specifically: a missing key surfaces as redis.Nil inside the
// pipeline, which must not be mistaken for a transport failure and abort the whole scan.
func TestRemoveMisclassifiedPlacesToleratesMissingRecords(t *testing.T) {
	resetBucketCleanupFixtures(t)
	t.Cleanup(func() { resetBucketCleanupFixtures(t) })

	// a geo-bucket member whose place record is deliberately never written
	orphan := newPlaceWithTypes("tt-orphan", "Vanished Diner", POI.LocationTypeRestaurant, nil)
	key := POI.EncodeNearbySearchRedisKey(POI.PlaceCategoryEatery, orphan.PriceLevel)
	if err := RedisClient.AddGeoLocation(RedisContext, key, orphan); err != nil {
		t.Fatalf("AddGeoLocation(%s): %v", key, err)
	}
	// a real misclassified place in the same batch, to prove the scan keeps going
	seedGeoBucket(t, POI.PlaceCategoryEatery, newPlaceWithTypes("tt-lodging", "The Westin Palo Alto",
		POI.LocationType("fast_food_restaurant"), []string{"lodging", "point_of_interest", "establishment"}))

	report, err := RedisClient.RemoveMisclassifiedPlacesFromCategoryBuckets(RedisContext, POI.PlaceCategoryEatery, false)
	if err != nil {
		t.Fatalf("a bucket member with no place record must not fail the run: %v", err)
	}
	if report.Scanned < 2 {
		t.Errorf("Scanned = %d, want at least the 2 seeded members (report: %+v)", report.Scanned, report)
	}
	for _, id := range report.RemovedIDs {
		if id == "tt-orphan" {
			t.Errorf("tt-orphan was reported for removal; a member with no record must be skipped (report: %+v)", report)
		}
	}
	if got := countInEateryBuckets(t, "tt-orphan"); got == 0 {
		t.Error("tt-orphan was deleted; a member with no record must be left for RemovePlaces")
	}
	// the scan must not have stopped at the orphan
	if got := countInEateryBuckets(t, "tt-lodging"); got != 0 {
		t.Errorf("tt-lodging still in %d eatery buckets; the scan stopped at the orphaned member", got)
	}
}

// TestRemoveMisclassifiedPlacesReportsBucketSizes pins that the report states the scale of the
// scan up front. An operator has to review a dry-run report before applying, so the report has
// to say how many members exist even when the scan itself is what makes the run expensive.
func TestRemoveMisclassifiedPlacesReportsBucketSizes(t *testing.T) {
	resetBucketCleanupFixtures(t)
	t.Cleanup(func() { resetBucketCleanupFixtures(t) })

	seedGeoBucket(t, POI.PlaceCategoryEatery, newPlaceWithTypes("tt-cafe", "Peet's Coffee",
		POI.LocationTypeCafe, []string{"cafe", "food", "point_of_interest", "establishment"}))

	report, err := RedisClient.RemoveMisclassifiedPlacesFromCategoryBuckets(RedisContext, POI.PlaceCategoryEatery, true)
	if err != nil {
		t.Fatalf("RemoveMisclassifiedPlacesFromCategoryBuckets error: %v", err)
	}

	// one entry per eatery price bucket, whether or not the key exists yet
	if len(report.BucketSizes) != len(POI.AllPriceLevels) {
		t.Errorf("BucketSizes has %d entries, want %d: %+v",
			len(report.BucketSizes), len(POI.AllPriceLevels), report.BucketSizes)
	}
	for _, lvl := range POI.AllPriceLevels {
		key := POI.EncodeNearbySearchRedisKey(POI.PlaceCategoryEatery, lvl)
		if _, ok := report.BucketSizes[key]; !ok {
			t.Errorf("BucketSizes missing key %s: %+v", key, report.BucketSizes)
		}
	}
	if report.TotalMembers < 1 {
		t.Errorf("TotalMembers = %d, want at least the 1 seeded place", report.TotalMembers)
	}
	// ZCARD is taken before the scan; nothing writes concurrently in this test, so every
	// counted member must also have been visited.
	if int(report.TotalMembers) != report.Scanned {
		t.Errorf("TotalMembers = %d but Scanned = %d, want equal", report.TotalMembers, report.Scanned)
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
