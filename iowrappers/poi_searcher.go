package iowrappers

import (
	"context"
	"net/url"
	"time"

	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/utils"
	"go.uber.org/zap"
)

type ContextKey string

const (
	MaxSearchRadius              = 16000               // 10 miles
	MinMapsResultRefreshDuration = time.Hour * 24 * 14 // 14 days
	GoogleSearchHomePageURL      = "https://www.google.com/"
	ContextRequestIdKey          = ContextKey("request_id")
)

type PoiSearcher struct {
	mapsClient  MapsClient
	redisClient RedisClient
}

// GeocodeQuery can also be used as the result of reverse geocoding
type GeocodeQuery struct {
	City              string `json:"city"`
	AdminAreaLevelOne string `json:"admin_area_level_one"`
	Country           string `json:"country"`
}

var Logger *zap.SugaredLogger

func CreatePoiSearcher(mapsApiKey string, redisUrl *url.URL) *PoiSearcher {
	poiSearcher := PoiSearcher{
		mapsClient:  CreateMapsClient(mapsApiKey),
		redisClient: CreateRedisClient(redisUrl),
	}
	return &poiSearcher
}

func (poiSearcher PoiSearcher) GetMapsClient() *MapsClient {
	return &poiSearcher.mapsClient
}

func (poiSearcher PoiSearcher) GetRedisClient() *RedisClient {
	return &poiSearcher.redisClient
}

func DestroyLogger() {
	_ = Logger.Sync()
}

// Geocode performs geocoding, mapping city and country to latitude and longitude
func (poiSearcher PoiSearcher) Geocode(context context.Context, query *GeocodeQuery) (lat float64, lng float64, err error) {
	originalGeocodeQuery := GeocodeQuery{}
	originalGeocodeQuery.City = query.City
	originalGeocodeQuery.Country = query.Country
	originalGeocodeQuery.AdminAreaLevelOne = query.AdminAreaLevelOne
	var geocodeMissingErr error
	lat, lng, geocodeMissingErr = poiSearcher.redisClient.Geocode(context, query)
	if geocodeMissingErr != nil {
		lat, lng, err = poiSearcher.mapsClient.Geocode(context, query)
		if err != nil {
			return
		}
		// either redisClient or mapsClient may have corrected location fields in the query
		poiSearcher.redisClient.SetGeocode(context, *query, lat, lng, originalGeocodeQuery)
		Logger.Debugf("Geolocation (lat,lng) Cache miss for location %s, %s is %.4f, %.4f",
			query.City, query.Country, lat, lng)
	}
	return
}

func (poiSearcher PoiSearcher) ReverseGeocode(ctx context.Context, lat, lng float64) (*GeocodeQuery, error) {
	Logger.Debugf("PoiSearcher ->ReverseGeocode: decoding latitude %.2f, longitude %.2f", lat, lng)
	return poiSearcher.mapsClient.ReverseGeocode(ctx, lat, lng)
}

func (poiSearcher PoiSearcher) NearbySearch(context context.Context, request *PlaceSearchRequest) ([]POI.Place, error) {
	if err := poiSearcher.processLocation(context, request); err != nil {
		return nil, err
	}
	location := request.Location

	var cachedPlaces, places []POI.Place
	var err error
	cachedPlaces, err = poiSearcher.redisClient.NearbySearch(context, request)
	if err != nil {
		Logger.Error(err)
	}

	Logger.Debugf("[request_id: %s] number of results from redis is %d", context.Value(ContextRequestIdKey), len(cachedPlaces))

	// update last search time for the city
	lastSearchTime, cacheMiss := poiSearcher.redisClient.GetMapsLastSearchTime(context, location, request.PlaceCat)

	currentTime := time.Now()
	// use place data from database if the location is known and the data is fresh, and we have sufficient data
	if cacheMiss == nil && (currentTime.Sub(lastSearchTime) <= MinMapsResultRefreshDuration && uint(len(cachedPlaces)) >= request.MinNumResults) {
		Logger.Infof("[request_id: %s] Using Redis to fulfill request. Place Type: %s", context.Value(ContextRequestIdKey), request.PlaceCat)
		places = append(places, cachedPlaces...)
		return places, nil
	}

	utils.LogErrorWithLevel(poiSearcher.redisClient.SetMapsLastSearchTime(context, location, request.PlaceCat, currentTime.Format(time.RFC3339)), utils.LogError)

	// initiate a new external search
	newPlaces, searchErr := poiSearcher.searchPlacesWithMaps(context, request)
	if searchErr != nil {
		return nil, searchErr
	}

	// safeguard on accessing elements in a nil slice
	if len(newPlaces) > 0 {
		// update Redis with all the new places obtained
		poiSearcher.UpdateRedis(context, newPlaces)

		// include places from cache in the result
		places = append(places, newPlaces...)
	}

	return places, nil
}

//processLocation performs reverse geocoding for precise location to find city-level information and performs geocoding to find precise latitude and longitude values
func (poiSearcher PoiSearcher) processLocation(ctx context.Context, req *PlaceSearchRequest) error {
	location := &req.Location
	if req.UsePreciseLocation {
		Logger.Debugf("->NearbySearch: using precise location")
		geoQuery, err := poiSearcher.GetMapsClient().ReverseGeocode(ctx, req.Location.Latitude, req.Location.Longitude)
		if err != nil {
			return err
		}
		location.City = geoQuery.City
		location.AdminAreaLevelOne = geoQuery.AdminAreaLevelOne
		location.Country = geoQuery.Country
		return nil
	}

	lat, lng, err := poiSearcher.Geocode(ctx, &GeocodeQuery{
		City:              location.City,
		AdminAreaLevelOne: location.AdminAreaLevelOne,
		Country:           location.Country,
	})
	if err != nil {
		return err
	}
	location.Latitude = lat
	location.Longitude = lng
	return nil
}

func (poiSearcher PoiSearcher) searchPlacesWithMaps(ctx context.Context, req *PlaceSearchRequest) ([]POI.Place, error) {
	originalRadius := req.Radius

	// use a large search radius whenever we call external maps services
	req.Radius = MaxSearchRadius

	places, err := poiSearcher.GetMapsClient().NearbySearch(ctx, req)

	// restore search radius upon search completion
	req.Radius = originalRadius
	if err != nil {
		return nil, err
	}

	if req.BusinessStatus == POI.Operational {
		totalPlacesCount := len(places)
		places = filter(places, func(place POI.Place) bool { return place.Status == POI.Operational })
		Logger.Debugf("%d places out of %d left after business status filtering", len(places), totalPlacesCount)
	}

	if uint(len(places)) < req.MinNumResults {
		Logger.Debugf("Found %d POI results for place type %s, less than requested number of %d",
			len(places), req.PlaceCat, req.MinNumResults)
	}
	if len(places) == 0 {
		Logger.Debugf("No qualified POI result found in the given location %v, radius %d, and place type: %s",
			req.Location, req.Radius, req.PlaceCat)
		Logger.Debug("location may be invalid")
	}
	return places, nil
}

func (poiSearcher PoiSearcher) UpdateRedis(context context.Context, places []POI.Place) {
	poiSearcher.redisClient.SetPlacesOnCategory(context, places)
	requestId := context.Value(ContextRequestIdKey)
	Logger.Debugf("[request_id: %s]Redis update complete", requestId)
}
