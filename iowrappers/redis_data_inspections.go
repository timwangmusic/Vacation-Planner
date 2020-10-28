package iowrappers

import (
	"context"
	"github.com/weihesdlegend/Vacation-planner/POI"
	"strings"
)

const (
	PlaceDetailsKeyPrefix = "place_details"
	PlaceIDsKeyPrefix     = "placeIDs"
)

func (redisClient *RedisClient) GetPlaceCountInRedis(context context.Context) (count int, err error) {
	var cursor uint64

	for {
		var keys []string
		var err error
		keys, cursor, err = redisClient.client.Scan(context, cursor, PlaceDetailsKeyPrefix+"*", 100).Result()
		if err != nil {
			return 0, err
		}
		count += len(keys)
		if cursor == 0 {
			break
		}
	}
	return count, nil
}

func (redisClient *RedisClient) GetCityCountInRedis(context context.Context) (map[string]string, error) {
	redisKey := "geocode:cities"
	geocodes, err := redisClient.client.HGetAll(context, redisKey).Result()
	if err != nil {
		return nil, err
	}
	return geocodes, nil
}

func (redisClient *RedisClient) GetPlaceCountByCategory(context context.Context, category POI.PlaceCategory) (int64, error) {
	redisKey := strings.Join([]string{PlaceIDsKeyPrefix, strings.ToLower(string(category))}, ":")
	var count int64
	var err error
	count, err = redisClient.client.ZCard(context, redisKey).Result()
	return count, err
}
