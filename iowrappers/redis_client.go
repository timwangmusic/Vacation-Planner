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

	"github.com/bobg/go-generics/slices"
	gogeonames "github.com/timwangmusic/go-geonames"

	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/utils"
)

const (
	PlanningSolutionsExpirationTime = 24 * time.Hour
	CityInfoExpirationTime          = 0

	NumVisitorsPlanningAPI = "visitor_count:planning_api"

	TravelPlansRedisCacheKeyPrefix = "travel_plans"
	TravelPlanRedisCacheKeyPrefix  = "travel_plan"
	CityRedisKeyPrefix             = "city"
	CitiesRedisKey                 = "known_cities_ids"
	KnownCitiesHashMapRedisKey     = "known_cities_name_to_id"
	MapsLastSearchTimeRedisKey     = "MapsLastSearchTime"
	// AnnouncementsRedisKey is a Redis Hash that maps ID to announcement details
	AnnouncementsRedisKey      = "announcements"
	PlaceDetailsRedisKeyPrefix = "place_details:place_ID:"
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

// CollectPlanningAPIStats updates the number of calls to the planning APIs per hour.
// It also updates API call stats for each city per hour.
// These keys expire after 30 days.
func (r *RedisClient) CollectPlanningAPIStats(event PlanningEvent, workerIdx int) {
	c := r.client

	pipeline := c.Pipeline()

	bucketIdx, err := hourBucketIndex(event.Timestamp)
	if err != nil {
		Logger.Debugf("failed to collect API stat %+v", event)
		return
	}

	ctx := context.Background()
	totalVisitorsKey := strings.Join([]string{NumVisitorsPlanningAPI, bucketIdx}, ":")
	if exists, err := r.client.Exists(ctx, totalVisitorsKey).Result(); err != nil {
		Logger.Errorf("failed to check Redis %s for key %s existence", err.Error(), totalVisitorsKey)
		return
	} else if exists == 0 {
		pipeline.Set(ctx, totalVisitorsKey, 0, time.Hour*24*30)
	}
	pipeline.Incr(ctx, totalVisitorsKey)

	location := strings.ReplaceAll(strings.Join([]string{event.City, event.Country}, ":"), " ", "_")
	if event.AdminAreaLevelOne != "" {
		location = strings.ReplaceAll(strings.Join([]string{event.City, event.AdminAreaLevelOne, event.Country}, ":"), " ", "_")
	}
	location = strings.ToLower(location)

	redisKey := strings.Join([]string{NumVisitorsPlanningAPI, location, bucketIdx}, ":")
	if exists, err := r.client.Exists(ctx, redisKey).Result(); err != nil {
		Logger.Errorf("failed to check Redis %s for key %s existence", err.Error(), redisKey)
		return
	} else if exists == 0 {
		pipeline.Set(ctx, redisKey, 0, time.Hour*24*30)
	}
	pipeline.Incr(ctx, redisKey)

	if _, err = pipeline.Exec(ctx); err != nil {
		Logger.Debugf("failed to collect API stats %+v: %s", event, err.Error())
	}
	Logger.Debugf("API event worker %d successfully handled job", workerIdx)
}

// hour bucket in UTC standard time
func hourBucketIndex(timestamp string) (string, error) {
	t, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return "", err
	}

	t = t.UTC()

	// Convert to YYYYMMDD:HH format
	return t.Format("20060102:15"), nil
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

	_, err = r.client.Set(context, PlaceDetailsRedisKeyPrefix+place.ID, json_, 0).Result()
	if err != nil {
		Logger.Error(err)
	}
}

