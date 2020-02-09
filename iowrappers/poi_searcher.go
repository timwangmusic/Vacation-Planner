package iowrappers

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/utils"
	"go.uber.org/zap"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	MaxSearchRadius              = 16000          // 10 miles
	MinMapsResultRefreshDuration = time.Hour * 24 // 1 day
)

type PlaceSearcher interface {
	NearbySearch(request *PlaceSearchRequest) ([]POI.Place, error)
}

type PoiSearcher struct {
	mapsClient  *MapsClient
	dbHandler   *DbHandler
	redisClient *RedisClient
}

type GeocodeQuery struct {
	City    string
	Country string
}

var Logger *zap.SugaredLogger

func (poiSearcher *PoiSearcher) Init(mapsApiKey string, dbUrl string, redisUrl *url.URL, dbName string) {
	mapsClient := &MapsClient{}
	utils.CheckErrImmediate(mapsClient.Init(mapsApiKey), utils.LogFatal)

	poiSearcher.mapsClient = mapsClient

	poiSearcher.dbHandler = &DbHandler{}
	// delegate error check of db handler to dbHandler.Init
	poiSearcher.dbHandler.Init(dbName, dbUrl)

	poiSearcher.redisClient = &RedisClient{}
	poiSearcher.redisClient.Init(redisUrl)
}

func DestroyLogger() {
	_ = Logger.Sync()
}

// currently geocode is equivalent to mapping city and country to latitude and longitude
func (poiSearcher *PoiSearcher) Geocode(query *GeocodeQuery) (lat float64, lng float64, err error) {
	originalGeocodeQuery := GeocodeQuery{}
	originalGeocodeQuery.City = query.City
	originalGeocodeQuery.Country = query.Country
	lat, lng, exist := poiSearcher.redisClient.GetGeocode(query)
	if !exist {
		lat, lng, err = poiSearcher.mapsClient.Geocode(query)
		if err != nil {
			return
		}
		// either redisClient or mapsClient may have corrected location name in the query
		poiSearcher.redisClient.SetGeocode(*query, lat, lng, originalGeocodeQuery)
		log.Debugf("Geolocation (lat,lng) Cache miss for location %s, %s is %.4f, %.4f",
			query.City, query.Country, lat, lng)
	}
	return
}

// if client API key is invalid but not empty string, nearby search result will be empty
func (poiSearcher *PoiSearcher) NearbySearch(request *PlaceSearchRequest) (places []POI.Place, err error) {
	dbHandler := poiSearcher.dbHandler

	location := request.Location
	cityCountry := strings.Split(location, ",")
	lat, lng, err := poiSearcher.Geocode(&GeocodeQuery{
		City:    cityCountry[0],
		Country: cityCountry[1],
	})
	if logErr(err, utils.LogError) {
		return
	}

	places = make([]POI.Place, 0)
	// request.Location is overwritten to lat/lng
	request.Location = fmt.Sprint(lat) + "," + fmt.Sprint(lng)

	//cachedPlaces := poiSearcher.redisClient.NearbySearch(request)
	cachedPlaces := poiSearcher.redisClient.GetPlaces(request)
	log.Debugf("number of results from redis is %d", len(cachedPlaces))
	if uint(len(cachedPlaces)) >= request.MinNumResults {
		Logger.Infof("Using Redis to fulfill request. Place Type: %s", request.PlaceCat)
		maxResultNum := utils.MinInt(len(cachedPlaces), int(request.MaxNumResults))
		places = append(places, cachedPlaces[:maxResultNum]...)
		return
	} else {
		dbStoredPlaces, dbSearchErr := dbHandler.PlaceSearch(request)
		utils.CheckErrImmediate(dbSearchErr, utils.LogError)

		maxResultNum := utils.MinInt(len(dbStoredPlaces), int(request.MaxNumResults))
		if uint(len(dbStoredPlaces)) < request.MinNumResults {
			lastSearchTime, cacheErr := poiSearcher.redisClient.GetMapsLastSearchTime(location, request.PlaceCat)
			currentTime := time.Now()

			// balances trade-off between data staleness and the number of Maps API calls
			if cacheErr != nil || currentTime.Sub(lastSearchTime) > MinMapsResultRefreshDuration {
				originalSearchRadius := request.Radius
				request.Radius = MaxSearchRadius // use a large search radius whenever we call external maps service
				newPlaces, mapsNearbySearchErr := poiSearcher.mapsClient.NearbySearch(request)
				utils.CheckErrImmediate(mapsNearbySearchErr, utils.LogError)

				// record search start time
				_ = poiSearcher.redisClient.SetMapsLastSearchTime(location, request.PlaceCat, currentTime.Format(time.RFC3339))

				maxResultNum = utils.MinInt(len(newPlaces), int(request.MaxNumResults))
				places = append(places, newPlaces[:maxResultNum]...)

				// update database
				poiSearcher.UpdateMongo(request.PlaceCat, newPlaces)

				request.Radius = originalSearchRadius // restore search radius

				// refresh results from the database
				dbStoredPlaces, dbSearchErr = dbHandler.PlaceSearch(request)
				utils.CheckErrImmediate(dbSearchErr, utils.LogError)
				maxResultNum = utils.MinInt(len(dbStoredPlaces), int(request.MaxNumResults))
			}
		}
		if len(dbStoredPlaces) > 0 {
			places = append(places, dbStoredPlaces[:maxResultNum]...)
		}
		Logger.Debugf("Using MongoDB to fulfill request. Returning %d places of type: %s", len(places), request.PlaceCat)
	}
	// update cache
	poiSearcher.UpdateRedis(request.Location, places, request.PlaceCat)

	if uint(len(places)) < request.MinNumResults {
		log.Debugf("Found %d POI results for place type %s, less than requested number of %d",
			len(places), request.PlaceCat, request.MinNumResults)
	}
	if len(places) == 0 {
		log.Debugf("No qualified POI result found in the given location %s, radius %d, and place type: %s",
			request.Location, request.Radius, request.PlaceCat)
		log.Debugf("location may be invalid")
	}
	return
}

//update Redis when hitting cache miss
func (poiSearcher *PoiSearcher) UpdateRedis(location string, places []POI.Place, placeCategory POI.PlaceCategory) {
	poiSearcher.redisClient.SetPlacesOnCategory(places)
	log.Debugf("Redis update complete")
}

//update MongoDB if number of results is not sufficient
func (poiSearcher *PoiSearcher) UpdateMongo(placeCat POI.PlaceCategory, places []POI.Place) {
	var wg sync.WaitGroup
	var numNewDocs uint64 = 0
	wg.Add(len(places))
	for _, place := range places {
		go poiSearcher.dbHandler.InsertPlace(place, placeCat, &wg, &numNewDocs)
	}
	wg.Wait()
	if numNewDocs > 0 {
		Logger.Infof("Inserted %d places into the database", numNewDocs)
	}
}
