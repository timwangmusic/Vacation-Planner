package iowrappers

// RedisClient is a type wrapping-up over functionality defined in the go-redis library
// serving the caching needs of the Vacation Planner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-redis/redis/v8"
	log "github.com/sirupsen/logrus"
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/utils"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	SlotSolutionExpirationTime = 24 * time.Hour
	PlanningStatExpirationTime = 24 * time.Hour

	NumVisitorsPlanningAPI = "visitor_count:planning_APIs"
	NumVisitorsPrefix      = "visitor_count"
)

var RedisClientContext context.Context

func init() {
	RedisClientContext = context.Background()
}

type RedisClient struct {
	client redis.Client
}

// close Redis connection
func (redisClient *RedisClient) Destroy() {
	if err := redisClient.client.Close(); err != nil {
		log.Error(err)
	}
}

// CreateRedisClient is a factory method for RedisClient
func CreateRedisClient(url *url.URL) RedisClient {
	password, _ := url.User.Password()
	return RedisClient{client: *redis.NewClient(&redis.Options{
		Addr:     url.Host,
		Password: password,
	})}
}

// analytics of total number of unique visitors to the planning APIs in the last 24 hours
// analytics of number of unique users planning for each city
func (redisClient *RedisClient) CollectPlanningAPIStats(event PlanningEvent) {
	c := redisClient.client

	pipeline := c.Pipeline()

	pipeline.PFAdd(RedisClientContext, NumVisitorsPlanningAPI, event.User)

	// set expiration time
	if _, err := pipeline.Exists(RedisClientContext, NumVisitorsPlanningAPI).Result(); err != nil && err == redis.Nil {
		pipeline.Expire(RedisClientContext, NumVisitorsPlanningAPI, PlanningStatExpirationTime)
	}

	city := strings.ToLower(strings.Join(strings.Split(event.City, " "), "_"))
	redisKey := strings.Join([]string{NumVisitorsPrefix, event.Country, city}, ":")
	pipeline.PFAdd(RedisClientContext, redisKey, event.User)

	if _, err := pipeline.Exec(RedisClientContext); err != nil {
		log.Error(err)
	}
}

func (redisClient *RedisClient) RemoveKeys(keys []string) {
	redisClient.client.Del(RedisClientContext, keys...)
}

// serialize place using JSON and store in Redis with key place_details:place_ID:placeID
func (redisClient *RedisClient) cachePlace(place POI.Place, wg *sync.WaitGroup) {
	defer wg.Done()
	json_, err := json.Marshal(place)
	utils.CheckErrImmediate(err, utils.LogError)

	_, err = redisClient.client.Set(RedisClientContext, "place_details:place_ID:"+place.ID, json_, -1).Result()
	if err != nil {
		Logger.Error(err)
	}
}

func (redisClient *RedisClient) GetMapsLastSearchTime(location string, category POI.PlaceCategory) (lastSearchTime time.Time, err error) {
	redisKey := "MapsLastSearchTime"
	cityCountry := strings.Split(location, ",")
	city, country := cityCountry[0], cityCountry[1]
	redisField := strings.ToLower(strings.Join([]string{country, city, string(category)}, ":"))
	lst, cacheErr := redisClient.client.HGet(RedisClientContext, redisKey, redisField).Result()
	if cacheErr != nil {
		err = cacheErr
		return
	}

	ParsedLastSearchTime, parseErr := time.Parse(time.RFC3339, lst)
	if parseErr != nil {
		err = parseErr
		return
	}
	lastSearchTime = ParsedLastSearchTime
	return
}

func (redisClient *RedisClient) SetMapsLastSearchTime(location string, category POI.PlaceCategory, requestTime string) (err error) {
	redisKey := "MapsLastSearchTime"
	cityCountry := strings.Split(location, ",")
	city, country := cityCountry[0], cityCountry[1]
	redisField := strings.ToLower(strings.Join([]string{country, city, string(category)}, ":"))
	_, err = redisClient.client.HSet(RedisClientContext, redisKey, redisField, requestTime).Result()
	return
}

// currently not used, but it is still a primitive implementation that might have faster search time compared
// with all places stored under one key
// store places obtained from database or external API in Redis
// places for a location are stored in separate sorted sets based on category
func (redisClient *RedisClient) StorePlacesForLocation(geocodeInString string, places []POI.Place) error {
	client := redisClient.client
	latLng, _ := utils.ParseLocation(geocodeInString)
	lat, lng := latLng[0], latLng[1]
	wg := &sync.WaitGroup{}
	wg.Add(len(places))
	for _, place := range places {
		sortedSetKey := strings.Join([]string{geocodeInString, string(POI.GetPlaceCategory(place.LocationType))}, "_")
		dist := utils.HaversineDist([]float64{lng, lat}, place.Location.Coordinates[:])
		_, err := client.ZAdd(RedisClientContext, sortedSetKey, &redis.Z{Score: dist, Member: place.ID}).Result()
		if err != nil {
			return err
		}
		redisClient.cachePlace(place, wg)
	}
	wg.Wait()
	return nil
}

