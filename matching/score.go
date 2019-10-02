// score design doc: https://bit.ly/2OTuBhM
package matching

import (
	"github.com/weihesdlegend/Vacation-planner/utils"
	"gonum.org/v1/gonum/floats"
	"math"
)

type KnapsackNode struct {
	score float64
	solution []Place
}
func KnapsackMatrixCopy(dst [][]KnapsackNode, src [][]KnapsackNode){
	rowDst:=len(dst)
	rowSrc:=len(src)
	if rowDst != rowSrc {
		return
	}
	colDst:=len(dst)
	colSrc:=len(src)
	if colDst != colSrc {
		return
	}
	for i:=0; i<rowSrc; i++{
		for j:=0; j<colSrc; j++{
			dst[i][j].score = src[i][j].score
			copy(dst[i][j].solution, src[i][j].solution)
		}
	}
}
func Knapsack(places []Place, staytime []uint8, timeLimit uint8, budget uint) (results []Place){
	//INIT
	current := make([][]KnapsackNode, timeLimit)
	for i:=0; i<int(timeLimit); i++ {
		current[i] = make([]KnapsackNode, budget)
		for j:=0; j<int(budget); j++{
			current[i][j].score = 0
		}
	}
	next := make([][]KnapsackNode, timeLimit)
	for i:=0; i<int(timeLimit); i++ {
		next[i] = make([]KnapsackNode, budget)
		for j:=0; j<int(budget); j++{
			next[i][j].score = 0
		}
	}
	optimalNode := KnapsackNode{0,make([]Place,0)}
	tempPlaces := make([]Place, 0)
	var tempScore float64 = 0
	tempi := 0
	tempj := 0
	//MAIN
	for k:=0;k<len(places);k++{
		KnapsackMatrixCopy(current,next)
		//Do 0,0
		if staytime[k] < timeLimit && int(math.Round(places[k].Price)) < int(budget) {
			tempPlaces = append(current[0][0].solution, places[k])
			tempScore = Score(tempPlaces)
			if tempScore > next[staytime[k]][int(math.Round(places[k].Price))].score {
				next[staytime[k]][int(math.Round(places[k].Price))].score = tempScore
				copy(next[staytime[k]][int(math.Round(places[k].Price))].solution, tempPlaces)
			}
			if tempScore > optimalNode.score{
				optimalNode.score = tempScore
				copy(optimalNode.solution, tempPlaces)
			}
		}
		//Do others
		for i:=0; i<int(timeLimit); i++ {
			for j:=0; j<int(budget); j++{
				if current[i][j].score >0{
					if i + int(staytime[k]) < int(timeLimit) && j + int(math.Round(places[k].Price)) < int(budget){
						tempi = i + int(staytime[k])
						tempj = j + int(math.Round(places[k].Price))
						tempPlaces = append(current[i][j].solution, places[k])
						tempScore = Score(tempPlaces)
						if tempScore > next[tempi][tempj].score {
							next[tempi][tempj].score = tempScore
							copy(next[tempi][tempj].solution, tempPlaces)
						}
						if tempScore > optimalNode.score{
							optimalNode.score = tempScore
							copy(optimalNode.solution, tempPlaces)
						}
					}
				}
			}
		}
	}
	return optimalNode.solution
}

func Score(places []Place) float64{
	if len(places) == 1{
		maxRating := 5.0
		if places[0].Price == 0{
			return float64(places[0].Rating) / maxRating
		}
		return float64(places[0].Rating) / places[0].Price
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
		locationX := places[i].Location
		locationY := places[i+1].Location
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
		if place.Price == 0 {
			ratingPriceRatios[k] = AvgRating / AvgPricing
		} else {
			ratio := float64(place.Rating) / place.Price
			ratingPriceRatios[k] = ratio
		}
	}
	return stat.Mean(ratingPriceRatios, nil) / floats.Max(ratingPriceRatios)
}
