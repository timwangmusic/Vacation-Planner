// score design doc: https://bit.ly/2OTuBhM
package matching

import (
	"Vacation-planner/utils"
	"gonum.org/v1/gonum/stat"
	"gonum.org/v1/gonum/floats"
)

func Score(places []Place) float64{
	if len(places) == 1{
		maxRating := 5.0
		if places[0].Price == 0{
			return float64(places[0].Rating) / maxRating
		}
		return float64(places[0].Rating) / places[0].Price
	}
	distances := calDistances(&places)	// Haversine distances
	maxDist := calMaxDistance(distances)	// maximum distance
	avgDistance := stat.Mean(distances, nil) / maxDist // normalized average distance

	avgRatingPriceRatio := calAvgRatingPriceRatio(&places) // normalized average rating to price ratio

	return avgRatingPriceRatio - avgDistance
}

// calculate Haversine distances between places
func calDistances(places *[]Place) []float64{
	places_ := *places
	distances := make([]float64, len(*places)-1)

	for i := 0; i < len(distances); i++{
		locationX := places_[i].Location
		locationY := places_[i+1].Location
		distances[i] = utils.HaversineDist([]float64{locationX[0], locationX[1]}, []float64{locationY[0], locationY[1]})
	}
	return distances
}

// calculate average distance
func calMaxDistance(distances []float64) float64{
	return floats.Max(distances)
}

// calculate normalized average rating to price ratio
func calAvgRatingPriceRatio(places *[]Place) float64{
	numPlaces := len(*places)
	ratingPriceRatios := make([]float64, numPlaces)
	for k, place := range *places{
		if place.Price == 0{	// ratings <= 5.0 and prices >= 10, makes an upper bound for the ratio of 0.5
			maxRatio := 0.5
			ratingPriceRatios[k] = maxRatio
		} else {
			ratio := float64(place.Rating) / float64(place.Price)
			ratingPriceRatios[k] = ratio
		}
	}
	return stat.Mean(ratingPriceRatios, nil) / floats.Max(ratingPriceRatios)
}
