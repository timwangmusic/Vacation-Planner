package iowrappers

import (
	"context"
	"fmt"
	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/weihesdlegend/Vacation-planner/POI"
	"net/url"
	"testing"
)

var redisClient *RedisClient
var ctx context.Context

func init() {
	// set up
	RedisMockSvr, _ := miniredis.Run()

	redisUrl := "redis://" + RedisMockSvr.Addr()
	redisURL, _ := url.Parse(redisUrl)
	redisClient = CreateRedisClient(redisURL)
	ctx = context.Background()
}

func TestRedisClient_SaveLocationPhotoID(t *testing.T) {
	type fields struct {
		client *RedisClient
	}
	type args struct {
		ctx      context.Context
		location POI.Location
		id       string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "test save function should return no error",
			fields: fields{
				redisClient,
			},
			args: args{
				ctx: ctx,
				location: POI.Location{
					City:              "San Francisco",
					AdminAreaLevelOne: "CA",
					Country:           "USA",
				},
				id: "xyz12345",
			},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.fields.client
			tt.wantErr(t, r.SaveLocationPhotoID(tt.args.ctx, tt.args.location, tt.args.id),
				fmt.Sprintf("SaveLocationPhotoID(%v, %v, %v)", tt.args.ctx, tt.args.location, tt.args.id))

			photoIDs, err := r.GetLocationPhotoIDs(ctx, tt.args.location)
			tt.wantErr(t, err, fmt.Sprintf("GetLocationPhotoIDs(%v, %v) returned error %v\n", tt.args.location, tt.args.id, err))

			assert.Equal(t, []string{tt.args.id}, photoIDs)
		})
	}
}
