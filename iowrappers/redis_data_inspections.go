package iowrappers

import (
	"context"

	"github.com/weihesdlegend/Vacation-planner/POI"
)

const (
	PlaceDetailsKeyPrefix = "place_details"
)

func (r *RedisClient) GetPlaceCountInRedis(context context.Context) (placeKeys []string, count int, err error) {
	var cursor uint64
	placeKeys = make([]string, 0)

	for {
		var keys []string
		var err error
		keys, cursor, err = r.client.Scan(context, cursor, PlaceDetailsKeyPrefix+"*", 100).Result()
		if err != nil {
			return placeKeys, count, err
		}
		count += len(keys)
		placeKeys = append(placeKeys, keys...)
		if cursor == 0 {
			break
		}
	}
	return placeKeys, count, nil
}

func (r *RedisClient) GetCities(context context.Context) (map[string]string, error) {
	redisKey := "geocode:cities"
	geocodes, err := r.client.HGetAll(context, redisKey).Result()
	if err != nil {
		return nil, err
	}
	return geocodes, nil
}

// GetPlaceCountByCategory returns how many places sit in a category's geo bucket. It goes
// through POI.EncodeNearbySearchRedisKey rather than assembling the key locally: the hand-rolled
// version produced "placeIDs:eatery" while the write path used "placeIDs:eatery:level<N>", so
// this reported zero eateries no matter how many were stored.
func (r *RedisClient) GetPlaceCountByCategory(context context.Context, category POI.PlaceCategory) (int64, error) {
	return r.client.ZCard(context, POI.EncodeNearbySearchRedisKey(category)).Result()
}
