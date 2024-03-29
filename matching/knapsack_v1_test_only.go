package matching

import (
	"math"

	"github.com/weihesdlegend/Vacation-planner/POI"
)

type knapsackNode struct {
	score    float64
	solution []Place
}

func KnapsackMatrixCopy(dst [][]knapsackNode, src [][]knapsackNode) {
	rowDst := len(dst)
	rowSrc := len(src)
	if rowDst != rowSrc || rowDst == 0 || rowSrc == 0 {
		return
	}
	colDst := len(dst[0])
	colSrc := len(src[0])
	if colDst != colSrc {
		return
	}
	for i := 0; i < rowSrc; i++ {
		for j := 0; j < colSrc; j++ {
			dst[i][j].score = src[i][j].score
			dst[i][j].solution = append(make([]Place, 0, len(src[i][j].solution)), src[i][j].solution...)
		}
	}
}

func KnapsackV1(places []Place, interval TimeInterval, budget uint) (results []Place) {
	timeLimit := interval.EndHour - interval.StartHour
	//INIT KNAPSACK MATRIX
	current := make([][]knapsackNode, timeLimit+1)
	for i := 0; i < int(timeLimit)+1; i++ {
		current[i] = make([]knapsackNode, budget+1)
		for j := 0; j < int(budget)+1; j++ {
			current[i][j].score = SelectionThreshold
		}
	}
	next := make([][]knapsackNode, timeLimit+1)
	for i := 0; i < int(timeLimit)+1; i++ {
		next[i] = make([]knapsackNode, budget+1)
		for j := 0; j < int(budget)+1; j++ {
			next[i][j].score = SelectionThreshold
		}
	}
	optimalNode := knapsackNode{SelectionThreshold, make([]Place, 0)}
	var tempPlaces []Place
	var tempScore float64
	tempi := 0
	tempj := 0

	//KNAPSACK DP process
	var staytime POI.StayingTime
	for k := 0; k < len(places); k++ {
		KnapsackMatrixCopy(current, next)
		staytime = POI.GetStayingTimeForLocationType(places[k].Type())
		//INITIALIZE 0,0
		if uint8(staytime) <= timeLimit && int(math.Ceil(places[k].Price)) <= int(budget) && places[k].IsOpenBetween(interval, uint8(staytime)) {
			tempPlaces = append(current[0][0].solution, places[k])
			tempScore = ScoreOld(tempPlaces)
			if tempScore > next[staytime][int(math.Ceil(places[k].PlacePrice()))].score {
				next[staytime][int(math.Round(places[k].PlacePrice()))].score = tempScore
				next[staytime][int(math.Round(places[k].PlacePrice()))].solution = append(make([]Place, 0, len(tempPlaces)), tempPlaces...)
			}
			if tempScore > optimalNode.score {
				optimalNode.score = tempScore
				optimalNode.solution = append(make([]Place, 0, len(tempPlaces)), tempPlaces...)
			}
		}
		//DP to the remaining matrix
		for i := 0; i < int(timeLimit); i++ {
			for j := 0; j < int(budget); j++ {
				if current[i][j].score > SelectionThreshold {
					currentQueryStart, _ := interval.AddOffsetHours(uint8(i))
					if i+int(staytime) <= int(timeLimit) && j+int(math.Ceil(places[k].Price)) <= int(budget) && places[k].IsOpenBetween(currentQueryStart, uint8(staytime)) {
						tempi = i + int(staytime)
						tempj = j + int(math.Ceil(places[k].PlacePrice()))
						tempPlaces = append(current[i][j].solution, places[k])
						tempScore = ScoreOld(tempPlaces)
						if tempScore > next[tempi][tempj].score {
							next[tempi][tempj].score = tempScore
							next[tempi][tempj].solution = append(make([]Place, 0, len(tempPlaces)), tempPlaces...)
						}
						if tempScore > optimalNode.score {
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
