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
		location   POI.Location
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
				location:   POI.Location{City: "San Francisco", AdminAreaLevelOne: "CA", Country: "USA"},
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
			err := r.SetMapsLastSearchTime(tt.args.context, tt.args.location, tt.args.category, tt.args.priceLevel, tt.args.timeToSave.Format(time.RFC3339))
			if err != nil {
				t.Fatal(err)
			}

			gotLastSearchTime, err := r.GetMapsLastSearchTime(tt.args.context, tt.args.location, tt.args.category, tt.args.priceLevel)
			if !tt.wantErr && err != nil {
				t.Errorf("GetMapsLastSearchTime(%v, %v, %v, %v) encountered error: %v", tt.args.context, tt.args.location, tt.args.category, tt.args.priceLevel, err)
				return
			}

			assert.Equalf(t, tt.wantLastSearchTime.Format(time.RFC3339), gotLastSearchTime.Format(time.RFC3339), "GetMapsLastSearchTime(%v, %v, %v, %v)", tt.args.context, tt.args.location, tt.args.category, tt.args.priceLevel)
		})
	}
}
