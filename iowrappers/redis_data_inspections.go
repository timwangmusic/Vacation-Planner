package iowrappers

import (
	"context"
	"strings"

	"github.com/weihesdlegend/Vacation-planner/POI"
)

const (
	PlaceDetailsKeyPrefix = "place_details"
	PlaceIDsKeyPrefix     = "placeIDs"
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

func (r *RedisClient) GetPlaceCountByCategory(context context.Context, category POI.PlaceCategory) (int64, error) {
	redisKey := strings.Join([]string{PlaceIDsKeyPrefix, strings.ToLower(string(category))}, ":")
	var count int64
	var err error
	count, err = r.client.ZCard(context, redisKey).Result()
	return count, err
}
