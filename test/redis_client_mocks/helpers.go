package redis_client_mocks

import (
	"github.com/alicebob/miniredis/v2"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"net/url"
)

var RedisClient iowrappers.RedisClient
var RedisMockSvr *miniredis.Miniredis

func init() {
	// set up
	RedisMockSvr, _ = miniredis.Run()

	redisUrl := "redis://" + RedisMockSvr.Addr()
	redisURL, _ := url.Parse(redisUrl)
	RedisClient.Init(redisURL)
}
