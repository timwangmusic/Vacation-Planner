package solution

import (
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/matching"
)

// ToPlaceView transforms matching.Place to solution.PlaceView
func ToPlaceView(place matching.Place) PlaceView {
	placeView := PlaceView{
		ID:           place.GetPlaceId(),
		Name:         place.GetPlaceName(),
		URL:          place.GetURL(),
		Rating:       place.GetRating(),
		RatingsCount: place.GetUserRatingsCount(),
		PriceLevel:   place.Place.GetPriceLevel(),
		Hours:        place.GetHours(),
	}
	return placeView
}

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
