package iowrappers

import (
	"context"
	"fmt"
	"net/url"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/weihesdlegend/Vacation-planner/POI"
)

func unionTestClient(t *testing.T) (*RedisClient, context.Context) {
	t.Helper()
	svr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	t.Cleanup(svr.Close)

	redisURL, _ := url.Parse("redis://" + svr.Addr())
	_ = CreateLogger()
	return CreateRedisClient(redisURL), context.Background()
}

func legacyEateryKey(level POI.PriceLevel) string {
	return fmt.Sprintf("%s:level%d", POI.EncodeNearbySearchRedisKey(POI.PlaceCategoryEatery), level)
}

func TestUnionEateryPriceBuckets(t *testing.T) {
	r, ctx := unionTestClient(t)

	// one place per legacy price bucket, at distinct coordinates
	seeded := map[POI.PriceLevel]POI.Place{
		POI.PriceLevelZero:  placeAt("lvl0", 37.7749, -122.4194),
		POI.PriceLevelOne:   placeAt("lvl1", 37.7750, -122.4195),
		POI.PriceLevelTwo:   placeAt("lvl2", 37.7751, -122.4196),
		POI.PriceLevelThree: placeAt("lvl3", 37.7752, -122.4197),
		POI.PriceLevelFour:  placeAt("lvl4", 37.7753, -122.4198),
	}
	for level, place := range seeded {
		if err := r.AddGeoLocation(ctx, legacyEateryKey(level), place); err != nil {
			t.Fatalf("AddGeoLocation(%s): %v", legacyEateryKey(level), err)
		}
	}

	t.Run("a dry run reports the outcome without writing", func(t *testing.T) {
		report, err := r.UnionEateryPriceBucketsIntoCategoryBucket(ctx, true)
		if err != nil {
			t.Fatalf("dry run: %v", err)
		}
		if report.SourceTotal != 5 {
			t.Errorf("SourceTotal = %d, want 5", report.SourceTotal)
		}
		if report.ExpectedAfter != 5 {
			t.Errorf("ExpectedAfter = %d, want 5", report.ExpectedAfter)
		}
		if report.TargetAfter != 0 {
			t.Errorf("a dry run wrote to the target: TargetAfter = %d, want 0", report.TargetAfter)
		}
		count, err := r.Get().ZCard(ctx, report.TargetKey).Result()
		if err != nil {
			t.Fatalf("ZCard: %v", err)
		}
		if count != 0 {
			t.Errorf("target has %d members after a dry run, want 0", count)
		}
	})

	t.Run("apply merges every legacy bucket", func(t *testing.T) {
		report, err := r.UnionEateryPriceBucketsIntoCategoryBucket(ctx, false)
		if err != nil {
			t.Fatalf("apply: %v", err)
		}
		if report.TargetAfter != 5 {
			t.Errorf("TargetAfter = %d, want 5 (report: %+v)", report.TargetAfter, report)
		}

		members, err := r.Get().ZRange(ctx, report.TargetKey, 0, -1).Result()
		if err != nil {
			t.Fatalf("ZRange: %v", err)
		}
		got := make(map[string]bool, len(members))
		for _, m := range members {
			got[m] = true
		}
		for _, place := range seeded {
			if !got[place.ID] {
				t.Errorf("%s missing from the merged bucket", place.ID)
			}
		}
	})
}

// TestUnionEateryPriceBucketsPreservesCoordinates is the AGGREGATE MIN guard. A GEO member's
// score IS its 52-bit geohash, so the SUM that redis.ZStore.Aggregate defaults to would add the
// scores of a place present in two source buckets and relocate it — in practice to the middle of
// the ocean. The place below is seeded into two buckets specifically to exercise that path.
func TestUnionEateryPriceBucketsPreservesCoordinates(t *testing.T) {
	r, ctx := unionTestClient(t)

	const lat, lng = 37.7749, -122.4194
	duplicated := placeAt("in-two-buckets", lat, lng)
	for _, level := range []POI.PriceLevel{POI.PriceLevelZero, POI.PriceLevelTwo} {
		if err := r.AddGeoLocation(ctx, legacyEateryKey(level), duplicated); err != nil {
			t.Fatalf("AddGeoLocation: %v", err)
		}
	}

	report, err := r.UnionEateryPriceBucketsIntoCategoryBucket(ctx, false)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if report.TargetAfter != 1 {
		t.Fatalf("TargetAfter = %d, want 1 (the duplicate must merge to one member)", report.TargetAfter)
	}

	// A tight radius around the true coordinates finds the member only if its score survived as a
	// real geohash. Under SUM the score would be roughly doubled and this would return nothing.
	found, err := r.Get().GeoRadius(ctx, report.TargetKey, lng, lat, &redis.GeoRadiusQuery{
		Radius: 100,
		Unit:   "m",
		Sort:   "ASC",
	}).Result()
	if err != nil {
		t.Fatalf("GeoRadius: %v", err)
	}
	if len(found) != 1 || found[0].Name != duplicated.ID {
		t.Errorf("a 100 m search around the true coordinates returned %+v; the geohash score did not survive the union", found)
	}
}

// TestUnionEateryPriceBucketsIsRerunnable pins that the target is included in the union sources,
// so members written by already-deployed code are not dropped and the migration can be repeated.
func TestUnionEateryPriceBucketsIsRerunnable(t *testing.T) {
	r, ctx := unionTestClient(t)
	target := POI.EncodeNearbySearchRedisKey(POI.PlaceCategoryEatery)

	legacy := placeAt("legacy", 37.7749, -122.4194)
	if err := r.AddGeoLocation(ctx, legacyEateryKey(POI.PriceLevelOne), legacy); err != nil {
		t.Fatalf("AddGeoLocation: %v", err)
	}
	// a place the new write path already put in the collapsed bucket
	fresh := placeAt("already-collapsed", 37.7760, -122.4200)
	if err := r.AddGeoLocation(ctx, target, fresh); err != nil {
		t.Fatalf("AddGeoLocation: %v", err)
	}

	for run := 1; run <= 2; run++ {
		report, err := r.UnionEateryPriceBucketsIntoCategoryBucket(ctx, false)
		if err != nil {
			t.Fatalf("run %d: %v", run, err)
		}
		if report.TargetAfter != 2 {
			t.Errorf("run %d: TargetAfter = %d, want 2 (report: %+v)", run, report.TargetAfter, report)
		}
	}

	members, err := r.Get().ZRange(ctx, target, 0, -1).Result()
	if err != nil {
		t.Fatalf("ZRange: %v", err)
	}
	got := make(map[string]bool, len(members))
	for _, m := range members {
		got[m] = true
	}
	if !got[legacy.ID] || !got[fresh.ID] {
		t.Errorf("members = %v, want both %s and %s", members, legacy.ID, fresh.ID)
	}
}
