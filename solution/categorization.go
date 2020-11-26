package solution

import (
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/matching"
)

type CategorizedPlaces struct {
	EateryPlaces []matching.Place
	VisitPlaces  []matching.Place
}

func Categorize(cluster matching.TimePlacesCluster) CategorizedPlaces {
	res := CategorizedPlaces{
		EateryPlaces: make([]matching.Place, 0),
		VisitPlaces:  make([]matching.Place, 0),
	}

	for _, place := range cluster.Places {
		if place.GetPlaceCategory() == POI.PlaceCategoryVisit {
			res.VisitPlaces = append(res.VisitPlaces, place)
		} else if place.GetPlaceCategory() == POI.PlaceCategoryEatery {
			res.EateryPlaces = append(res.EateryPlaces, place)
		}
	}

	return res
}
