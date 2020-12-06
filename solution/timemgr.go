package solution

import (
	"github.com/weihesdlegend/Vacation-planner/matching"
	"github.com/weihesdlegend/Vacation-planner/utils"
)

const Dis2minTest = 0.05

func GetTimeSlotLengthInMin(placeClusters []matching.TimePlacesCluster) int {
	if len(placeClusters) == 0 {
		return 0
	}
	var start = placeClusters[0].Slot.Slot.Start
	var end = placeClusters[len(placeClusters)-1].Slot.Slot.End
	var min = int((end - start) * 60)
	return utils.MaxInt(0, min)
}

func GetTravelTimeByDistance(cclusters []CategorizedPlaces, mdti MDtagIter) ([]float64, float64) {
	var travelTime = make([]float64, len(mdti.Tag), len(mdti.Tag))
	var sumTime float64 = 0
	var startPlace matching.Place
	var endPlace matching.Place
	var invalid bool
	for i := 0; i < len(mdti.Tag); i++ {
		if mdti.Tag[i] == 'E' || mdti.Tag[i] == 'e' {
			startPlace = cclusters[i].EateryPlaces[mdti.Status[i]]
		} else if mdti.Tag[i] == 'V' || mdti.Tag[i] == 'v' {
			startPlace = cclusters[i].VisitPlaces[mdti.Status[i]]
		}
		if i < len(mdti.Tag)-1 {
			if mdti.Tag[i+1] == 'E' || mdti.Tag[i+1] == 'e' {
				endPlace = cclusters[i+1].EateryPlaces[mdti.Status[i+1]]
			} else if mdti.Tag[i+1] == 'V' || mdti.Tag[i+1] == 'v' {
				endPlace = cclusters[i+1].VisitPlaces[mdti.Status[i+1]]
			}
		} else {
			endPlace = matching.Place{}
			invalid = true
			//TODO: Put default endPlace from slot here.
		}
		if invalid {
			continue
		}
		locationX := startPlace.GetLocation()
		locationY := endPlace.GetLocation()
		travelTime[i] = utils.HaversineDist([]float64{locationX[0], locationX[1]}, []float64{locationY[0], locationY[1]}) * Dis2minTest
		sumTime += travelTime[i]
	}
	return travelTime, sumTime
}
