package iowrappers

import (
	"context"
	"fmt"
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/utils"
	"go.uber.org/zap"
	"googlemaps.github.io/maps"
	"net/url"
	"strings"
	"time"
)

const (
	MaxSearchRadius              = 16000          // 10 miles
	MinMapsResultRefreshDuration = time.Hour * 24 // 1 day
	GoogleSearchHomePageURL      = "https://www.google.com/"
)

type PoiSearcher struct {
	mapsClient  MapsClient
	redisClient RedisClient
}

type GeocodeQuery struct {
	City    string
	Country string
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

func DestroyLogger() {
	_ = Logger.Sync()
}

// currently geocode is equivalent to mapping city and country to latitude and longitude
func (poiSearcher PoiSearcher) GetGeocode(query *GeocodeQuery) (lat float64, lng float64, err error) {
	originalGeocodeQuery := GeocodeQuery{}
	originalGeocodeQuery.City = query.City
	originalGeocodeQuery.Country = query.Country
	lat, lng, geocodeMissingErr := poiSearcher.redisClient.GetGeocode(query)
	if geocodeMissingErr != nil {
		lat, lng, err = poiSearcher.mapsClient.GetGeocode(query)
		if err != nil {
			return
		}
		// either redisClient or mapsClient may have corrected location name in the query
		poiSearcher.redisClient.SetGeocode(*query, lat, lng, originalGeocodeQuery)
		Logger.Debugf("Geolocation (lat,lng) Cache miss for location %s, %s is %.4f, %.4f",
			query.City, query.Country, lat, lng)
	}
	return
}

func (poiSearcher PoiSearcher) NearbySearch(context context.Context, request *PlaceSearchRequest) (places []POI.Place, err error) {
	location := request.Location
	cityCountry := strings.Split(location, ",")
	lat, lng, err := poiSearcher.GetGeocode(&GeocodeQuery{
		City:    cityCountry[0],
		Country: cityCountry[1],
	})
	if logErr(err, utils.LogError) {
		return
	}

	places = make([]POI.Place, 0)
	// request.Location is overwritten to lat,lng
	request.Location = fmt.Sprint(lat) + "," + fmt.Sprint(lng)

	var cachedPlaces []POI.Place
	cachedPlaces, err = poiSearcher.redisClient.NearbySearch(context, request)
	if err != nil {
		Logger.Error(err)
	}

	Logger.Debugf("number of results from redis is %d", len(cachedPlaces))

	lastSearchTime, cacheErr := poiSearcher.redisClient.GetMapsLastSearchTime(location, request.PlaceCat)

	currentTime := time.Now()
	if uint(len(cachedPlaces)) >= request.MinNumResults || currentTime.Sub(lastSearchTime) <= MinMapsResultRefreshDuration {
		Logger.Infof("Using Redis to fulfill request. Place Type: %s", request.PlaceCat)
		maxResultNum := utils.MinInt(len(cachedPlaces), int(request.MaxNumResults))
		places = append(places, cachedPlaces[:maxResultNum]...)
		return
	}

	cacheErr = poiSearcher.redisClient.SetMapsLastSearchTime(location, request.PlaceCat, currentTime.Format(time.RFC3339))
	utils.CheckErrImmediate(cacheErr, utils.LogError)

	maxResultNum := utils.MinInt(len(cachedPlaces), int(request.MaxNumResults))

	originalSearchRadius := request.Radius

	request.Radius = MaxSearchRadius // use a large search radius whenever we call external maps services

	// initiate a new external search
	newPlaces, mapsNearbySearchErr := poiSearcher.mapsClient.NearbySearch(context, request)
	utils.CheckErrImmediate(mapsNearbySearchErr, utils.LogError)

	request.Radius = originalSearchRadius // restore search radius

	maxResultNum = utils.MinInt(len(newPlaces), int(request.MaxNumResults))

	// update Redis with all the new places obtained
	poiSearcher.UpdateRedis(newPlaces)

	// safe-guard on accessing elements in a nil slice
	if len(newPlaces) > 0 {
		places = append(places, newPlaces[:maxResultNum]...)
	}

	if uint(len(places)) < request.MinNumResults {
		Logger.Debugf("Found %d POI results for place type %s, less than requested number of %d",
			len(places), request.PlaceCat, request.MinNumResults)
	}
	if len(places) == 0 {
		Logger.Debugf("No qualified POI result found in the given location %s, radius %d, and place type: %s",
			request.Location, request.Radius, request.PlaceCat)
		Logger.Debug("location may be invalid")
	}
	return
}

func (poiSearcher PoiSearcher) PlaceDetailsSearch(placeId string) (place POI.Place, err error) {
	var res maps.PlaceDetailsResult
	res, err = PlaceDetailedSearch(&poiSearcher.mapsClient, placeId)
	if err != nil {
		Logger.Errorf("PlaceDetailedSearch(PoiSearcher) error %s", err.Error())
		return
	}
	// for now only updates the URL field
	place.SetURL(res.URL)
	Logger.Debugf("Place %s url set to %s", place.Name, res.URL)
	return
}

func (poiSearcher PoiSearcher) UpdateRedis(places []POI.Place) {
	poiSearcher.redisClient.SetPlacesOnCategory(places)
	Logger.Debugf("Redis update complete")
}
