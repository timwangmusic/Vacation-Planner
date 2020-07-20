package matching

import (
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
		delete(recordTable.NewRecord, key)
	}
}

/*
	Knapsack v2 uses sparse matrix like storage for step values and saves memory
	Knapsack v1 is migrated to knapsack_old_testonly.go
*/
func Knapsackv2(places []Place, timeLimit uint8, budget uint) (results []Place) {
	//Initialize knapsack data structures
	var recordtable knapsackRecordTable
	rt := &recordtable
	rt.Init(timeLimit, budget)
	optimalNode := knapsackNodeRecord{0, 0, SelectionThreshold, make([]Place, 0)}
	//DP process
	var staytime POI.StayingTime
	for k := 0; k < len(places); k++ {
		rt.update()
		staytime = POI.GetStayingTimeForLocationType(places[k].PlaceType)
		for key, record := range rt.SavedRecord {
			currentTravelTime, cost := rt.getTimeLimitAndCost(key)
			newTravelTime := currentTravelTime + uint8(staytime)
			newCost := cost + uint(math.Ceil(places[k].Price))
			if newTravelTime <= rt.timeLimit && newCost <= budget {
				newKey := rt.getKey(newTravelTime, newCost)
				newSolution := make([]Place, len(record.Solution))
				copy(newSolution, record.Solution)
				newSolution = append(newSolution, places[k])
				newScore := Score(newSolution)
				newRecord := knapsackNodeRecord{newTravelTime, newCost, newScore, newSolution}
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
	return optimalNode.Solution
}
