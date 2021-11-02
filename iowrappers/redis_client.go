package iowrappers

// RedisClient is a type wrapping-up over functionality defined in the go-redis library
// serving the caching needs of the Vacation Planner

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	log "github.com/sirupsen/logrus"
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/utils"
)

const (
	PlanningSolutionsExpirationTime = 24 * time.Hour
	PlanningStatExpirationTime      = 24 * time.Hour

	MaximumNumSlotsPerPlan = 5

	NumVisitorsPlanningAPI         = "visitor_count:planning_APIs"
	NumVisitorsPrefix              = "visitor_count"
	TravelPlansRedisCacheKeyPrefix = "travel_plans"
	TravelPlanRedisCacheKeyPrefix  = "travel_plan"
)

var RedisClientDefaultBlankContext context.Context

func init() {
	RedisClientDefaultBlankContext = context.Background()
}

type RedisClient struct {
	client redis.Client
}

// Destroy closes Redis connection from the client
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

// CollectPlanningAPIStats generates analytics of total number of unique visitors to the planning APIs in the last 24 hours
// analytics of number of unique users planning for each city
func (redisClient *RedisClient) CollectPlanningAPIStats(event PlanningEvent) {
	c := redisClient.client

	pipeline := c.Pipeline()

	pipeline.PFAdd(RedisClientDefaultBlankContext, NumVisitorsPlanningAPI, event.User)

	// set expiration time
	if _, err := pipeline.Exists(RedisClientDefaultBlankContext, NumVisitorsPlanningAPI).Result(); err != nil && err == redis.Nil {
		pipeline.Expire(RedisClientDefaultBlankContext, NumVisitorsPlanningAPI, PlanningStatExpirationTime)
	}

	city := strings.ReplaceAll(strings.ToLower(event.City), " ", "_")

	redisKey := strings.Join([]string{NumVisitorsPrefix, event.Country, city}, ":")
	pipeline.PFAdd(RedisClientDefaultBlankContext, redisKey, event.User)

	if _, err := pipeline.Exec(RedisClientDefaultBlankContext); err != nil {
		log.Error(err)
	}
}

func (redisClient *RedisClient) RemoveKeys(context context.Context, keys []string) {
	redisClient.client.Del(context, keys...)
}

// serialize place using JSON and store in Redis with key place_details:place_ID:placeID
func (redisClient *RedisClient) setPlace(context context.Context, place POI.Place, wg *sync.WaitGroup) {
	defer wg.Done()
	json_, err := json.Marshal(place)
	utils.LogErrorWithLevel(err, utils.LogError)

	_, err = redisClient.client.Set(context, "place_details:place_ID:"+place.ID, json_, 0).Result()
	if err != nil {
		Logger.Error(err)
	}
}

func (redisClient *RedisClient) GetMapsLastSearchTime(context context.Context, location POI.Location, category POI.PlaceCategory) (lastSearchTime time.Time, err error) {
	redisKey := "MapsLastSearchTime"

	redisField := strings.ToLower(strings.Join([]string{location.Country, location.City, string(category)}, ":"))
	lst, cacheErr := redisClient.client.HGet(context, redisKey, redisField).Result()
	if cacheErr != nil {
		err = cacheErr
		return
	}

	ParsedLastSearchTime, timeParsingErr := time.Parse(time.RFC3339, lst)
	if timeParsingErr != nil {
		utils.LogErrorWithLevel(timeParsingErr, utils.LogError)
	}
	lastSearchTime = ParsedLastSearchTime
	return
}

func (redisClient *RedisClient) SetMapsLastSearchTime(context context.Context, location POI.Location, category POI.PlaceCategory, requestTime string) (err error) {
	redisKey := "MapsLastSearchTime"
	redisField := strings.ToLower(strings.Join([]string{location.Country, location.City, string(category)}, ":"))
	_, err = redisClient.client.HSet(context, redisKey, redisField, requestTime).Result()
	return
}

// StorePlacesForLocation is currently not used, but it is still a primitive implementation that might have faster search time compared
// with all places stored under one key
// store places obtained from database or external API in Redis
// places for a location are stored in separate sorted sets based on category
func (redisClient *RedisClient) StorePlacesForLocation(context context.Context, geocodeInString string, places []POI.Place) error {
	client := redisClient.client
	latLng, _ := utils.ParseLocation(geocodeInString)
	lat, lng := latLng[0], latLng[1]
	wg := &sync.WaitGroup{}
	wg.Add(len(places))
	for _, place := range places {
		sortedSetKey := strings.Join([]string{geocodeInString, string(POI.GetPlaceCategory(place.LocationType))}, "_")
		dist := utils.HaversineDist([]float64{lat, lng}, []float64{place.GetLocation().Latitude, place.GetLocation().Longitude})
		_, err := client.ZAdd(context, sortedSetKey, &redis.Z{Score: dist, Member: place.ID}).Result()
		if err != nil {
			return err
		}
		redisClient.setPlace(context, place, wg)
	}
	wg.Wait()
	return nil
}