func (r *RedisClient) GetMapsLastSearchTime(context context.Context, location POI.Location, category POI.PlaceCategory, priceLevel POI.PriceLevel) (lastSearchTime time.Time, err error) {
	redisField := strings.ToLower(strings.Join([]string{location.Country, location.AdminAreaLevelOne, location.City, string(category), strconv.Itoa(int(priceLevel))}, ":"))
	// for places in Visit category Google Maps do not provide pricing info, this is subject to change in the future
	if category == POI.PlaceCategoryVisit {
		redisField = strings.ToLower(strings.Join([]string{location.Country, location.AdminAreaLevelOne, location.City, string(category)}, ":"))
	}
	lst, cacheErr := r.client.HGet(context, MapsLastSearchTimeRedisKey, redisField).Result()
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

func (r *RedisClient) SetMapsLastSearchTime(context context.Context, location POI.Location, category POI.PlaceCategory, priceLevel POI.PriceLevel, requestTime string) (err error) {
	redisField := strings.ToLower(strings.Join([]string{location.Country, location.AdminAreaLevelOne, location.City, string(category), strconv.Itoa(int(priceLevel))}, ":"))
	// for places in Visit category Google Maps do not provide pricing info, this is subject to change in the future
	if category == POI.PlaceCategoryVisit {
		redisField = strings.ToLower(strings.Join([]string{location.Country, location.AdminAreaLevelOne, location.City, string(category)}, ":"))
	}
	_, err = r.client.HSet(context, MapsLastSearchTimeRedisKey, redisField, requestTime).Result()
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
				pipe.Set(c, PlaceDetailsRedisKeyPrefix+place.ID, json_, 0)
				return err
			})
			if err != nil {
				Logger.Error(err)
			}
		}(place)
	}
	wg.Wait()
}

func (r *RedisClient) UpdatePlace(ctx context.Context, id string, data map[string]interface{}) error {
	var p POI.Place
	err := r.FetchSingleRecord(ctx, PlaceDetailsRedisKeyPrefix+id, &p)
	if err != nil {
		return err
	}

	for field, val := range data {
		switch field {
		case "photo":
			p.Photo = val.(POI.PlacePhoto)
		default:
			return errors.New("field not known")
		}
	}

	wg := &sync.WaitGroup{}
	wg.Add(1)
	r.setPlace(ctx, p, wg)
	wg.Wait()
	return nil
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
	var err error
	for _, city := range cities {
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
	}
	return err
}

func (r *RedisClient) NearbyCities(ctx context.Context, lat, lng, radius float64, filter gogeonames.SearchFilter) ([]City, error) {
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

	populationThreshold := searchFilterToPopulation(filter)
	nearbyCities, _ = slices.Filter(nearbyCities, func(city City) (bool, error) { return city.Population >= populationThreshold, nil })
	return nearbyCities, nil
}

// obtain place info from Redis based with key place_details:place_ID:placeID
func (r *RedisClient) getPlace(context context.Context, placeId string) (place POI.Place, err error) {
	res, err := r.client.Get(context, PlaceDetailsRedisKeyPrefix+placeId).Result()
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
		places = Filter(places, func(place POI.Place) bool { return place.Status == POI.Operational })
		Logger.Debugf("(RedisClient)NearbySearch -> %d places out of %d left after business status filtering", len(places), totalPlacesCount)
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
	Weekdays        []string            `json:"weekdays"`
	TimeSlots       []string            `json:"time_slots"`
	Destination     POI.Location        `json:"destination"`
	PlanSpec        string              `json:"plan_spec"`
}

type PlanningSolutionsResponse struct {
	PlanningSpec            string                   `json:"planning_spec"`
	PlanningSolutionRecords []PlanningSolutionRecord `json:"cached_planning_solutions"`
}

type PlanningSolutionsSaveRequest struct {
	Location                POI.Location
	PriceLevel              POI.PriceLevel
	PlaceCategories         []POI.PlaceCategory
	Intervals               []POI.TimeInterval
	Weekdays                []POI.Weekday
	PlanningSolutionRecords []PlanningSolutionRecord
	NumPlans                int64
}

func timeSlotsIndex(placeCategories []POI.PlaceCategory, intervals []POI.TimeInterval, weekdays []POI.Weekday) (string, error) {
	if len(placeCategories) != len(intervals) {
		return "", fmt.Errorf("the number of place categories %d does not match the number of intervals %d", len(placeCategories), len(intervals))
	}

	if len(placeCategories) != len(weekdays) {
		return "", fmt.Errorf("the number of place categories %d does not match the number of weekdays %d", len(placeCategories), len(weekdays))
	}

	parts := make([]string, 0)
	for idx := range placeCategories {
		timeSlotIdx, err := singleTimeSlotIndex(placeCategories[idx], intervals[idx], weekdays[idx])
		if err != nil {
			return "", err
		}
		parts = append(parts, timeSlotIdx)
	}

	return strings.Join(parts, "_"), nil
}

