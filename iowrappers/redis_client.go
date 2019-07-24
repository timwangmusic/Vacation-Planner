package iowrappers

import (
	"Vacation-planner/POI"
	"Vacation-planner/utils"
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

// Store a single place as a hash in Redis with key as the place ID
func (redisClient *RedisClient) storePlace(place POI.Place) {
	placeId := place.ID
	placeName := place.Name
	locationType := place.LocationType
	formattedAddr := place.FormattedAddress
	priceLevel := place.PriceLevel
	rating := place.Rating
	hours := strings.Join(place.Hours[:], ";")

	redisClient.client.HSet(placeId, "placeId", placeId)
	redisClient.client.HSet(placeId, "placeName", placeName)
	redisClient.client.HSet(placeId, "locationType", locationType)
	redisClient.client.HSet(placeId, "address", formattedAddr)
	redisClient.client.HSet(placeId, "priceLevel", priceLevel)
	redisClient.client.HSet(placeId, "rating", rating)
	redisClient.client.HSet(placeId, "hours", hours)
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
		redisClient.storePlace(place)
	}
}

func (redisClient *RedisClient) getPlace(placeId string) (place POI.Place) {
	values, err := redisClient.client.HGetAll(placeId).Result()
	utils.CheckErr(err)

	place.ID = values["placeId"]
	place.Name = values["placeName"]
	place.LocationType = values["locationType"]
	place.FormattedAddress = values["address"]

	price, err := strconv.ParseInt(values["priceLevel"], 10, 64)
	utils.CheckErr(err)
	place.PriceLevel = int(price)

	rating, err := strconv.ParseFloat(values["rating"], 64)
	utils.CheckErr(err)
	place.Rating = float32(rating)

	hours := strings.Split(values["hours"], ";")

	for idx, hour := range hours {
		place.Hours[idx] = hour
	}

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
