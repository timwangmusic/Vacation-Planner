package iowrappers

import (
	"context"
	"fmt"
	"github.com/weihesdlegend/Vacation-planner/POI"
)

func (r *RedisClient) SaveLocationPhotoID(ctx context.Context, location *POI.Location, id string) error {
	location.Normalize()

	// facilitate search from larger area to smaller area, e.g. photos:china:* looks for all the keys related to China
	key := fmt.Sprintf("photos:%s:%s:%s", location.Country, location.AdminAreaLevelOne, location.City)

	return r.Get().SAdd(ctx, key, id).Err()
}

func (r *RedisClient) GetLocationPhotoIDs(ctx context.Context, location *POI.Location) ([]string, error) {
	location.Normalize()

	key := fmt.Sprintf("photos:%s:%s:%s", location.Country, location.AdminAreaLevelOne, location.City)

	var res []string
	var err error
	if res, err = r.Get().SMembers(ctx, key).Result(); err != nil {
		return res, err
	}
	return res, nil
}