func singleTimeSlotIndex(category POI.PlaceCategory, interval POI.TimeInterval, weekday POI.Weekday) (string, error) {
	parts := make([]string, 0)
	switch category {
	case POI.PlaceCategoryVisit:
		parts = append(parts, "V")
	case POI.PlaceCategoryEatery:
		parts = append(parts, "E")
	default:
		return "", fmt.Errorf("unknown place category %s", category)
	}

	if int(interval.Start) >= 24 || int(interval.Start) < 0 {
		return "", fmt.Errorf("interval start time should be between 0 and 23 inclusive, got %s", interval.Start.ToString())
	}

	if int(interval.End) >= 24 || int(interval.End) < 0 {
		return "", fmt.Errorf("interval start time should be between 0 and 23 inclusive, got %s", interval.End.ToString())
	}

	parts = append(parts, interval.Start.ToString())
	parts = append(parts, interval.End.ToString())

	if int(weekday) > 6 || int(weekday) < 0 {
		return "", fmt.Errorf("weekday falls between 0 and 6 inclusive, got %s", weekday.String())
	}

	parts = append(parts, weekday.String())
	return strings.Join(parts, "-"), nil
}

func TravelPlansCacheKey(req *PlanningSolutionsSaveRequest) (string, error) {
	country, region, city := req.Location.Country, req.Location.AdminAreaLevelOne, req.Location.City
	slotsIndex, err := timeSlotsIndex(req.PlaceCategories, req.Intervals, req.Weekdays)
	if err != nil {
		return "", err
	}

	country = strings.ReplaceAll(strings.ToLower(country), " ", "_")
	region = strings.ReplaceAll(strings.ToLower(region), " ", "_")
	city = strings.ReplaceAll(strings.ToLower(city), " ", "_")

	redisFieldKey := strings.ToLower(strings.Join([]string{TravelPlansRedisCacheKeyPrefix, country, region, city, strconv.Itoa(int(req.PriceLevel)), slotsIndex}, ":"))
	return redisFieldKey, nil
}

func (r *RedisClient) SavePlanningSolutions(ctx context.Context, request *PlanningSolutionsSaveRequest) error {
	// solutions with no valid solutions do not worth saving
	if len(request.PlanningSolutionRecords) == 0 {
		return nil
	}
	sortedSetKey, keyGenerationErr := TravelPlansCacheKey(request)
	if keyGenerationErr != nil {
		Logger.Errorf("failed to generate travel plans cache key, error %s", keyGenerationErr.Error())
		return keyGenerationErr
	}

	// cleans up previous results
	exists, _ := r.client.Exists(ctx, sortedSetKey).Result()
	if exists == 1 {
		if err := r.client.Del(ctx, sortedSetKey).Err(); err != nil {
			Logger.Error(err)
		}
	}

	numRecords := len(request.PlanningSolutionRecords)
	recordKeys := make([]string, numRecords)
	scores := make([]float64, numRecords)
	wg := &sync.WaitGroup{}
	wg.Add(numRecords)

	errChan := make(chan error)
	go func() {
		for err := range errChan {
			Logger.Error(err)
		}
	}()

	for i, record := range request.PlanningSolutionRecords {
		go func(idx int, solutionRecord PlanningSolutionRecord) {
			defer wg.Done()
			solutionRedisKey := strings.Join([]string{TravelPlanRedisCacheKeyPrefix, solutionRecord.ID}, ":")
			json_, err := json.Marshal(solutionRecord)
			if err != nil {
				errChan <- err
				return
			}
			_, recordSaveErr := r.client.Set(ctx, solutionRedisKey, json_, PlanningSolutionsExpirationTime).Result()
			if recordSaveErr != nil {
				errChan <- recordSaveErr
				return
			}
			recordKeys[idx] = solutionRedisKey
			scores[idx] = solutionRecord.Score
		}(i, record)
	}

	wg.Wait()
	close(errChan)

	var members = make([]redis.Z, 0)
	for idx, key := range recordKeys {
		members = append(members, redis.Z{
			Score:  scores[idx],
			Member: key,
		})
	}

	if len(recordKeys) > 0 {
		_, err := r.Get().ZAdd(ctx, sortedSetKey, members...).Result()
		if err != nil {
			return err
		}

		r.Get().Expire(ctx, sortedSetKey, PlanningSolutionsExpirationTime)
		Logger.Debugf("added the %d travel plan keys to %s", len(members), sortedSetKey)

		return err
	}

	return nil
}

