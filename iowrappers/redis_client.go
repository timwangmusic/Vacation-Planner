package iowrappers

// RedisClient is a type wrapping-up over functionality defined in the go-redis library
// serving the caching needs of the Vacation Planner

import (
	"Vacation-planner/POI"
	"Vacation-planner/utils"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-redis/redis/v7"
	log "github.com/sirupsen/logrus"
	"strconv"
	"strings"
	"time"
)

const (
	SlotSolutionExpirationTime = time.Duration(24 * time.Hour)
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
	utils.CheckErrImmediate(err, utils.LogError)

	redisClient.client.Set(place.ID, json_, -1)
}

// currently not used, but it is still a primitive implementation that might have faster search time compared
// with all places stored under one key
// store places obtained from database or external API in Redis
// places for a location are stored in separate sorted sets based on category
func (redisClient *RedisClient) StorePlacesForLocation(geocodeInString string, places []POI.Place) error {
	client := redisClient.client
	latLng, _ := utils.ParseLocation(geocodeInString)
	lat, lng := latLng[0], latLng[1]
	for _, place := range places {
		sortedSetKey := strings.Join([]string{geocodeInString, string(POI.GetPlaceCategory(place.LocationType))}, "_")
		dist := utils.HaversineDist([]float64{lng, lat}, place.Location.Coordinates[:])
		_, err := client.ZAdd(sortedSetKey, &redis.Z{Score: dist, Member: place.ID}).Result()
		if err != nil {
			return err
		}
		redisClient.cachePlace(place)
	}
	return nil
}

func (redisClient *RedisClient) SetPlacesOnCategory(places []POI.Place) {
	var geoAddSuccessCount int
	for _, place := range places {
		placeCategory := POI.GetPlaceCategory(place.LocationType)
		geolocation := &redis.GeoLocation{
			Name:      place.ID,
			Longitude: place.Location.Coordinates[0],
			Latitude:  place.Location.Coordinates[1],
		}
		cmdVal, cmdErr := redisClient.client.GeoAdd(string(placeCategory), geolocation).Result()
		utils.CheckErrImmediate(cmdErr, utils.LogError)
		if cmdVal == 0 {
			err := utils.Error{
				Err:   fmt.Errorf("geo adding place %s to Redis failure", place.Name),
				Level: utils.LogInfo,
			}
			utils.CheckErr(err)
		} else {
			geoAddSuccessCount++
			redisClient.cachePlace(place)
		}
	}
	log.Infof("%d places geo added to Redis", geoAddSuccessCount)
}

// obtain place info from Redis based on placeId
func (redisClient *RedisClient) getPlace(placeId string) (place POI.Place, err error) {
	res, err := redisClient.client.Get(placeId).Result()
	utils.CheckErrImmediate(err, utils.LogError)
	if err != nil {
		return
	}
	utils.CheckErrImmediate(json.Unmarshal([]byte(res), &place), utils.LogError)
	return
}

// currently not used
// use with StorePlacesForLocation method
// if no geocode in Redis, then we assume no nearby place exists either
func (redisClient *RedisClient) NearbySearch(request *PlaceSearchRequest) ([]POI.Place, error) {
	cityCountry := strings.Split(request.Location, ",")
	lat, lng, exist := redisClient.GetGeocode(GeocodeQuery{
		City:    cityCountry[0],
		Country: cityCountry[1],
	})
	if !exist {
		return nil, errors.New("no nearby place exist for the requested location")
	}
	latLng := strings.Join([]string{fmt.Sprintf("%f", lat), fmt.Sprintf("%f", lng)}, ",")
	sortedSetKey := strings.Join([]string{latLng, string(request.PlaceCat)}, "_")

	placeIds, _ := redisClient.client.ZRangeByScore(sortedSetKey, &redis.ZRangeBy{
		Min: "0",
		Max: fmt.Sprintf("%d", request.Radius),
	}).Result()

	res := make([]POI.Place, len(placeIds))

	for idx, placeId := range placeIds {
		res[idx], _ = redisClient.getPlace(fmt.Sprintf("%v", placeId))
	}
	return res, nil
}

func (redisClient *RedisClient) GetPlaces(request *PlaceSearchRequest) (places []POI.Place) {
	requestCategory := string(request.PlaceCat)

	totalNumCachedResults, err := redisClient.client.ZCount(requestCategory, "-inf", "inf").Result()
	utils.CheckErrImmediate(err, utils.LogInfo)
	if uint(totalNumCachedResults) < request.MinNumResults {
		return
	}

	latLng, _ := utils.ParseLocation(request.Location)
	requestLat, requestLng := latLng[0], latLng[1]

	radiusMultiplier := uint(1)
	numQualifiedCachedPlaces := 0
	cachedQualifiedPlaces := make([]redis.GeoLocation, 0)
	searchRadius := request.Radius

	if searchRadius > MaxSearchRadius {
		searchRadius = MaxSearchRadius
	}

	for searchRadius <= MaxSearchRadius && uint(numQualifiedCachedPlaces) < request.MinNumResults {
		searchRadius = request.Radius * radiusMultiplier
		fmt.Printf("Redis now using search of radius %d meters \n", searchRadius)
		geoQuery := redis.GeoRadiusQuery{
			Radius: float64(searchRadius),
			Unit:   "m",
			Sort:   "ASC", // sort ascending
		}
		cachedQualifiedPlaces, err = redisClient.client.GeoRadius(requestCategory, requestLng, requestLat, &geoQuery).Result()
		utils.CheckErrImmediate(err, utils.LogError)
		numQualifiedCachedPlaces = len(cachedQualifiedPlaces)
		radiusMultiplier *= 2
	}
	request.Radius = searchRadius

	places = make([]POI.Place, 0)
	for _, placeInfo := range cachedQualifiedPlaces {
		place, err := redisClient.getPlace(placeInfo.Name)
		if err == nil {
			places = append(places, place)
		}
	}
	return
}

