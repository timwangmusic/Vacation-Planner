package iowrappers

import (
	"Vacation-planner/POI"
	"Vacation-planner/utils"
	"encoding/json"
	"fmt"
	"github.com/go-redis/redis"
	"strconv"
	"strings"
)

type RedisClient struct {
	client redis.Client

}

func (redisClient *RedisClient) Init(addr string, password string, databaseIdx int) {
	redisClient.client = *redis.NewClient(&redis.Options{
		Addr: addr,
		Password: password,
		DB: databaseIdx,
	})
}

// serialize place using JSON and store in Redis with key as the place ID
func (redisClient *RedisClient) setPlace(place POI.Place) {
	json_, err := json.Marshal(place)
	utils.CheckErr(err)

	redisClient.client.Set(place.ID, json_, -1)
}

// Store places obtained from database in Redis
// 1) Store place ID to the sorted set for the location
// 2) Store place details in hash
func (redisClient *RedisClient) StorePlacesForLocation(location string, places []POI.Place) {
	client := redisClient.client
	latLng := strings.Split(location, ",")
	lat, _ := strconv.ParseFloat(latLng[0], 64)
	lng, _ := strconv.ParseFloat(latLng[1], 64)

	for _, place := range places {
		dist := utils.HaversineDist([]float64{lng, lat}, place.Location.Coordinates[:])
		client.ZAdd(location, redis.Z{dist, place.ID})
		redisClient.setPlace(place)
	}
}

func (redisClient *RedisClient) getPlace(placeId string) (place POI.Place) {
	res, err := redisClient.client.Get(placeId).Result()
	utils.CheckErr(err)

	utils.CheckErr(json.Unmarshal([]byte(res), &place))
	return
}

func (redisClient *RedisClient) NearbySearch(request *PlaceSearchRequest) []POI.Place {
	placeIds, _ := redisClient.client.ZRangeWithScores(request.Location, 0, int64(request.Radius)).Result()
	res := make([]POI.Place, len(placeIds))

	for idx, placeId := range placeIds {
		res[idx] = redisClient.getPlace(fmt.Sprintf("%v", placeId.Member))
	}
	return res
}