func (redisClient *RedisClient) SetPlacesOnCategory(context context.Context, places []POI.Place) {
	wg := &sync.WaitGroup{}
	wg.Add(len(places))
	for _, place := range places {
		placeCategory := POI.GetPlaceCategory(place.LocationType)
		geolocation := &redis.GeoLocation{
			Name:      place.ID,
			Latitude:  place.GetLocation().Latitude,
			Longitude: place.GetLocation().Longitude,
		}
		redisKey := "placeIDs:" + strings.ToLower(string(placeCategory))
		_, cmdErr := redisClient.client.GeoAdd(context, redisKey, geolocation).Result()

		utils.LogErrorWithLevel(cmdErr, utils.LogError)

		redisClient.setPlace(context, place, wg)
	}
	wg.Wait()
}

// obtain place info from Redis based with key place_details:place_ID:placeID
func (redisClient *RedisClient) getPlace(context context.Context, placeId string) (place POI.Place, err error) {
	res, err := redisClient.client.Get(context, "place_details:place_ID:"+placeId).Result()
	utils.LogErrorWithLevel(err, utils.LogError)
	if err != nil {
		return
	}
	utils.LogErrorWithLevel(json.Unmarshal([]byte(res), &place), utils.LogError)
	return
}

// NearbySearchNotUsed is currently NOT used
// to be used with the StorePlacesForLocation method
// if no geocode in Redis, then we assume no nearby place exists either
func (redisClient *RedisClient) NearbySearchNotUsed(context context.Context, request *PlaceSearchRequest) ([]POI.Place, error) {
	lat, lng, err := redisClient.Geocode(context, &GeocodeQuery{
		City:    request.Location.City,
		Country: request.Location.Country,
	})
	if err != nil {
		return nil, err
	}
	latLng := strings.Join([]string{fmt.Sprintf("%f", lat), fmt.Sprintf("%f", lng)}, ",")
	sortedSetKey := strings.Join([]string{latLng, string(request.PlaceCat)}, "_")

	placeIds, _ := redisClient.client.ZRangeByScore(context, sortedSetKey, &redis.ZRangeBy{
		Min: "0",
		Max: fmt.Sprintf("%d", request.Radius),
	}).Result()

	res := make([]POI.Place, len(placeIds))

	for idx, placeId := range placeIds {
		res[idx], _ = redisClient.getPlace(context, placeId)
	}
	return res, nil
}

func (redisClient *RedisClient) NearbySearch(context context.Context, request *PlaceSearchRequest) (places []POI.Place, err error) {
	requestCategory := strings.ToLower(string(request.PlaceCat))
	redisKey := "placeIDs:" + requestCategory

	requestLat, requestLng := request.Location.Latitude, request.Location.Longitude

	searchRadius := request.Radius

	if searchRadius > MaxSearchRadius {
		searchRadius = MaxSearchRadius
	}

	Logger.Debugf("[%s] Redis geo radius is using search radius of %d meters", context.Value(RequestIdKey), searchRadius)
	geoQuery := redis.GeoRadiusQuery{
		Radius: float64(searchRadius),
		Unit:   "m",
		Sort:   "ASC", // sort ascending
	}
	var cachedQualifiedPlaces []redis.GeoLocation
	cachedQualifiedPlaces, err = redisClient.client.GeoRadius(context, redisKey, requestLng, requestLat, &geoQuery).Result()
	if err != nil {
		Logger.Error(err)
		return
	}

	request.Radius = searchRadius

	places = make([]POI.Place, 0)
	for _, placeInfo := range cachedQualifiedPlaces {
		place, err := redisClient.getPlace(context, placeInfo.Name)
		if err == nil {
			places = append(places, place)
		}
	}
	return
}

// CacheLocationAlias caches the mapping from user input location name to geo-coding-corrected location name
// correct location name is an alias of itself
func (redisClient *RedisClient) CacheLocationAlias(context context.Context, query GeocodeQuery, correctedQuery GeocodeQuery) (err error) {
	_, err = redisClient.client.HSet(context, "location_name_alias_mapping:city_names", strings.ToLower(query.City), strings.ToLower(correctedQuery.City)).Result()
	if err != nil {
		return
	}
	_, err = redisClient.client.HSet(context, "location_name_alias_mapping:admin_area_level_one_names", strings.ToLower(query.AdminAreaLevelOne), strings.ToLower(correctedQuery.AdminAreaLevelOne)).Result()
	if err != nil {
		return
	}
	_, err = redisClient.client.HSet(context, "location_name_alias_mapping:country_names", strings.ToLower(query.Country), strings.ToLower(correctedQuery.Country)).Result()
	if err != nil {
		return
	}
	return
}

