package iowrappers

// RedisClient is a type wrapping-up over functionality defined in the go-redis library
// serving the caching needs of the Vacation Planner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/utils"
)

const (
	PlanningSolutionsExpirationTime = 24 * time.Hour
	PlanningStatExpirationTime      = 24 * time.Hour
	CityInfoExpirationTime          = 0

	MaximumNumSlotsPerPlan = 5

	NumVisitorsPlanningAPI         = "visitor_count:planning_APIs"
	NumVisitorsPrefix              = "visitor_count"
	TravelPlansRedisCacheKeyPrefix = "travel_plans"
	TravelPlanRedisCacheKeyPrefix  = "travel_plan"
	CityRedisKeyPrefix             = "city"
	CitiesRedisKey                 = "known_cities_ids"
	KnownCitiesHashMapRedisKey     = "known_cities_name_to_id"
)

var RedisClientDefaultBlankContext context.Context

func init() {
	RedisClientDefaultBlankContext = context.Background()
}

type RedisClient struct {
	client redis.Client
}

func (r *RedisClient) Get() *redis.Client {
	return &r.client
}

// Destroy closes Redis connection from the client
func (r *RedisClient) Destroy() {
	if err := r.client.Close(); err != nil {
		log.Error(err)
	}
}

// CreateRedisClient is a factory method for RedisClient
func CreateRedisClient(url *url.URL) *RedisClient {
	password, _ := url.User.Password()
	return &RedisClient{client: *redis.NewClient(&redis.Options{
		Addr:     url.Host,
		Password: password,
	})}
}

