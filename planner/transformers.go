package planner

import (
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"github.com/weihesdlegend/Vacation-planner/matching"
	"regexp"
	"strconv"
	"strings"
	"time"
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

func toRedisRequest(req *PlanningReq) *iowrappers.PlanningSolutionsCacheRequest {
	stayTimes := toTimeSlots(req.Slots)
	intervals := make([]POI.TimeInterval, len(stayTimes))
	for idx, stayTime := range stayTimes {
		intervals[idx] = stayTime.Slot
	}

	return &iowrappers.PlanningSolutionsCacheRequest{
		Location:        req.Location,
		Radius:          uint64(req.SearchRadius),
		PriceLevel:      req.PriceLevel,
		PlaceCategories: toPlaceCategories(req.Slots),
		Intervals:       intervals,
		Weekday:         req.Weekday,
	}
}

func toWeekday(date string) POI.Weekday {
	datePattern := regexp.MustCompile(`(?P<year>\d{4})-(?P<month>\d{2})-(?P<day>\d{2})`)
	dateFields := datePattern.FindStringSubmatch(date)
	year, _ := strconv.Atoi(dateFields[1])
	month, _ := strconv.Atoi(dateFields[2])
	day, _ := strconv.Atoi(dateFields[3])
	t := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	return POI.Weekday(t.Weekday())
}

func toPriceLevel(priceLevel string) POI.PriceLevel {
	price, _ := strconv.Atoi(priceLevel)
	return POI.PriceLevel(price)
}
