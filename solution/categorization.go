package solution

import "Vacation-planner/matching"

type CategorizedPlaces struct {
	EateryPlaces []matching.Place
	VisitPlaces  []matching.Place
}

func Categorize(cluster matching.PlaceCluster) CategorizedPlaces {
	res := CategorizedPlaces{
		EateryPlaces: make([]matching.Place, 0),
		VisitPlaces:  make([]matching.Place, 0),
	}

	for _, place := range cluster.Places {
		if place.CatTag == "Visit" {
			res.VisitPlaces = append(res.VisitPlaces, place)
		} else if place.CatTag == "Eatery" {
			res.EateryPlaces = append(res.EateryPlaces, place)
		}
	}

	return res
}
