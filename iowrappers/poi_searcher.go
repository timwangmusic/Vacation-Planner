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
	mapsClient *MapsClient
	dbHandler  *DbHandler
}

func (poiSearcher *PoiSearcher) Init(mapsClient *MapsClient, dbName string, url string) {
	if mapsClient == nil || mapsClient.client == nil {
		log.Fatal("maps client is nil")
	}
	poiSearcher.dbHandler = &DbHandler{}
	// delegate error check of db handler to dbHandler.Init
	poiSearcher.dbHandler.Init(dbName, url)
	poiSearcher.mapsClient = mapsClient
}

// if client API key is invalid but not empty string, nearby search result will be empty
func (poiSearcher *PoiSearcher) NearbySearch(request *PlaceSearchRequest) (places []POI.Place) {
	dbHandler := poiSearcher.dbHandler
	dbHandler.SetCollHandler(string(request.PlaceCat))
	storedPlaces, err := dbHandler.PlaceSearch(request)
	utils.CheckErr(err)
	if uint(len(storedPlaces)) < request.MinNumResults {
		places = append(places, poiSearcher.mapsClient.NearbySearch(request)...)
		for _, place := range places {
			utils.CheckErr(dbHandler.InsertPlace(place, request.PlaceCat))
		}
	} else {
		places = append(places, storedPlaces...)
	}
	if len(places) == 0 {
		log.Printf("No qualified POI result found in the given location %s, radius %d, place type: %s",
			request.Location, request.Radius, request.PlaceCat)
		log.Printf("maps client may be invalid")
	}
	return
}