func (redisClient *RedisClient) GetGeocode(query GeocodeQuery) (lat float64, lng float64, exist bool) {
	redisKey := "cities"
	redisField := strings.ToLower(strings.Join([]string{query.City, query.Country}, "_"))
	geocode, err := redisClient.client.HGet(redisKey, redisField).Result()
	if err != nil {
		log.Infof("geocode of location %s, %s does not exist in cache", query.City, query.Country)
		return // location does not exist
	}
	latLng, _ := utils.ParseLocation(geocode)
	lat = latLng[0]
	lng = latLng[1]
	exist = true
	return
}

func (redisClient *RedisClient) SetGeocode(query GeocodeQuery, lat float64, lng float64) bool {
	redisKey := "cities"
	redisField := strings.ToLower(strings.Join([]string{query.City, query.Country}, "_"))
	redisVal := strings.Join([]string{fmt.Sprintf("%.6f", lat), fmt.Sprintf("%.6f", lng)}, ",") // 1/9 meter precision
	res, err := redisClient.client.HSet(redisKey, redisField, redisVal).Result()
	utils.CheckErrImmediate(err, utils.LogError)
	if res {
		log.Infof("Cached geolocation for location %s, %s success", query.City, query.Country)
	}
	return res
}

// returns redis streams ID if XADD command execution is successful
func (redisClient *RedisClient) StreamsLogging(streamName string, data map[string]string) string {
	xArgs := redis.XAddArgs{Stream: streamName}
	xArgs.Values = make(map[string]interface{}, 0)
	for field, value := range data {
		xArgs.Values[field] = strings.ToLower(value)
	}
	streamsId, err := redisClient.client.XAdd(&xArgs).Result()
	if err != nil {
		log.Info(err)
	}
	return streamsId
}

type SlotSolutionCandidateCache struct {
	PlaceIds       []string     `json:"place_ids"`
	Score          float64      `json:"score"`
	PlaceNames     []string     `json:"place_names"`
	PlaceLocations [][2]float64 `json:"place_locations"`
}

type SlotSolutionCacheResponse struct {
	SlotSolutionCandidate []SlotSolutionCandidateCache `json:"slot_solution_candidate"`
}

type SlotSolutionCacheRequest struct {
	Country   string
	City      string
	Radius    uint64
	EVTags    []string
	Intervals []POI.TimeInterval
	Weekday   POI.Weekday
}

// convert time intervals and an EV tag to an integer
// each time interval and E/V pair has 23 * 24 * 2 = 1104 possibilities
// treat each pair as one digit in 1104-ary number and we have maximum 4 digits
func encodeTimeCatIdx(eVTag []string, intervals []POI.TimeInterval) (res int64, err error) {
	if len(eVTag) != len(intervals) {
		err = errors.New("wrong inputs")
		res = -1
		return
	}
	for idx, tagVal := range eVTag {
		res *= 1104
		interval := intervals[idx]
		if strings.ToLower(tagVal) == "e" {
			res += int64(interval.Start) * int64(interval.End)
		} else if strings.ToLower(tagVal) == "v" {
			res += int64(interval.Start) * int64(interval.End) * 2
		} else {
			err = errors.New("wrong input EV tag")
			res = -1
			return
		}
	}
	return
}

func genSlotSolutionCacheKey(req SlotSolutionCacheRequest) string {
	country, city := req.Country, req.City
	timeCatIdx, err := encodeTimeCatIdx(req.EVTags, req.Intervals)
	utils.CheckErrImmediate(err, utils.LogError)

	radius := strconv.FormatUint(req.Radius, 10)
	timeCatIdxStr := strconv.FormatInt(timeCatIdx, 10)

	redisFieldKey := strings.Join([]string{"slot_solution", country, city, radius, string(req.Weekday), timeCatIdxStr}, ":")
	return redisFieldKey
}

// cache iowrapper level version of slot solution
func (redisClient *RedisClient) CacheSlotSolution(req SlotSolutionCacheRequest, solution SlotSolutionCacheResponse) {
	redisKey := genSlotSolutionCacheKey(req)
	json_, err := json.Marshal(solution)
	utils.CheckErrImmediate(err, utils.LogError)

	if err != nil {
		log.Errorf("cache slot solution failure for request with key: %s", redisKey)
	} else {
		redisClient.client.Set(redisKey, json_, SlotSolutionExpirationTime)
	}
}

func (redisClient *RedisClient) GetSlotSolution(req SlotSolutionCacheRequest) (solution SlotSolutionCacheResponse, err error) {
	redisKey := genSlotSolutionCacheKey(req)
	json_, err := redisClient.client.Get(redisKey).Result()
	if err != nil {
		log.Errorf("get slot solution cache failure for request with key: %s", redisKey)
		return
	}
	err = json.Unmarshal([]byte(json_), &solution)
	return
}
