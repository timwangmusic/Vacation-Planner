package graph

import (
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"googlemaps.github.io/maps"
	"strings"
	"sync"
)

type PlaceDetailsResultMap struct {
	sync.Mutex
	results map[string]maps.PlaceDetailsResult
}

func newPlaceDetailsResultMap() *PlaceDetailsResultMap {
	return &PlaceDetailsResultMap{results: map[string]maps.PlaceDetailsResult{}}
}

func placeNeedUpdate(place *POI.Place) bool {
	return len(strings.TrimSpace(place.GetURL())) == 0
}

func updatePlacesDetails(searcher *iowrappers.PoiSearcher, places []POI.Place) {
	m := newPlaceDetailsResultMap()
	// for filter places need update and look up in places
	placeIdToIdx := make(map[string]int)
	for idx, place := range places {
		if placeNeedUpdate(&place) {
			placeIdToIdx[place.GetID()] = idx
		}
	}

	wg := sync.WaitGroup{}
	for placeId := range placeIdToIdx {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			result, err := searcher.GetMapsClient().PlaceDetailedSearch(id)
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
	}
}

func updatePlaceDetails(place *POI.Place, details maps.PlaceDetailsResult) {
	place.URL = details.URL
}
