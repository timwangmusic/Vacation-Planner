package iowrappers

// RedisClient is a type wrapping-up over functionality defined in the go-redis library
// serving the caching needs of the Vacation Planner

import (
	"Vacation-planner/POI"
	"Vacation-planner/utils"
	"encoding/json"
	"fmt"
	"github.com/go-redis/redis"
	log "github.com/sirupsen/logrus"
	"strconv"
	"strings"
)

type RedisClient struct {
	client redis.Client
}

func (redisClient *RedisClient) Init(addr string, password string, databaseIdx int) {
	redisClient.client = *redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       databaseIdx,
	})
}

// serialize place using JSON and store in Redis with key as the place ID
func (redisClient *RedisClient) cachePlace(place POI.Place) {
	json_, err := json.Marshal(place)
	utils.CheckErr(err)

	redisClient.client.Set(place.ID, json_, -1)
}

// store places obtained from database or external API in Redis
// places for a location are stored in separate sorted sets based on category
func (redisClient *RedisClient) StorePlacesForLocation(location string, places []POI.Place, placeCategory POI.PlaceCategory) {
	client := redisClient.client
	latLng := strings.Split(location, ",")
	lat, _ := strconv.ParseFloat(latLng[0], 64)
	lng, _ := strconv.ParseFloat(latLng[1], 64)

	sortedSetKey := strings.Join([]string{location, string(placeCategory)}, "_")

	for _, place := range places {
		dist := utils.HaversineDist([]float64{lng, lat}, place.Location.Coordinates[:])
		client.ZAdd(sortedSetKey, redis.Z{dist, place.ID})
		redisClient.cachePlace(place)
	}
}

func (redisClient *RedisClient) SetPlacesOnCategory(places []POI.Place) {
	for _, place := range places {
		placeCategory := getPlaceCategory(LocationType(place.LocationType))
		geolocation := &redis.GeoLocation{
			Name:      place.ID,
			Longitude: place.Location.Coordinates[0],
			Latitude:  place.Location.Coordinates[1],
		}
		cmd_val, cmd_err := redisClient.client.GeoAdd(string(placeCategory), geolocation).Result()
		utils.CheckErr(cmd_err)
		if cmd_val == 1 {
			log.Printf("new place %s cache success", place.Name)
		}
		redisClient.cachePlace(place)
	}
}

// obtain place info from Redis based on placeId
func (redisClient *RedisClient) getPlace(placeId string) (place POI.Place) {
	res, err := redisClient.client.Get(placeId).Result()
	utils.CheckErr(err)

	utils.CheckErr(json.Unmarshal([]byte(res), &place))
	return
}

func (redisClient *RedisClient) NearbySearch(request *PlaceSearchRequest) []POI.Place {
	sortedSetKey := strings.Join([]string{request.Location, string(request.PlaceCat)}, "_")

	placeIds, _ := redisClient.client.ZRangeByScore(sortedSetKey, redis.ZRangeBy{
		Min: "0",
		Max: string(request.Radius),
	}).Result()

	res := make([]POI.Place, len(placeIds))

	for idx, placeId := range placeIds {
		res[idx] = redisClient.getPlace(fmt.Sprintf("%v", placeId))
	}
	return res
}

func (redisClient *RedisClient) GetPlaces(request *PlaceSearchRequest) (places []POI.Place) {
	requestCategory := string(request.PlaceCat)

	totalNumCachedResults, err := redisClient.client.ZCount(requestCategory, "-inf", "inf").Result()
	utils.CheckErr(err)
	if uint(totalNumCachedResults) < request.MinNumResults {
		return
	}

	lat_lng := utils.ParseLocation(request.Location)
	requestLat, requestLng := lat_lng[0], lat_lng[1]

	radiusMultiplier := uint(1)
	numCachedPlaces := 0
	cachedPlaces := make([]redis.GeoLocation, 0)
	searchRadius := request.Radius

	for searchRadius < MAX_SEARCH_RADIUS && uint(numCachedPlaces) < request.MinNumResults {
		searchRadius = request.Radius * radiusMultiplier
		fmt.Printf("Redis now using search of radius %d meters \n", searchRadius)
		geoQuery := redis.GeoRadiusQuery{
			Radius: float64(searchRadius),
			Unit:   "m",
			Sort:   "ASC", // sort ascending
		}
		cachedPlaces, err = redisClient.client.GeoRadius(requestCategory, requestLng, requestLat, &geoQuery).Result()
		utils.CheckErr(err)
		numCachedPlaces = len(cachedPlaces)
		radiusMultiplier *= 2
	}
	request.Radius = searchRadius

	places = make([]POI.Place, len(cachedPlaces))
	for idx, placeInfo := range cachedPlaces {
		places[idx] = redisClient.getPlace(placeInfo.Name)
	}
	return
}

func (redisClient *RedisClient) GetGeocode(query GeocodeQuery) (lat float64, lng float64, exist bool) {
	redis_key := "cities"
	redis_field := strings.ToLower(strings.Join([]string{query.City, query.Country}, "_"))
	geocode, err := redisClient.client.HGet(redis_key, redis_field).Result()
	if err != nil {
		return
	}
	lat_lng := utils.ParseLocation(geocode)
	lat = lat_lng[0]
	lng = lat_lng[1]
	exist = true
	log.Infof("Get geolocation for location %s, %s from cache success", query.City, query.Country)
	return
}

func (redisClient *RedisClient) SetGeocode(query GeocodeQuery, lat float64, lng float64) bool {
	redis_key := "cities"
	redis_field := strings.ToLower(strings.Join([]string{query.City, query.Country}, "_"))
	redis_val := strings.Join([]string{fmt.Sprint(lat), fmt.Sprint(lng)}, ",")
	res, err := redisClient.client.HSet(redis_key, redis_field, redis_val).Result()
	utils.CheckErr(err)
	if res {
		log.Infof("Cached geolocation for location %s, %s success", query.City, query.Country)
	}
	return res
}