// GetLocationWithAlias retrieves corrected location name from Redis; returns empty string if not exist;
// corrects geocode query if corrected location name exists
func (redisClient *RedisClient) GetLocationWithAlias(context context.Context, query *GeocodeQuery) string {
	var err error
	resCity, err := redisClient.client.HGet(context, "location_name_alias_mapping:city_names", strings.ToLower(query.City)).Result()
	if err != nil {
		return ""
	}

	resCountry, err := redisClient.client.HGet(context, "location_name_alias_mapping:country_names", strings.ToLower(query.Country)).Result()
	if err != nil {
		return ""
	}

	resAdminAreaLevelOne, err := redisClient.client.HGet(context, "location_name_alias_mapping:admin_area_level_one_names", strings.ToLower(query.AdminAreaLevelOne)).Result()
	if err != nil {
		return ""
	}

	query.Country = resCountry
	query.City = resCity
	query.AdminAreaLevelOne = resAdminAreaLevelOne
	return strings.Join([]string{resCity, resAdminAreaLevelOne, resCountry}, "_")
}

func (redisClient *RedisClient) Geocode(context context.Context, query *GeocodeQuery) (lat float64, lng float64, err error) {
	redisKey := "geocode:cities"
	redisField := redisClient.GetLocationWithAlias(context, query)
	errMsg := fmt.Errorf("geocode of location %s, %s, %s does not exist in cache", query.City, query.AdminAreaLevelOne, query.Country)
	if redisField == "" {
		err = errMsg
		return
	}
	var geocode string
	geocode, err = redisClient.client.HGet(context, redisKey, redisField).Result()
	if err != nil {
		err = errMsg
		return
	}
	latLng, _ := utils.ParseLocation(geocode)
	lat = latLng[0]
	lng = latLng[1]
	return
}

func (redisClient *RedisClient) SetGeocode(context context.Context, query GeocodeQuery, lat float64, lng float64, originalQuery GeocodeQuery) {
	redisKey := "geocode:cities"
	redisHashField := strings.ToLower(strings.Join([]string{query.City, query.AdminAreaLevelOne, query.Country}, "_"))
	redisHashVal := strings.Join([]string{fmt.Sprintf("%.6f", lat), fmt.Sprintf("%.6f", lng)}, ",") // 1/9 meter precision
	_, err := redisClient.client.HSet(context, redisKey, redisHashField, redisHashVal).Result()
	utils.LogErrorWithLevel(err, utils.LogError)
	if err != nil {
		Logger.Errorf("Failed to cache geolocation for location %s, %s with error %s", query.City, query.Country, err.Error())
		return
	} else {
		Logger.Debugf("Cached geolocation for location %s, %s success", query.City, query.Country)
	}
	utils.LogErrorWithLevel(redisClient.CacheLocationAlias(context, originalQuery, query), utils.LogError)
}

// StreamsLogging returns redis streams ID if XADD command execution is successful
func (redisClient *RedisClient) StreamsLogging(streamName string, data map[string]string) string {
	xArgs := redis.XAddArgs{Stream: streamName}
	keyValues := make([]string, 0)
	for key, val := range data {
		keyValues = append(keyValues, []string{key, val}...)
	}
	xArgs.Values = keyValues
	streamsId, err := redisClient.client.XAdd(RedisClientDefaultBlankContext, &xArgs).Result()
	if err != nil {
		Logger.Error(err)
	}
	return streamsId
}

type PlanningSolutionRecord struct {
	ID              string              `json:"id"`
	PlaceIDs        []string            `json:"place_ids"`
	Score           float64             `json:"score"`
	PlaceNames      []string            `json:"place_names"`
	PlaceLocations  [][2]float64        `json:"place_locations"`
	PlaceAddresses  []string            `json:"place_addresses"`
	PlaceURLs       []string            `json:"place_urls"`
	PlaceCategories []POI.PlaceCategory `json:"place_categories"`
}

type PlanningSolutionsResponse struct {
	PlanningSolutionRecords []PlanningSolutionRecord `json:"cached_planning_solutions"`
}

type PlanningSolutionsCacheRequest struct {
	Location        POI.Location
	Radius          uint64
	PlaceCategories []POI.PlaceCategory
	Intervals       []POI.TimeInterval
	Weekday         POI.Weekday
}

