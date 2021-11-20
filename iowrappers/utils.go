package iowrappers

import (
	"context"
	"errors"
	"github.com/modern-go/reflect2"
)

func scanRedisKeys(context context.Context, redisClient *RedisClient, redisKeyPrefix string) ([]string, error) {
	var redisKeys []string
	if reflect2.IsNil(redisClient) {
		return redisKeys, errors.New("redis client pointer is nil")
	}

	var cursor uint64
	for {
		var err error
		var keys []string
		keys, cursor, err = redisClient.client.Scan(context, cursor, redisKeyPrefix+"*", 100).Result()
		if err != nil {
			return redisKeys, err
		}

		redisKeys = append(redisKeys, keys...)

		if cursor == 0 {
			break
		}
	}
	return redisKeys, nil
}
