package solution

import (
	"Vacation-planner/matching"
	"Vacation-planner/planner"
	"Vacation-planner/utils"
)

const DIS2MIN_TEST = 0.05

func GetSlotLengthinMin(pcluster *matching.PlaceCluster) int {
	var start = pcluster.Slot.Slot.Start
	var end = pcluster.Slot.Slot.End
	var min = int((end - start) * 60)
	if min <= 0 {
		return 0
	} else {
		return min
	}
}

func GetTravelTimeByDistance(cclusters planner.CategorizedPlaces, mdti planner.MDtagIter) ([]float64, float64) {
	var travelTime = make([]float64, len(mdti.Tag), len(mdti.Tag))
	var sumTime float64 = 0
	var startplace matching.Place
	var endplace matching.Place
	var invalid bool = false
	for i := 0; i < len(mdti.Tag); i++ {
		if mdti.Tag[i] == 'E' || mdti.Tag[i] == 'e' {
			startplace = cclusters.EateryPlaces[mdti.Status[i]]
		} else if mdti.Tag[i] == 'V' || mdti.Tag[i] == 'v' {
			startplace = cclusters.VisitPlaces[mdti.Status[i]]
		}
		if i < len(mdti.Tag)-1 {
			if mdti.Tag[i+1] == 'E' || mdti.Tag[i+1] == 'e' {
				endplace = cclusters.EateryPlaces[mdti.Status[i+1]]
			} else if mdti.Tag[i+1] == 'V' || mdti.Tag[i+1] == 'v' {
				endplace = cclusters.VisitPlaces[mdti.Status[i+1]]
			}
		} else {
			endplace = matching.Place{}
			invalid = true
			//TODO: Put default endplace from slot here.
		}
		if invalid {
			continue
		}
		locationX := startplace.Location
		locationY := endplace.Location
		travelTime[i] = utils.HaversineDist([]float64{locationX[0], locationX[1]}, []float64{locationY[0], locationY[1]}) * DIS2MIN_TEST
		sumTime += travelTime[i]
	}
	return travelTime, sumTime
}