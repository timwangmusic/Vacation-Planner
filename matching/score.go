// Package matching score design doc: https://bit.ly/2OTuBhM
package matching

import (
	"math"

	"github.com/weihesdlegend/Vacation-planner/utils"
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/stat"
)

const (
	MaxPlaceRating = 5.0
)

// OLD plan scoring method, use maxDist as the normalisation factor
// TODO, RW: remove in the future
func ScoreOld(places []Place) float64 {
	if len(places) == 1 {
		return PlaceScore(places[0])
	}
	distances := calDistances(places)                     // Haversine distances
	maxDist := math.Max(0.001, calMaxDistance(distances)) // protect against maximum distance being zero
	avgDistance := stat.Mean(distances, nil) / maxDist    // normalized average distance
	avgScore := avgPlacesScore(places)

	return avgScore - avgDistance
}

// Score uses constant distance normalisation factor
func Score(places []Place, distNorm int) float64 {
	if len(places) == 1 {
		return PlaceScore(places[0])
	}
	distances := calDistances(places)                            // Haversine distances
	avgDistance := stat.Mean(distances, nil) / float64(distNorm) // normalized average distance
	avgScore := avgPlacesScore(places)

	return avgScore - avgDistance
}

func PlaceScore(place Place) float64 {
	var boostFactor float64
	if place.PlacePrice() == 0 {
		boostFactor = float64(place.Rating()) / MaxPlaceRating
	} else {
		boostFactor = float64(place.Rating()) / place.PlacePrice()
	}
	return math.Log10(float64(1+place.UserRatingsCount())) * boostFactor
}

// calculate Haversine distances between places
func calDistances(places []Place) []float64 {
	distances := make([]float64, len(places)-1)

	for i := 0; i < len(distances); i++ {
		locationX := places[i].Location()
		locationY := places[i+1].Location()
		distances[i] = utils.HaversineDist([]float64{locationX.Latitude, locationX.Longitude}, []float64{locationY.Latitude, locationY.Longitude})
	}
	return distances
}

func calMaxDistance(distances []float64) float64 {
	return floats.Max(distances)
}

// calculate normalized average rating to price ratio
func avgPlacesScore(places []Place) float64 {
	numPlaces := len(places)
	placeScores := make([]float64, numPlaces)
	for k, place := range places {
		placeScores[k] = PlaceScore(place)
	}
	return stat.Mean(placeScores, nil)
}