func (redisClient *RedisClient) SetPlacesOnCategory(places []POI.Place) {
	wg := &sync.WaitGroup{}
	wg.Add(len(places))
	for _, place := range places {
		placeCategory := POI.GetPlaceCategory(place.LocationType)
		geolocation := &redis.GeoLocation{
			Name:      place.ID,
			Longitude: place.Location.Coordinates[0],
			Latitude:  place.Location.Coordinates[1],
		}
		redisKey := "placeIDs:" + strings.ToLower(string(placeCategory))
		_, cmdErr := redisClient.client.GeoAdd(RedisClientContext, redisKey, geolocation).Result()

		utils.CheckErrImmediate(cmdErr, utils.LogError)

		redisClient.cachePlace(place, wg)
	}
	wg.Wait()
}

// obtain place info from Redis based with key place_details:place_ID:placeID
func (redisClient *RedisClient) getPlace(placeId string) (place POI.Place, err error) {
	res, err := redisClient.client.Get(RedisClientContext, "place_details:place_ID:"+placeId).Result()
	utils.CheckErrImmediate(err, utils.LogError)
	if err != nil {
		return
	}
	utils.CheckErrImmediate(json.Unmarshal([]byte(res), &place), utils.LogError)
	return
}

// currently NOT used
// to be used with the StorePlacesForLocation method
// if no geocode in Redis, then we assume no nearby place exists either
func (redisClient *RedisClient) NearbySearchNotUsed(request *PlaceSearchRequest) ([]POI.Place, error) {
	cityCountry := strings.Split(request.Location, ",")
	lat, lng, err := redisClient.GetGeocode(&GeocodeQuery{
		City:    cityCountry[0],
		Country: cityCountry[1],
	})
	if err != nil {
		return nil, err
	}
	latLng := strings.Join([]string{fmt.Sprintf("%f", lat), fmt.Sprintf("%f", lng)}, ",")
	sortedSetKey := strings.Join([]string{latLng, string(request.PlaceCat)}, "_")

	placeIds, _ := redisClient.client.ZRangeByScore(RedisClientContext, sortedSetKey, &redis.ZRangeBy{
		Min: "0",
		Max: fmt.Sprintf("%d", request.Radius),
	}).Result()

	res := make([]POI.Place, len(placeIds))

	for idx, placeId := range placeIds {
		res[idx], _ = redisClient.getPlace(placeId)
	}
	return res, nil
}

