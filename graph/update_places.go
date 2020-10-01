package graph

import (
	"github.com/sirupsen/logrus"
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"strings"
	"sync"
)

type PlaceDetailsResultMap struct {
	sync.Mutex
	results map[string]POI.Place
}

func detailsResultMap() *PlaceDetailsResultMap {
	return &PlaceDetailsResultMap{results: map[string]POI.Place{}}
}

func placeNeedUpdate(place *POI.Place) bool {
	if len(strings.TrimSpace(place.GetURL())) == 0 || place.GetURL() == iowrappers.GoogleSearchHomePageURL {
		logrus.Debugf("[REDIS URL UPDATE] place with ID: %s needs URL update", place.GetID())
		return true
	}
	return false
}

// use this function as a migration method as more fields are added to the POI.Place data model
func UpdatePlacesDetails(searcher iowrappers.SearchClient, places []POI.Place) (placesNeedUpdate []POI.Place) {
	m := detailsResultMap()
	// for filter places need update and look up in places
	placeIdToIdx := make(map[string]int)
	for idx, place := range places {
		if placeNeedUpdate(&place) {
			placeIdToIdx[place.GetID()] = idx
		}
	}

	placesNeedUpdate = make([]POI.Place, 0)
	wg := sync.WaitGroup{}
	for placeId := range placeIdToIdx {
		wg.Add(1)
		placeId := placeId
		go func(id string) {
			defer wg.Done()
			result, err := searcher.PlaceDetailsSearch(placeId)
			if err != nil {
				iowrappers.Logger.Error(err)
				return
			}
			defer m.Unlock()
			m.Lock()
			m.results[id] = result
		}(placeId)
	}
	wg.Wait()

	for placeId, idx := range placeIdToIdx {
		updatePlaceDetails(&places[idx], m.results[placeId])
		placesNeedUpdate = append(placesNeedUpdate, places[idx])
	}
	return placesNeedUpdate
}

func updatePlaceDetails(place *POI.Place, details POI.Place) {
	place.SetURL(details.GetURL())
	logrus.Debugf("[REDIS URL UPDATE] the URL of place with ID: %s has been updated to %s", place.GetID(), place.GetURL())
}