func (r *RedisClient) PlanningSolutions(ctx context.Context, request *PlanningSolutionsSaveRequest) (*PlanningSolutionsResponse, error) {
	Logger.Debugf("->RedisClient.PlanningSolutions(%v)", request)
	var response = &PlanningSolutionsResponse{}
	sortedSetKey, keyGenerationErr := TravelPlansCacheKey(request)
	if keyGenerationErr != nil {
		Logger.Error(keyGenerationErr)
		return response, keyGenerationErr
	}

	response.PlanningSpec = sortedSetKey

	exists, err := r.Get().Exists(ctx, sortedSetKey).Result()
	if err != nil {
		return response, err
	}
	if exists == 0 {
		return response, fmt.Errorf("redis key %s does not exist", sortedSetKey)
	}

	ttl := r.Get().TTL(ctx, sortedSetKey).Val()

	userId, ok := ctx.Value(ContextRequestUserId).(string)
	if !ok {
		userId = "guest"
	}
	userPlansSSKey := strings.Join([]string{"user", userId, sortedSetKey}, ":")

	exists, err = r.Get().Exists(ctx, userPlansSSKey).Result()
	if err != nil {
		Logger.Error(err)
	}
	if exists == 0 {
		if err = r.Get().Copy(ctx, sortedSetKey, userPlansSSKey, 0, false).Err(); err != nil {
			return response, err
		}
		// TTL for user plans set should follow the expiration of the master set
		r.Get().Expire(ctx, userPlansSSKey, ttl)
	}

	recordKeys, ssFetchErr := r.Get().ZRevRange(ctx, userPlansSSKey, 0, request.NumPlans-1).Result()
	if ssFetchErr != nil {
		return response, ssFetchErr
	}

	response.PlanningSolutionRecords = make([]PlanningSolutionRecord, 0)
	for idx, key := range recordKeys {
		if int64(idx) >= request.NumPlans {
			break
		}

		var json_ string
		json_, err = r.Get().Get(ctx, key).Result()
		if err != nil {
			Logger.Error(err)
			continue
		}

		var r PlanningSolutionRecord
		err = json.Unmarshal([]byte(json_), &r)
		if err != nil {
			Logger.Error(err)
			continue
		}
		response.PlanningSolutionRecords = append(response.PlanningSolutionRecords, r)
	}

	return response, nil
}

func (r *RedisClient) SaveAnnouncement(ctx context.Context, id, data string) error {
	return r.Get().HSet(ctx, AnnouncementsRedisKey, id, data).Err()
}

func (r *RedisClient) FetchSingleRecord(context context.Context, redisKey string, response interface{}) error {
	json_, err := r.client.Get(context, redisKey).Result()
	if err != nil {
		return fmt.Errorf("[request_id: %s] redis server find no result for key: %s", context.Value(ContextRequestIdKey), redisKey)
	}
	err = json.Unmarshal([]byte(json_), response)
	if err != nil {
		return err
	}
	return nil
}

func (r *RedisClient) FetchSingleRecordTypeSet(context context.Context, redisKey string) ([]string, error) {
	members, err := r.client.SMembers(context, redisKey).Result()
	if err != nil {
		return members, fmt.Errorf("[request_id: %s] redis server find no result for key: %s", context.Value(ContextRequestIdKey), redisKey)
	}
	return members, nil
}
