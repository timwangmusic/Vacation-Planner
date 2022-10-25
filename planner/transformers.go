package planner

import (
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/matching"
	"strings"
)

type CityView struct {
	City    string `json:"city"`
	Region  string `json:"region"`
	Country string `json:"country"`
}

func toCityViews(geocodes map[string]string) []CityView {
	var views []CityView
	for key := range geocodes {
		fields := strings.SplitN(key, "_", 3)
		if len(fields) == 2 {
			views = append(views, CityView{City: strings.TrimSpace(fields[0]), Country: strings.TrimSpace(fields[1])})
		} else if len(fields) == 3 {
			views = append(views, CityView{City: strings.TrimSpace(fields[0]), Region: strings.TrimSpace(fields[1]), Country: strings.TrimSpace(fields[2])})
		}
	}
	return views
}

func toString(view CityView) string {
	var results []string
	if strings.TrimSpace(view.City) != "" {
		results = append(results, strings.TrimSpace(view.City))
	}
	if strings.TrimSpace(view.Region) != "" {
		results = append(results, strings.TrimSpace(view.Region))
	}
	if strings.TrimSpace(view.Country) != "" {
		results = append(results, strings.TrimSpace(view.Country))
	}
	return strings.Join(results, ", ")
}

func toTimeSlots(slotRequests []SlotRequest) []matching.TimeSlot {
	timeSlots := make([]matching.TimeSlot, len(slotRequests))
	for idx := range slotRequests {
		timeSlots[idx] = slotRequests[idx].TimeSlot
	}
	return timeSlots
}

func toPlaceCategories(slotRequests []SlotRequest) []POI.PlaceCategory {
	categories := make([]POI.PlaceCategory, len(slotRequests))
	for idx := range slotRequests {
		categories[idx] = slotRequests[idx].Category
	}
	return categories
}