// convert time intervals and place categories of a travel plan into an unsigned integer
// a time interval and place category has 23 * 24 * 2 = 1104 possibilities
// treat each combination as one digit in a 1104-ary number
// TODO: [NOTE] that the maximum number of slots it can hold is approximately 5, this encoding should be improved in the future
func encodePlanIndex(placeCategories []POI.PlaceCategory, intervals []POI.TimeInterval) (uint64, error) {
	var result uint64
	if len(placeCategories) != len(intervals) {
		return 0, fmt.Errorf("the size of place category is %d, which does not match the size of intervals %d", len(placeCategories), len(intervals))
	}

	if len(placeCategories) > MaximumNumSlotsPerPlan {
		return 0, fmt.Errorf("the number of time slots in the plan is %d, which exceeds the limit of %d", len(placeCategories), MaximumNumSlotsPerPlan)
	}

	for idx, placeCategory := range placeCategories {
		result *= 1104
		interval := intervals[idx]
		switch placeCategory {
		case POI.PlaceCategoryEatery:
			result += uint64(interval.Start) * uint64(interval.End)
		case POI.PlaceCategoryVisit:
			result += uint64(interval.Start) * uint64(interval.End) * 2
		}
	}
	return result, nil
}

func generateTravelPlansCacheKey(req PlanningSolutionsCacheRequest) (string, error) {
	country, region, city := req.Location.Country, req.Location.AdminAreaLevelOne, req.Location.City
	planIndex, err := encodePlanIndex(req.PlaceCategories, req.Intervals)
	if err != nil {
		return "", err
	}

	radius := strconv.FormatUint(req.Radius, 10)
	planIndexStr := strconv.FormatUint(planIndex, 10)

	country = strings.ReplaceAll(strings.ToLower(country), " ", "_")
	region = strings.ReplaceAll(strings.ToLower(region), " ", "_")
	city = strings.ReplaceAll(strings.ToLower(city), " ", "_")

	redisFieldKey := strings.ToLower(strings.Join([]string{TravelPlansRedisCacheKeyPrefix, country, region, city, radius, strconv.Itoa(int(req.Weekday)), planIndexStr}, ":"))
	return redisFieldKey, nil
}

func (redisClient *RedisClient) SavePlanningSolutions(context context.Context, request PlanningSolutionsCacheRequest, response PlanningSolutionsResponse) (string, error) {
	redisListKey, keyGenerationErr := generateTravelPlansCacheKey(request)
	if keyGenerationErr != nil {
		Logger.Errorf("failed to generate travel plans cache key, error %s", keyGenerationErr.Error())
		return redisListKey, keyGenerationErr
	}

	var recordKeys []string
	for _, record := range response.PlanningSolutionRecords {
		solutionRedisKey := strings.Join([]string{TravelPlanRedisCacheKeyPrefix, record.ID}, ":")
		json_, err := json.Marshal(record)
		if err != nil {
			return redisListKey, err
		}
		_, recordSaveErr := redisClient.client.Set(context, solutionRedisKey, json_, 0).Result()
		if recordSaveErr != nil {
			return redisListKey, recordSaveErr
		}
		recordKeys = append(recordKeys, solutionRedisKey)
	}

	numTravelPlanKeys, listSaveErr := redisClient.client.LPush(context, redisListKey, recordKeys).Result()
	Logger.Debugf("added the %d travel plan keys to %s", numTravelPlanKeys, redisListKey)
	redisClient.client.Expire(context, redisListKey, PlanningSolutionsExpirationTime)

	return redisListKey, listSaveErr
}

func (redisClient *RedisClient) PlanningSolutions(context context.Context, request PlanningSolutionsCacheRequest) (PlanningSolutionsResponse, error) {
	var response PlanningSolutionsResponse
	redisListKey, keyGenerationErr := generateTravelPlansCacheKey(request)
	if keyGenerationErr != nil {
		Logger.Error(keyGenerationErr)
		return response, keyGenerationErr
	}

	exists, _ := redisClient.client.Exists(context, redisListKey).Result()
	if exists == 0 {
		return response, fmt.Errorf("redis key %s does not exist", redisListKey)
	}

	recordKeys, listFetchErr := redisClient.client.LRange(context, redisListKey, 0, -1).Result()
	if listFetchErr != nil {
		Logger.Error(listFetchErr)
		return response, listFetchErr
	}

	response.PlanningSolutionRecords = make([]PlanningSolutionRecord, len(recordKeys))
	for idx, key := range recordKeys {
		json_, err := redisClient.client.Get(context, key).Result()
		if err != nil {
			return response, err
		}

		err = json.Unmarshal([]byte(json_), &response.PlanningSolutionRecords[idx])
		if err != nil {
			return response, err
		}
	}

	return response, nil
}
