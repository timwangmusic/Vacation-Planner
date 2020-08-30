// score design doc: https://bit.ly/2OTuBhM
package matching

import (
	"github.com/weihesdlegend/Vacation-planner/utils"
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/stat"
	"math"
)

const (
	AvgRating  = 3.0
	AvgPricing = PriceLevel2
)

func Score(places []Place) float64 {
	if len(places) == 1 {
		if places[0].GetPrice() == 0 {
			return AvgRating / AvgPricing // set to average single Place rating-price ratio
		}
		return float64(places[0].GetRating()) / places[0].GetPrice()
	}
	distances := calDistances(places)                     // Haversine distances
	maxDist := math.Max(0.001, calMaxDistance(distances)) // protect against maximum distance being zero
	avgDistance := stat.Mean(distances, nil) / maxDist    // normalized average distance

	avgRatingPriceRatio := calAvgRatingPriceRatio(places) // normalized average rating to price ratio

	return avgRatingPriceRatio - avgDistance
}

// calculate Haversine distances between places
func calDistances(places []Place) []float64 {
	distances := make([]float64, len(places)-1)

	for i := 0; i < len(distances); i++ {
		locationX := places[i].GetLocation()
		locationY := places[i+1].GetLocation()
		distances[i] = utils.HaversineDist([]float64{locationX[0], locationX[1]}, []float64{locationY[0], locationY[1]})
	}
	return distances
}

func calMaxDistance(distances []float64) float64 {
	return floats.Max(distances)
}

// calculate normalized average rating to price ratio
func calAvgRatingPriceRatio(places []Place) float64 {
	numPlaces := len(places)
	ratingPriceRatios := make([]float64, numPlaces)
	for k, place := range places {
		if place.GetPrice() == 0 {
			ratingPriceRatios[k] = AvgRating / AvgPricing
		} else {
			ratio := float64(place.GetRating()) / place.GetPrice()
			ratingPriceRatios[k] = ratio
		}
	}
	return stat.Mean(ratingPriceRatios, nil) / floats.Max(ratingPriceRatios)
}