// CollectPlanningAPIStats generates analytics of total number of unique visitors to the planning APIs in the last 24 hours
// analytics of number of unique users planning for each city
func (r *RedisClient) CollectPlanningAPIStats(event PlanningEvent) {
	c := r.client

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

func (r *RedisClient) RemoveKeys(context context.Context, keys []string) (err error) {
	_, err = r.client.Del(context, keys...).Result()
	return err
}

// serialize place using JSON and store in Redis with key place_details:place_ID:placeID
func (r *RedisClient) setPlace(context context.Context, place POI.Place, wg *sync.WaitGroup) {
	defer wg.Done()
	json_, err := json.Marshal(place)
	utils.LogErrorWithLevel(err, utils.LogError)

	_, err = r.client.Set(context, "place_details:place_ID:"+place.ID, json_, 0).Result()
	if err != nil {
		Logger.Error(err)
	}
}

func (r *RedisClient) GetMapsLastSearchTime(context context.Context, location POI.Location, category POI.PlaceCategory) (lastSearchTime time.Time, err error) {
	redisKey := "MapsLastSearchTime"

	redisField := strings.ToLower(strings.Join([]string{location.Country, location.City, string(category)}, ":"))
	lst, cacheErr := r.client.HGet(context, redisKey, redisField).Result()
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

func (r *RedisClient) SetMapsLastSearchTime(context context.Context, location POI.Location, category POI.PlaceCategory, requestTime string) (err error) {
	redisKey := "MapsLastSearchTime"
	redisField := strings.ToLower(strings.Join([]string{location.Country, location.City, string(category)}, ":"))
	_, err = r.client.HSet(context, redisKey, redisField, requestTime).Result()
	return
}

// StorePlacesForLocation is deprecated, but it is still a primitive implementation that might have faster search time compared
// with all places stored under one key
// store places obtained from database or external API in Redis
// places for a location are stored in separate sorted sets based on category
func (r *RedisClient) StorePlacesForLocation(context context.Context, geocodeInString string, places []POI.Place) error {
	client := r.client
	latLng, _ := utils.ParseLocation(geocodeInString)
	lat, lng := latLng[0], latLng[1]
	wg := &sync.WaitGroup{}
	wg.Add(len(places))
	for _, place := range places {
		sortedSetKey := strings.Join([]string{geocodeInString, string(POI.GetPlaceCategory(place.LocationType))}, "_")
		dist := utils.HaversineDist([]float64{lat, lng}, []float64{place.GetLocation().Latitude, place.GetLocation().Longitude})
		_, err := client.ZAdd(context, sortedSetKey, redis.Z{Score: dist, Member: place.ID}).Result()
		if err != nil {
			return err
		}
		r.setPlace(context, place, wg)
	}
	wg.Wait()
	return nil
}

// SetPlacesAddGeoLocations stores two types of information in redis
// 1. key-value pair, {placeID: POI.place}
// 2. add place to the correct bucket in geohashing for nearby search
func (r *RedisClient) SetPlacesAddGeoLocations(c context.Context, places []POI.Place) {
	wg := &sync.WaitGroup{}
	wg.Add(len(places))
	for _, place := range places {
		go func(place POI.Place) {
			defer wg.Done()
			_, err := r.Get().Pipelined(c, func(pipe redis.Pipeliner) error {
				placeCategory := POI.GetPlaceCategory(place.LocationType)
				geoLocation := &redis.GeoLocation{
					Name:      place.ID,
					Latitude:  place.GetLocation().Latitude,
					Longitude: place.GetLocation().Longitude,
				}

				redisKey := POI.EncodeNearbySearchRedisKey(placeCategory, place.PriceLevel)
				pipe.GeoAdd(c, redisKey, geoLocation)

				json_, err := json.Marshal(place)
				pipe.Set(c, "place_details:place_ID:"+place.ID, json_, 0)
				return err
			})
			if err != nil {
				Logger.Error(err)
			}
		}(place)
	}
	wg.Wait()
}

func updateCity(ctx context.Context, pipe redis.Pipeliner, city *City) error {
	key := strings.Join([]string{CityRedisKeyPrefix, city.ID}, ":")
	json_, err := json.Marshal(city)
	if err != nil {
		return err
	}

	if err = pipe.Set(ctx, key, json_, CityInfoExpirationTime).Err(); err != nil {
		return err
	}

	return nil
}

func (r *RedisClient) AddCities(ctx context.Context, cities []City) error {
	wg := sync.WaitGroup{}
	wg.Add(len(cities))
	var err error
	for _, city := range cities {
		go func(city City) {
			defer wg.Done()
			_, err = r.Get().Pipelined(ctx, func(pipe redis.Pipeliner) error {
				hashKey := strings.Join([]string{city.Name, city.AdminArea1, city.Country}, "_")
				var existedCityId string
				existedCityId, err = r.Get().HGet(ctx, KnownCitiesHashMapRedisKey, hashKey).Result()
				if err == nil && existedCityId != "" {
					city.ID = existedCityId
					return updateCity(ctx, pipe, &city)
				}

				err = pipe.HSet(ctx, KnownCitiesHashMapRedisKey, hashKey, city.ID).Err()
				if err != nil {
					Logger.Debugf("error adding city %s to %s: %v", city.Name, KnownCitiesHashMapRedisKey, err)
				}

				key := strings.Join([]string{CityRedisKeyPrefix, city.ID}, ":")
				var json_ []byte
				json_, err = json.Marshal(city)
				if err != nil {
					return err
				}
				if err = pipe.Set(ctx, key, json_, CityInfoExpirationTime).Err(); err != nil {
					return err
				}

				if err = pipe.GeoAdd(ctx, CitiesRedisKey, &redis.GeoLocation{
					Name:      city.ID,
					Longitude: city.Longitude,
					Latitude:  city.Latitude,
				}).Err(); err != nil {
					return err
				}
				Logger.Debugf("added city %s to Redis with key: %s", city.Name, key)
				return nil
			})
			if err != nil {
				Logger.Error(err)
			}
		}(city)
	}
	wg.Wait()
	return err
}

func (r *RedisClient) NearbyCities(ctx context.Context, lat, lng, radius float64) ([]City, error) {
	cities, err := r.Get().GeoRadius(ctx, CitiesRedisKey, lng, lat, &redis.GeoRadiusQuery{
		Radius: radius,
		Unit:   "km",
		Sort:   "ASC",
	}).Result()
	if err != nil {
		return nil, err
	}

	wg := sync.WaitGroup{}
	wg.Add(len(cities))
	nearbyCities := make([]City, 0)
	retrievedCities := make(chan City)

	for _, city := range cities {
		go func(city redis.GeoLocation) {
			defer wg.Done()

			cityKey := strings.Join([]string{CityRedisKeyPrefix, city.Name}, ":")
			cityString, cityQueryErr := r.Get().Get(ctx, cityKey).Result()
			if cityQueryErr != nil {
				Logger.Error(cityQueryErr)
				return
			}

			var c City
			unmarshallErr := json.Unmarshal([]byte(cityString), &c)
			if unmarshallErr != nil {
				Logger.Error(unmarshallErr)
				return
			}
			Logger.Debugf("retrieved city %s, %s from Redis", c.ID, c.Name)

			retrievedCities <- c
		}(city)
	}

	go func() {
		wg.Wait()
		close(retrievedCities)
	}()

	for c := range retrievedCities {
		nearbyCities = append(nearbyCities, c)
	}

	return nearbyCities, nil
}

// obtain place info from Redis based with key place_details:place_ID:placeID
func (r *RedisClient) getPlace(context context.Context, placeId string) (place POI.Place, err error) {
	res, err := r.client.Get(context, "place_details:place_ID:"+placeId).Result()
	utils.LogErrorWithLevel(err, utils.LogError)
	if err != nil {
		return
	}
	utils.LogErrorWithLevel(json.Unmarshal([]byte(res), &place), utils.LogError)
	return
}

func (r *RedisClient) NearbySearch(ctx context.Context, req *PlaceSearchRequest) ([]POI.Place, error) {
	redisKey := POI.EncodeNearbySearchRedisKey(req.PlaceCat, req.PriceLevel)
	requestLat, requestLng := req.Location.Latitude, req.Location.Longitude
	searchRadius := req.Radius

	if searchRadius > MaxSearchRadius {
		searchRadius = MaxSearchRadius
	}

	var cachedQualifiedPlaces []redis.GeoLocation
	for searchRadius <= MaxSearchRadius {
		Logger.Debugf("[request_id: %s] Redis geo radius is using search radius of %d meters", ctx.Value(ContextRequestIdKey), searchRadius)
		geoQuery := &redis.GeoRadiusQuery{
			Radius: float64(searchRadius),
			Unit:   "m",
			Sort:   "ASC", // sort ascending
		}

		var err error
		if cachedQualifiedPlaces, err = r.client.GeoRadius(ctx, redisKey, requestLng, requestLat, geoQuery).Result(); err != nil {
			return nil, err
		}
		if len(cachedQualifiedPlaces) >= int(req.MinNumResults) {
			break
		}
		searchRadius *= 2
	}

	req.Radius = searchRadius

	places := make([]POI.Place, 0)
	for _, placeInfo := range cachedQualifiedPlaces {
		if place, err := r.getPlace(ctx, placeInfo.Name); err == nil {
			places = append(places, place)
		}
	}

	if req.BusinessStatus == POI.Operational {
		totalPlacesCount := len(places)
		places = filter(places, func(place POI.Place) bool { return place.Status == POI.Operational })
		Logger.Debugf("(RedisClient) NearbySearch -> %d places out of %d left after business status filtering", len(places), totalPlacesCount)
	}
	return places, nil
}

// CacheLocationAlias caches the mapping from user input location name to geo-coding-corrected location name
// correct location name is an alias of itself
func (r *RedisClient) CacheLocationAlias(context context.Context, query GeocodeQuery, correctedQuery GeocodeQuery) (err error) {
	if strings.TrimSpace(query.City) != "" {
		_, err = r.client.HSet(context, "location_name_alias_mapping:city_names", strings.ToLower(query.City), strings.ToLower(correctedQuery.City)).Result()
		if err != nil {
			return
		}
	}

	if strings.TrimSpace(query.AdminAreaLevelOne) != "" {
		_, err = r.client.HSet(context, "location_name_alias_mapping:admin_area_level_one_names", strings.ToLower(query.AdminAreaLevelOne), strings.ToLower(correctedQuery.AdminAreaLevelOne)).Result()
		if err != nil {
			return
		}
	}

	if strings.TrimSpace(query.Country) != "" {
		_, err = r.client.HSet(context, "location_name_alias_mapping:country_names", strings.ToLower(query.Country), strings.ToLower(correctedQuery.Country)).Result()
		if err != nil {
			return
		}
	}

	return
}

func (r *RedisClient) GetLocationWithAlias(context context.Context, query *GeocodeQuery) (string, error) {
	Logger.Debugf("(RedisClient)GetLocationWithAlias -> request: %+v", *query)
	var err error
	var resCity, resAdminAreaLevelOne, resCountry string
	var locationSegments []string
	if strings.TrimSpace(query.City) != "" {
		resCity, err = r.client.HGet(context, "location_name_alias_mapping:city_names", strings.ToLower(query.City)).Result()
		if err != nil {
			return "", err
		}
		query.City = resCity
		locationSegments = append(locationSegments, resCity)
	}

	if strings.TrimSpace(query.AdminAreaLevelOne) != "" {
		resAdminAreaLevelOne, err = r.client.HGet(context, "location_name_alias_mapping:admin_area_level_one_names", strings.ToLower(query.AdminAreaLevelOne)).Result()
		if err != nil {
			return "", err
		}
		query.AdminAreaLevelOne = resAdminAreaLevelOne
		locationSegments = append(locationSegments, resAdminAreaLevelOne)
	}

	if strings.TrimSpace(query.Country) != "" {
		resCountry, err = r.client.HGet(context, "location_name_alias_mapping:country_names", strings.ToLower(query.Country)).Result()
		if err != nil {
			return "", err
		}
		query.Country = resCountry
		locationSegments = append(locationSegments, resCountry)
	}

	response := strings.Join(locationSegments, "_")
	Logger.Debugf("(RedisClient)GetLocationWithAlias -> response: %s", response)
	return response, nil
}

func (r *RedisClient) Geocode(context context.Context, query *GeocodeQuery) (lat float64, lng float64, err error) {
	redisKey := "geocode:cities"
	redisField, err := r.GetLocationWithAlias(context, query)
	if err != nil {
		return
	}

	var geocode string
	Logger.Debugf("(RedisClient)Geocode -> location in query is %+v", *query)
	geocode, err = r.client.HGet(context, redisKey, redisField).Result()
	if err != nil {
		return
	}
	var latLng [2]float64
	latLng, err = utils.ParseLocation(geocode)
	lat = latLng[0]
	lng = latLng[1]
	return
}

func (r *RedisClient) ReverseGeocode(context.Context, float64, float64) (*GeocodeQuery, error) {
	return nil, errors.New("->ReverseGeocode: not implemented for the RedisClient")
}

func (r *RedisClient) SetGeocode(context context.Context, query GeocodeQuery, lat float64, lng float64, originalQuery GeocodeQuery) {
	redisKey := "geocode:cities"
	redisHashField := strings.ToLower(strings.Join([]string{query.City, query.AdminAreaLevelOne, query.Country}, "_"))
	redisHashVal := strings.Join([]string{fmt.Sprintf("%.6f", lat), fmt.Sprintf("%.6f", lng)}, ",") // 1/9 meter precision
	_, err := r.client.HSet(context, redisKey, redisHashField, redisHashVal).Result()
	utils.LogErrorWithLevel(err, utils.LogError)
	if err != nil {
		Logger.Errorf("Failed to cache geolocation for location %s, %s with error %s", query.City, query.Country, err.Error())
		return
	} else {
		Logger.Debugf("Cached geolocation for location %s, %s success", query.City, query.Country)
	}
	utils.LogErrorWithLevel(r.CacheLocationAlias(context, originalQuery, query), utils.LogError)
}

// StreamsLogging returns redis streams ID if XADD command execution is successful
func (r *RedisClient) StreamsLogging(streamName string, data map[string]string) string {
	xArgs := redis.XAddArgs{Stream: streamName}
	keyValues := make([]string, 0)
	for key, val := range data {
		keyValues = append(keyValues, []string{key, val}...)
	}
	xArgs.Values = keyValues
	streamsId, err := r.client.XAdd(RedisClientDefaultBlankContext, &xArgs).Result()
	if err != nil {
		Logger.Error(err)
	}
	return streamsId
}

type PlanningSolutionRecord struct {
	ID              string              `json:"id"`
	PlaceIDs        []string            `json:"place_ids"`
	Score           float64             `json:"score"`
	ScoreOld        float64             `json:"score_old"`
	PlaceNames      []string            `json:"place_names"`
	PlaceLocations  [][2]float64        `json:"place_locations"`
	PlaceAddresses  []string            `json:"place_addresses"`
	PlaceURLs       []string            `json:"place_urls"`
	PlaceCategories []POI.PlaceCategory `json:"place_categories"`
	Destination     POI.Location        `json:"destination"`
}

type PlanningSolutionsResponse struct {
	PlanningSolutionRecords []PlanningSolutionRecord `json:"cached_planning_solutions"`
}

type PlanningSolutionsCacheRequest struct {
	Location        POI.Location
	Radius          uint64
	PriceLevel      POI.PriceLevel
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

func generateTravelPlansCacheKey(req *PlanningSolutionsCacheRequest) (string, error) {
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

	redisFieldKey := strings.ToLower(strings.Join([]string{TravelPlansRedisCacheKeyPrefix, country, region, city, radius, strconv.Itoa(int(req.Weekday)), strconv.Itoa(int(req.PriceLevel)), planIndexStr}, ":"))
	return redisFieldKey, nil
}

func (r *RedisClient) SavePlanningSolutions(context context.Context, request *PlanningSolutionsCacheRequest, response *PlanningSolutionsResponse) error {
	// solutions with no valid solutions do not worth saving
	if len(response.PlanningSolutionRecords) == 0 {
		return nil
	}
	redisListKey, keyGenerationErr := generateTravelPlansCacheKey(request)
	if keyGenerationErr != nil {
		Logger.Errorf("failed to generate travel plans cache key, error %s", keyGenerationErr.Error())
		return keyGenerationErr
	}

	var recordKeys []string
	for _, record := range response.PlanningSolutionRecords {
		solutionRedisKey := strings.Join([]string{TravelPlanRedisCacheKeyPrefix, record.ID}, ":")
		json_, err := json.Marshal(record)
		if err != nil {
			return err
		}
		_, recordSaveErr := r.client.Set(context, solutionRedisKey, json_, 0).Result()
		if recordSaveErr != nil {
			return recordSaveErr
		}
		recordKeys = append(recordKeys, solutionRedisKey)
	}

	if len(recordKeys) > 0 {
		numTravelPlanKeys, listSaveErr := r.client.LPush(context, redisListKey, recordKeys).Result()
		Logger.Debugf("added the %d travel plan keys to %s", numTravelPlanKeys, redisListKey)
		r.client.Expire(context, redisListKey, PlanningSolutionsExpirationTime)

		return listSaveErr
	}

	return nil
}

func (r *RedisClient) PlanningSolutions(context context.Context, request *PlanningSolutionsCacheRequest) (PlanningSolutionsResponse, error) {
	Logger.Debugf("->RedisClient.PlanningSolutions(%v)", request)
	var response PlanningSolutionsResponse
	redisListKey, keyGenerationErr := generateTravelPlansCacheKey(request)
	if keyGenerationErr != nil {
		Logger.Error(keyGenerationErr)
		return response, keyGenerationErr
	}

	exists, _ := r.client.Exists(context, redisListKey).Result()
	if exists == 0 {
		return response, fmt.Errorf("redis key %s does not exist", redisListKey)
	}

	recordKeys, listFetchErr := r.client.LRange(context, redisListKey, 0, -1).Result()
	if listFetchErr != nil {
		Logger.Error(listFetchErr)
		return response, listFetchErr
	}

	response.PlanningSolutionRecords = make([]PlanningSolutionRecord, len(recordKeys))
	for idx, key := range recordKeys {
		json_, err := r.client.Get(context, key).Result()
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

func (r *RedisClient) FetchSingleRecord(context context.Context, redisKey string, response interface{}) error {
	json_, err := r.client.Get(context, redisKey).Result()
	if err != nil {
		Logger.Debugf("[request_id: %s] redis server find no result for key: %s", context.Value(ContextRequestIdKey), redisKey)
		return err
	}
	err = json.Unmarshal([]byte(json_), response)
	if err != nil {
		Logger.Error(err)
		return err
	}
	return nil
}
