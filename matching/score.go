// score design doc: https://bit.ly/2OTuBhM
package matching

import (
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/utils"
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/stat"
	"math"
)

const (
	AvgRating  = 3.0
	AvgPricing = PRICE_LEVEL_2
)

const SelectionThreshold = -1


type KnapsackNode struct {
	score float64
	solution []Place
}
func KnapsackMatrixCopy(dst [][]KnapsackNode, src [][]KnapsackNode){
	rowDst:=len(dst)
	rowSrc:=len(src)
	if rowDst != rowSrc || rowDst ==0 || rowSrc == 0 {
		return
	}
	colDst:=len(dst[0])
	colSrc:=len(src[0])
	if colDst != colSrc {
		return
	}
	for i:=0; i<rowSrc; i++{
		for j:=0; j<colSrc; j++{
			dst[i][j].score = src[i][j].score
			dst[i][j].solution = append(make([]Place, 0, len(src[i][j].solution)), src[i][j].solution...)
		}
	}
}
func Knapsack(places []Place, timeLimit uint8, budget uint) (results []Place){
	//INIT
	current := make([][]KnapsackNode, timeLimit)
	for i:=0; i<int(timeLimit); i++ {
		current[i] = make([]KnapsackNode, budget)
		for j:=0; j<int(budget); j++{
			current[i][j].score = SelectionThreshold
		}
	}
	next := make([][]KnapsackNode, timeLimit)
	for i:=0; i<int(timeLimit); i++ {
		next[i] = make([]KnapsackNode, budget)
		for j:=0; j<int(budget); j++{
			next[i][j].score = SelectionThreshold
		}
	}
	optimalNode := KnapsackNode{SelectionThreshold,make([]Place,0)}
	tempPlaces := make([]Place, 0)
	var tempScore float64 = SelectionThreshold
	tempi := 0
	tempj := 0
	//MAIN
	var staytime POI.StayingTime
	for k:=0;k<len(places);k++{
		KnapsackMatrixCopy(current,next)
		staytime = POI.GetStayingTimeForLocationType(places[k].PlaceType)
		//Do 0,0
		if uint8(staytime) < timeLimit && int(math.Ceil(places[k].Price)) < int(budget) {
			tempPlaces = append(current[0][0].solution, places[k])
			tempScore = Score(tempPlaces)
			if tempScore > next[staytime][int(math.Ceil(places[k].Price))].score {
				next[staytime][int(math.Round(places[k].Price))].score = tempScore
				next[staytime][int(math.Round(places[k].Price))].solution =append(make([]Place, 0, len(tempPlaces)), tempPlaces...)
			}
			if tempScore > optimalNode.score{
				optimalNode.score = tempScore
				optimalNode.solution = append(make([]Place, 0, len(tempPlaces)), tempPlaces...)
			}
		}
		//Do others
		for i:=0; i<int(timeLimit); i++ {
			for j:=0; j<int(budget); j++{
				if current[i][j].score > SelectionThreshold{
					if i + int(staytime) < int(timeLimit) && j + int(math.Ceil(places[k].Price)) < int(budget){
						tempi = i + int(staytime)
						tempj = j + int(math.Ceil(places[k].Price))
						tempPlaces = append(current[i][j].solution, places[k])
						tempScore = Score(tempPlaces)
						if tempScore > next[tempi][tempj].score {
							next[tempi][tempj].score = tempScore
							next[tempi][tempj].solution = append(make([]Place, 0, len(tempPlaces)), tempPlaces...)
						}
						if tempScore > optimalNode.score{
							optimalNode.score = tempScore
							optimalNode.solution = append(make([]Place, 0, len(tempPlaces)), tempPlaces...)
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
		if places[0].Price == 0{
			return AvgRating / AvgPricing // set to average single place rating-price ratio
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
