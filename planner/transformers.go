package planner

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"github.com/weihesdlegend/Vacation-planner/matching"
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

func toSolutionsSaveRequest(req *PlanningRequest, solutions []iowrappers.PlanningSolutionRecord) *iowrappers.PlanningSolutionsSaveRequest {
	stayTimes := toTimeSlots(req.Slots)
	intervals := make([]POI.TimeInterval, len(stayTimes))
	for idx, stayTime := range stayTimes {
		intervals[idx] = stayTime.Slot
	}

	weekdays := make([]POI.Weekday, len(stayTimes))
	for idx := range weekdays {
		weekdays[idx] = req.Slots[idx].Weekday
	}

	return &iowrappers.PlanningSolutionsSaveRequest{
		Location:                req.Location,
		PriceLevel:              req.PriceLevel,
		PlaceCategories:         toPlaceCategories(req.Slots),
		Intervals:               intervals,
		Weekdays:                weekdays,
		PlanningSolutionRecords: solutions,
		NumPlans:                int64(req.NumPlans),
	}
}

func toWeekday(date string) POI.Weekday {
	datePattern := regexp.MustCompile(`(?P<year>\d{4})-(?P<month>\d{2})-(?P<day>\d{2})`)
	dateFields := datePattern.FindStringSubmatch(date)
	year, _ := strconv.Atoi(dateFields[1])
	month, _ := strconv.Atoi(dateFields[2])
	day, _ := strconv.Atoi(dateFields[3])
	t := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	// compensate for the difference between time.Weekday (starts on Sunday) and internal Weekday definition (starts on Monday)
	return POI.Weekday((t.Weekday() + 6) % 7)
}

func toPriceLevel(priceLevel string) POI.PriceLevel {
	price, _ := strconv.Atoi(priceLevel)
	return POI.PriceLevel(price)
}

func toPlacePlanningDetails(name string, slot SlotRequest, url string) PlacePlanningDetails {
	return PlacePlanningDetails{
		Name:     name,
		Category: string(slot.Category),
		TimeSlot: slot.TimeSlot,
		URL:      url,
	}
}

func slotToWeekday(slot SlotRequest) string {
	return slot.Weekday.Name()
}

func slotToTimeslot(slot SlotRequest) string {
	return slot.TimeSlot.ToString()
}

func toPlanningSolutionRecord(request *PlanningRequest, solution PlanningSolution, location POI.Location) iowrappers.PlanningSolutionRecord {
	weekdays := MapSlice(request.Slots, slotToWeekday)
	timeSlots := MapSlice(request.Slots, slotToTimeslot)
	return iowrappers.PlanningSolutionRecord{
		ID:              solution.ID,
		PlaceIDs:        solution.PlaceIDS,
		Score:           solution.Score,
		ScoreOld:        solution.ScoreOld,
		PlaceNames:      solution.PlaceNames,
		PlaceLocations:  solution.PlaceLocations,
		PlaceAddresses:  solution.PlaceAddresses,
		PlaceURLs:       solution.PlaceURLs,
		PlaceCategories: solution.PlaceCategories,
		Weekdays:        weekdays,
		TimeSlots:       timeSlots,
		Destination:     location,
		PlanSpec:        solution.PlanSpec,
	}
}

func toLocation(city iowrappers.City) POI.Location {
	return POI.Location{
		Latitude:          city.Latitude,
		Longitude:         city.Longitude,
		City:              city.Name,
		AdminAreaLevelOne: city.AdminArea1,
		Country:           city.Country,
	}
}

func locationToBlobKey(l *POI.Location) string {
	normalized := strings.Split(l.String(), ", ")

	slices.Reverse(normalized)
	return strings.Join(normalized, "/")
}

func toPATView(metadata *iowrappers.TokenMetadata) PATView {
	var expiresAt string
	if metadata.ExpiresAt != nil {
		expiresAt = metadata.ExpiresAt.Format(time.RFC3339)
	}

	return PATView{
		Id:        metadata.Id,
		Name:      metadata.Name,
		ExpiresAt: expiresAt,
	}
}
