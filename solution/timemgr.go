package solution

import (
	"github.com/weihesdlegend/Vacation-planner/matching"
	"github.com/weihesdlegend/Vacation-planner/utils"
)

func GetTimeSlotLengthInMin(placeClusters []matching.PlacesClusterForTime) int {
	if len(placeClusters) == 0 {
		return 0
	}
	var start = placeClusters[0].Slot.Slot.Start
	var end = placeClusters[len(placeClusters)-1].Slot.Slot.End
	var min = int((end - start) * 60)
	return utils.MaxInt(0, min)
}