func (redisClient *RedisClient) NearbySearch(request *PlaceSearchRequest) (places []POI.Place, err error) {
	requestCategory := strings.ToLower(string(request.PlaceCat))
	redisKey := "placeIDs:" + requestCategory

	latLng, _ := utils.ParseLocation(request.Location)
	requestLat, requestLng := latLng[0], latLng[1]

	searchRadius := request.Radius

	if searchRadius > MaxSearchRadius {
		searchRadius = MaxSearchRadius
	}

	Logger.Debugf("Redis geo radius is using search radius of %d meters", searchRadius)
	geoQuery := redis.GeoRadiusQuery{
		Radius: float64(searchRadius),
		Unit:   "m",
		Sort:   "ASC", // sort ascending
	}
	var cachedQualifiedPlaces []redis.GeoLocation
	cachedQualifiedPlaces, err = redisClient.client.GeoRadius(RedisClientContext, redisKey, requestLng, requestLat, &geoQuery).Result()
	if err != nil {
		Logger.Error(err)
		return
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

func (redisClient *RedisClient) PlaceDetailsSearch(string) (place POI.Place, err error) {
	return
}

// cache the mapping from user input location name to geo-coding-corrected location name
// correct location name is an alias of itself
func (redisClient *RedisClient) CacheLocationAlias(query GeocodeQuery, correctedQuery GeocodeQuery) (err error) {
	_, err = redisClient.client.HSet(RedisClientContext, "location_name_alias_mapping:city_names", strings.ToLower(query.City), strings.ToLower(correctedQuery.City)).Result()
	if err != nil {
		return
	}
	_, err = redisClient.client.HSet(RedisClientContext, "location_name_alias_mapping:country_names", strings.ToLower(query.Country), strings.ToLower(correctedQuery.Country)).Result()
	if err != nil {
		return
	}
	return
}

// retrieve corrected location name from cache. return empty string if not exist
// if corrected location name exists, corrects geocode query
func (redisClient *RedisClient) GetLocationWithAlias(query *GeocodeQuery) string {
	resCity, err := redisClient.client.HGet(RedisClientContext, "location_name_alias_mapping:city_names", strings.ToLower(query.City)).Result()
	if err != nil {
		return ""
	}

	resCountry, err := redisClient.client.HGet(RedisClientContext, "location_name_alias_mapping:country_names", strings.ToLower(query.Country)).Result()
	if err != nil {
		return ""
	}

	query.Country = resCountry
	query.City = resCity
	return strings.Join([]string{resCity, resCountry}, "_")
}

func (redisClient *RedisClient) GetGeocode(query *GeocodeQuery) (lat float64, lng float64, err error) {
	redisKey := "geocode:cities"
	redisField := redisClient.GetLocationWithAlias(query)
	errMsg := fmt.Errorf("geocode of location %s, %s does not exist in cache", query.City, query.Country)
	if redisField == "" {
		err = errMsg
		return
	}
	var geocode string
	geocode, err = redisClient.client.HGet(RedisClientContext, redisKey, redisField).Result()
	if err != nil {
		err = errMsg
		return
	}
	latLng, _ := utils.ParseLocation(geocode)
	lat = latLng[0]
	lng = latLng[1]
	return
}

func (redisClient *RedisClient) SetGeocode(query GeocodeQuery, lat float64, lng float64, originalQuery GeocodeQuery) {
	redisKey := "geocode:cities"
	redisField := strings.ToLower(strings.Join([]string{query.City, query.Country}, "_"))
	redisVal := strings.Join([]string{fmt.Sprintf("%.6f", lat), fmt.Sprintf("%.6f", lng)}, ",") // 1/9 meter precision
	_, err := redisClient.client.HSet(context.Background(), redisKey, redisField, redisVal).Result()
	utils.CheckErrImmediate(err, utils.LogError)
	if err != nil {
		Logger.Errorf("Failed to cache geolocation for location %s, %s", query.City, query.Country)
	} else {
		Logger.Infof("Cached geolocation for location %s, %s success", query.City, query.Country)
	}
	utils.CheckErrImmediate(redisClient.CacheLocationAlias(originalQuery, query), utils.LogError)
}

// returns redis streams ID if XADD command execution is successful
func (redisClient *RedisClient) StreamsLogging(streamName string, data map[string]string) string {
	xArgs := redis.XAddArgs{Stream: streamName}
	xArgs.Values = data
	streamsId, err := redisClient.client.XAdd(RedisClientContext, &xArgs).Result()
	if err != nil {
		Logger.Info(err)
	}
	return streamsId
}

type SlotSolutionCandidateCache struct {
	PlaceIds       []string     `json:"place_ids"`
	Score          float64      `json:"score"`
	PlaceNames     []string     `json:"place_names"`
	PlaceLocations [][2]float64 `json:"place_locations"`
	PlaceAddresses []string     `json:"place_addresses"`
	PlaceURLs      []string     `json:"place_urls"`
}

type SlotSolutionCacheResponse struct {
	SlotSolutionCandidate []SlotSolutionCandidateCache `json:"slot_solution_candidate"`
	Err                   error
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

	redisFieldKey := strings.ToLower(strings.Join([]string{"slot_solution", country, city, radius, string(req.Weekday), timeCatIdxStr}, ":"))
	return redisFieldKey
}

// cache iowrapper level version of slot solution
func (redisClient *RedisClient) CacheSlotSolution(req SlotSolutionCacheRequest, solution SlotSolutionCacheResponse) {
	redisKey := genSlotSolutionCacheKey(req)
	json_, err := json.Marshal(solution)
	utils.CheckErrImmediate(err, utils.LogError)

	if err != nil {
		Logger.Errorf("cache slot solution failure for request with key: %s", redisKey)
	} else {
		redisClient.client.Set(RedisClientContext, redisKey, json_, SlotSolutionExpirationTime)
	}
}

func (redisClient *RedisClient) GetSlotSolution(redisKey string, cacheResponses []SlotSolutionCacheResponse, wg *sync.WaitGroup, idx int) {
	defer wg.Done()

	json_, err := redisClient.client.Get(RedisClientContext, redisKey).Result()
	if err != nil {
		Logger.Debugf("redis server find no result for key: %s", redisKey)
		cacheResponses[idx].Err = err
		return
	}

	err = json.Unmarshal([]byte(json_), &cacheResponses[idx])
	if err != nil {
		Logger.Error(err)
		cacheResponses[idx].Err = err
		return
	}
}

func (redisClient *RedisClient) GetMultiSlotSolutions(requests []SlotSolutionCacheRequest) (responses []SlotSolutionCacheResponse) {
	var wg sync.WaitGroup
	wg.Add(len(requests))

	responses = make([]SlotSolutionCacheResponse, len(requests))

	for idx, request := range requests {
		redisKey := genSlotSolutionCacheKey(request)
		go redisClient.GetSlotSolution(redisKey, responses, &wg, idx)
	}
	wg.Wait()
	return
}
