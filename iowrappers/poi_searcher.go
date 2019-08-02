package iowrappers

import (
	"Vacation-planner/POI"
	"Vacation-planner/utils"
	"log"
)

type PlaceSearcher interface {
	NearbySearch(request *PlaceSearchRequest) []POI.Place
}

type PoiSearcher struct {
	mapsClient  *MapsClient
	dbHandler   *DbHandler
	redisClient *RedisClient
}

func (poiSearcher *PoiSearcher) Init(mapsClient *MapsClient, dbName string, db_url string,
	redis_addr string, redis_psw string, redis_idx int) {
	if mapsClient == nil || mapsClient.client == nil {
		log.Fatal("maps client is nil")
	}
	poiSearcher.mapsClient = mapsClient

	poiSearcher.dbHandler = &DbHandler{}
	// delegate error check of db handler to dbHandler.Init
	poiSearcher.dbHandler.Init(dbName, db_url)

	poiSearcher.redisClient = &RedisClient{}
	poiSearcher.redisClient.Init(redis_addr, redis_psw, redis_idx)
}

// if client API key is invalid but not empty string, nearby search result will be empty
func (poiSearcher *PoiSearcher) NearbySearch(request *PlaceSearchRequest) (places []POI.Place) {
	dbHandler := poiSearcher.dbHandler
	dbHandler.SetCollHandler(string(request.PlaceCat))

	cachedPlaces := poiSearcher.redisClient.NearbySearch(request)
	if uint(len(cachedPlaces)) >= request.MinNumResults {
		log.Printf("Place Type: %s, Using Redis to fulfill request! \n", request.PlaceCat)
		maxResultNum := utils.MinInt(len(cachedPlaces), int(request.MaxNumResults))
		places = append(places, cachedPlaces[:maxResultNum]...)
		return
	} else {
		dbStoredPlaces, err := dbHandler.PlaceSearch(request)
		utils.CheckErr(err)
		if uint(len(dbStoredPlaces)) < request.MinNumResults {
			// Call external API only when both cache and database cannot fulfill request
			newPlaces := poiSearcher.mapsClient.NearbySearch(request)
			maxResultNum := utils.MinInt(len(newPlaces), int(request.MaxNumResults))
			places = append(places, newPlaces[:maxResultNum]...)
			// update database
			poiSearcher.UpdateMongo(request.PlaceCat, newPlaces)
		} else {
			log.Printf("Place Type: %s, Using MongoDB to fulfill request! \n", request.PlaceCat)
			maxResultNum := utils.MinInt(len(dbStoredPlaces), int(request.MaxNumResults))
			places = append(places, dbStoredPlaces[:maxResultNum]...)
		}
	}
	// update cache
	poiSearcher.UpdateRedis(request.Location, places, request.PlaceCat)

	if uint(len(places)) < request.MinNumResults {
		log.Printf("Number of POI results found is %d, less than requested %d",
			len(places), request.MinNumResults)
	}
	if len(places) == 0 {
		log.Printf("No qualified POI result found in the given location %s, radius %d, place type: %s",
			request.Location, request.Radius, request.PlaceCat)
		log.Printf("location may be invalid")
	}
	return
}

//update Redis when hitting cache miss
func (poiSearcher *PoiSearcher) UpdateRedis(location string, places []POI.Place, placeCategory POI.PlaceCategory) {
	poiSearcher.redisClient.StorePlacesForLocation(location, places, placeCategory)
	log.Printf("Redis update complete")
}

//TODO: use bulk insert for new places
//update MongoDB if number of results is not sufficient
func (poiSearcher *PoiSearcher) UpdateMongo(placeCat POI.PlaceCategory, places []POI.Place) {
	for _, place := range places {
		utils.CheckErr(poiSearcher.dbHandler.InsertPlace(place, placeCat))
	}
	log.Printf("Inserted %d places into the database", len(places))
}
