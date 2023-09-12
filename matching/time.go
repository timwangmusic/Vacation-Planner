// Package matching matches request from planner for a particular day
package matching

import (
	"fmt"
	"github.com/weihesdlegend/Vacation-planner/POI"
)

type TimeSlot struct {
	Slot POI.TimeInterval `json:"slot"`
}

func (t *TimeSlot) ToString() string {
	return fmt.Sprintf("from %d to %d", t.Slot.Start, t.Slot.End)
}

type TimeInterval struct {
	Day       POI.Weekday
	StartHour uint8
	EndHour   uint8
}

type PlacesClusterForTime struct {
	Places []Place  `json:"places"`
	Slot   TimeSlot `json:"time slot"`
}

func (interval *TimeInterval) AddOffsetHours(offsetHour uint8) (intervalOut TimeInterval, valid bool) {
	//If a stay time after the start time exceeds the end of day, return false
	if intervalOut.StartHour+offsetHour > interval.EndHour {
		valid = false
		return
	}
	intervalOut.Day = interval.Day
	intervalOut.StartHour = interval.StartHour + offsetHour
	intervalOut.EndHour = interval.EndHour
	valid = true
	return
}
