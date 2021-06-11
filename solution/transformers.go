package solution

import (
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/matching"
)

//ToTimeSlots transforms a list of SlotRequest to a list of matching.TimeSlot
func ToTimeSlots(slotRequests []SlotRequest) []matching.TimeSlot {
	timeSlots := make([]matching.TimeSlot, len(slotRequests))
	for idx := range slotRequests {
		timeSlots[idx] = slotRequests[idx].TimeSlot
	}
	return timeSlots
}

func ToSlotCategories(slotRequests []SlotRequest) []POI.PlaceCategory {
	categories := make([]POI.PlaceCategory, len(slotRequests))
	for idx := range slotRequests {
		categories[idx] = slotRequests[idx].Category
	}
	return categories
}
