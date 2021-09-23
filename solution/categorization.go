package solution

import (
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/matching"
)

type CategorizedPlaces struct {
	Places       map[POI.PlaceCategory][]matching.Place
}

func Categorize(placesCluster matching.PlacesClusterForTime) CategorizedPlaces {
	res := CategorizedPlaces{
		Places:       make(map[POI.PlaceCategory][]matching.Place),
	}

	// this slice initialization section needs update when more categories are added
	res.Places[POI.PlaceCategoryVisit] = make([]matching.Place, 0)
	res.Places[POI.PlaceCategoryEatery] = make([]matching.Place, 0)

	for _, place := range placesCluster.Places {
		res.Places[place.GetPlaceCategory()] = append(res.Places[place.GetPlaceCategory()], place)
	}

	return res
}
