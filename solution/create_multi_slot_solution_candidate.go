package solution

import (
	"strings"

	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"github.com/weihesdlegend/Vacation-planner/matching"
)

type PlanningSolution struct {
	PlaceNames     []string            `json:"place_names"`
	PlaceIDS       []string            `json:"place_ids"`
	PlaceLocations [][2]float64        `json:"place_locations"` // lat,lng
	PlaceAddresses []string            `json:"place_addresses"`
	PlaceURLs      []string            `json:"place_urls"`
	PlaceTypes     []POI.PlaceCategory `json:"place_types"`
	Score          float64             `json:"score"`
	IsSet          bool                `json:"is_set"`
}

func CreateCandidate(iter MultiDimIterator, placeClusters [][]matching.Place) (res PlanningSolution) {
	if len(iter.Status) != len(placeClusters) {
		return
	}
	// deduplication of repeating places in the result
	record := make(map[string]bool)
	places := make([]matching.Place, len(iter.Status))
	for idx, placeIdx := range iter.Status {
		placesByCategory := placeClusters[idx]

		var place = placesByCategory[placeIdx]

		// if the same place appears in two indexes, return incomplete result
		if _, exist := record[place.GetPlaceId()]; exist {
			return
		}

		record[place.GetPlaceId()] = true
		places[idx] = place
		res.PlaceIDS = append(res.PlaceIDS, place.GetPlaceId())
		res.PlaceNames = append(res.PlaceNames, place.GetPlaceName())
		res.PlaceLocations = append(res.PlaceLocations, [2]float64{place.GetLocation().Latitude, place.GetLocation().Longitude})
		res.PlaceAddresses = append(res.PlaceAddresses, place.GetPlaceFormattedAddress())
		res.PlaceTypes = append(res.PlaceTypes, place.GetPlaceCategory())
		if len(strings.TrimSpace(place.GetURL())) == 0 {
			place.SetURL(iowrappers.GoogleSearchHomePageURL)
		}
		res.PlaceURLs = append(res.PlaceURLs, place.GetURL())
	}
	res.Score = matching.Score(places)
	res.IsSet = true
	return
}
