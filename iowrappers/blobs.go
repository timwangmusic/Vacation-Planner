package iowrappers

import (
	"context"
	"fmt"
	"github.com/weihesdlegend/Vacation-planner/POI"
	"strings"
)

func (r *RedisClient) SaveLocationPhotoID(ctx context.Context, location POI.Location, id string) error {
	normalized := strings.Split(location.String(), ", ")

	// facilitate search from larger area to smaller area, e.g. photos:china:* looks for all the keys related to China
	key := fmt.Sprintf("photos:%s:%s:%s", normalized[2], normalized[1], normalized[0])

	return r.Get().SAdd(ctx, key, id).Err()
}

func (r *RedisClient) GetLocationPhotoIDs(ctx context.Context, location POI.Location) ([]string, error) {
	normalized := strings.Split(location.String(), ", ")

	key := fmt.Sprintf("photos:%s:%s:%s", normalized[2], normalized[1], normalized[0])

	var res []string
	var err error
	if res, err = r.Get().SMembers(ctx, key).Result(); err != nil {
		return res, err
	}
	return res, nil
}
