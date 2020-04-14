package matching

import (
	log "github.com/sirupsen/logrus"
	"github.com/weihesdlegend/Vacation-planner/POI"
	"math"
)

const SelectionThreshold = -1

type knapsackNodeRecord struct {
	timeUsed uint8
	cost     uint
	score    float64
	Solution []Place
}
type knapsackNode struct {
	score    float64
	solution []Place
}
type knapsackRecordTable struct {
	timeLimit   uint8
	budget      uint
	SavedRecord map[uint]knapsackNodeRecord
	NewRecord   map[uint]knapsackNodeRecord
}

func (recordTable *knapsackRecordTable) Init(timeLimit uint8, budget uint) {
	recordTable.timeLimit = timeLimit
	recordTable.budget = budget
	recordTable.SavedRecord = make(map[uint]knapsackNodeRecord)
	recordTable.NewRecord = make(map[uint]knapsackNodeRecord)
	start := knapsackNodeRecord{0, 0, SelectionThreshold, make([]Place, 0)}
	recordTable.SavedRecord[recordTable.getKey(0, 0)] = start
}

func (recordTable *knapsackRecordTable) getKey(timeLimit uint8, budget uint) (key uint) {
	key = uint(timeLimit)*recordTable.budget + budget
	return
}

func (recordTable *knapsackRecordTable) getTimeLimitAndCost(key uint) (timeLimt uint8, budget uint) {
	budget = key % recordTable.budget
	timeLimt = uint8((key - budget) / recordTable.budget)
	return
}

func (recordTable *knapsackRecordTable) update() {
	for key, record := range recordTable.NewRecord {
		if oldRecord, ok := recordTable.SavedRecord[key]; ok {
			if oldRecord.score < record.score {
				recordTable.SavedRecord[key] = record
			}
		} else {
			recordTable.SavedRecord[key] = record
		}
		//delete new record entries
		delete(recordTable.NewRecord, key)
	}
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

func Knapsack(places []Place, timeLimit uint8, budget uint) (results []Place) {
	//INIT
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
	tempPlaces := make([]Place, 0)
	var tempScore float64 = SelectionThreshold
	tempi := 0
	tempj := 0
	//MAIN
	var staytime POI.StayingTime
	for k := 0; k < len(places); k++ {
		KnapsackMatrixCopy(current, next)
		staytime = POI.GetStayingTimeForLocationType(places[k].PlaceType)
		//Do 0,0
		if uint8(staytime) <= timeLimit && int(math.Ceil(places[k].Price)) <= int(budget) {
			tempPlaces = append(current[0][0].solution, places[k])
			tempScore = Score(tempPlaces)
			if tempScore > next[staytime][int(math.Ceil(places[k].Price))].score {
				next[staytime][int(math.Round(places[k].Price))].score = tempScore
				next[staytime][int(math.Round(places[k].Price))].solution = append(make([]Place, 0, len(tempPlaces)), tempPlaces...)
			}
			if tempScore > optimalNode.score {
				optimalNode.score = tempScore
				optimalNode.solution = append(make([]Place, 0, len(tempPlaces)), tempPlaces...)
			}
		}
		//Do others
		for i := 0; i < int(timeLimit); i++ {
			for j := 0; j < int(budget); j++ {
				if current[i][j].score > SelectionThreshold {
					if i+int(staytime) <= int(timeLimit) && j+int(math.Ceil(places[k].Price)) <= int(budget) {
						tempi = i + int(staytime)
						tempj = j + int(math.Ceil(places[k].Price))
						tempPlaces = append(current[i][j].solution, places[k])
						tempScore = Score(tempPlaces)
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
	log.Debugf("Optimal rate %f", optimalNode.score)
	return optimalNode.solution
}

/*
Knapsack v2 uses sparse matrix like storage for step values and saves memory
*/
func Knapsackv2(places []Place, timeLimit uint8, budget uint) (results []Place) {
	//INIT
	var recordtable knapsackRecordTable
	rt := &recordtable
	rt.Init(timeLimit, budget)
	optimalNode := knapsackNodeRecord{0, 0, SelectionThreshold, make([]Place, 0)}
	//MAIN
	var staytime POI.StayingTime
	for k := 0; k < len(places); k++ {
		rt.update()
		staytime = POI.GetStayingTimeForLocationType(places[k].PlaceType)
		for key, record := range rt.SavedRecord {
			timeLimitBase, cost := rt.getTimeLimitAndCost(key)
			newTimeLimit := timeLimitBase + uint8(staytime)
			newCost := cost + uint(math.Ceil(places[k].Price))
			if newTimeLimit <= rt.timeLimit && newCost <= budget {
				newKey := rt.getKey(newTimeLimit, newCost)
				newSolution := make([]Place, len(record.Solution))
				copy(newSolution, record.Solution)
				newSolution = append(newSolution, places[k])
				newScore := Score(newSolution)
				newRecord := knapsackNodeRecord{newTimeLimit, newCost, newScore, newSolution}
				if alreadyRecord, ok := rt.NewRecord[newKey]; ok {
					if alreadyRecord.score < newRecord.score {
						rt.NewRecord[newKey] = newRecord
					}
				} else {
					rt.NewRecord[newKey] = newRecord
				}
				if newScore > optimalNode.score {
					optimalNode.score = newScore
					optimalNode.Solution = append([]Place(nil), newSolution...)
				}
			}
		}
	}
	log.Debugf("Optimal rate %f", optimalNode.score)
	return optimalNode.Solution
}
