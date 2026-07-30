package redis_client_mocks

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/weihesdlegend/Vacation-planner/POI"
)

func TestRedisClient_GetMapsLastSearchTime(t *testing.T) {
	currentTime := time.Now()
	type args struct {
		context    context.Context
		lat        float64
		lng        float64
		category   POI.PlaceCategory
		priceLevel POI.PriceLevel
		timeToSave time.Time
	}
	tests := []struct {
		name               string
		args               args
		wantLastSearchTime time.Time
		wantErr            bool
	}{
		{
			name: "Redis client should retrieve Maps last search time",
			args: args{
				context:    context.Background(),
				lat:        37.7749,
				lng:        -122.4194,
				category:   POI.PlaceCategoryEatery,
				priceLevel: POI.PriceLevelFour,
				timeToSave: currentTime,
			},
			wantLastSearchTime: currentTime,
			wantErr:            false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := RedisClient
			err := r.SetMapsLastSearchTime(tt.args.context, tt.args.lat, tt.args.lng, tt.args.category, tt.args.priceLevel, tt.args.timeToSave.Format(time.RFC3339))
			if err != nil {
				t.Fatal(err)
			}

			gotLastSearchTime, err := r.GetMapsLastSearchTime(tt.args.context, tt.args.lat, tt.args.lng, tt.args.category, tt.args.priceLevel)
			if !tt.wantErr && err != nil {
				t.Errorf("GetMapsLastSearchTime(%v, %v, %v, %v, %v) encountered error: %v", tt.args.context, tt.args.lat, tt.args.lng, tt.args.category, tt.args.priceLevel, err)
				return
			}

			assert.Equalf(t, tt.wantLastSearchTime.Format(time.RFC3339), gotLastSearchTime.Format(time.RFC3339), "GetMapsLastSearchTime(%v, %v, %v, %v, %v)", tt.args.context, tt.args.lat, tt.args.lng, tt.args.category, tt.args.priceLevel)
		})
	}
}

// TestMapsLastSearchTimeIsCellScoped covers the defect that made the cache miss on nearly every
// merchant request: the marker used to be keyed on country/admin1/city while the geo bucket it
// guards is read from the caller's exact coordinates. A user far from the city centroid would
// read a marker claiming freshness over ground no search had populated.
func TestMapsLastSearchTimeIsCellScoped(t *testing.T) {
	ctx := context.Background()
	searchTime := time.Now().Format(time.RFC3339)

	// Deliberately mid-cell, near Los Angeles. A point picked at random has a real chance of
	// sitting against a cell edge, where a neighbour 1 km away is legitimately a different cell —
	// the boundary duplication the grid accepts in exchange for being a fixed, stateless key. The
	// precondition below keeps that property from being mistaken for a bug in the marker.
	const lat, lng = 34.092, -118.26
	const nearLat = lat + 0.009 // ~1 km north
	const farLat = lat + 0.2    // ~22 km north

	if POI.EncodeSearchCell(lat, lng) != POI.EncodeSearchCell(nearLat, lng) {
		t.Fatalf("fixture is not mid-cell: %s vs %s", POI.EncodeSearchCell(lat, lng), POI.EncodeSearchCell(nearLat, lng))
	}

	if err := RedisClient.SetMapsLastSearchTime(ctx, lat, lng, POI.PlaceCategoryShopping, POI.PriceLevelZero, searchTime); err != nil {
		t.Fatal(err)
	}

	t.Run("a nearby request reuses the marker", func(t *testing.T) {
		if _, err := RedisClient.GetMapsLastSearchTime(ctx, nearLat, lng, POI.PlaceCategoryShopping, POI.PriceLevelZero); err != nil {
			t.Errorf("expected a nearby request to hit the marker, got %v", err)
		}
	})

	t.Run("a request ~22 km away misses", func(t *testing.T) {
		if _, err := RedisClient.GetMapsLastSearchTime(ctx, farLat, lng, POI.PlaceCategoryShopping, POI.PriceLevelZero); err == nil {
			t.Error("expected a miss far outside the populated area, got a hit")
		}
	})

	// The specific asymmetry that was broken: Shopping/Lodging/Wellness buckets are not
	// price-partitioned, so their marker must not vary with price level either.
	t.Run("price level does not affect non-eatery markers", func(t *testing.T) {
		for _, level := range POI.AllPriceLevels {
			if _, err := RedisClient.GetMapsLastSearchTime(ctx, lat, lng, POI.PlaceCategoryShopping, level); err != nil {
				t.Errorf("price level %d missed a marker written at level 0: %v", level, err)
			}
		}
	})

	// Eatery levels 3-4 trigger a genuinely different Google request, so they must NOT be
	// satisfied by the generic marker.
	t.Run("pricey eatery searches keep their own marker", func(t *testing.T) {
		if err := RedisClient.SetMapsLastSearchTime(ctx, lat, lng, POI.PlaceCategoryEatery, POI.PriceLevelZero, searchTime); err != nil {
			t.Fatal(err)
		}
		for _, level := range []POI.PriceLevel{POI.PriceLevelThree, POI.PriceLevelFour} {
			if _, err := RedisClient.GetMapsLastSearchTime(ctx, lat, lng, POI.PlaceCategoryEatery, level); err == nil {
				t.Errorf("price level %d was served by the generic eatery marker", level)
			}
		}
		for _, level := range []POI.PriceLevel{POI.PriceLevelOne, POI.PriceLevelTwo} {
			if _, err := RedisClient.GetMapsLastSearchTime(ctx, lat, lng, POI.PlaceCategoryEatery, level); err != nil {
				t.Errorf("price level %d should share the generic eatery marker: %v", level, err)
			}
		}
	})
}
