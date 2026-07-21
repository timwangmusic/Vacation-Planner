package test

import (
	"testing"
	"time"

	"github.com/weihesdlegend/Vacation-planner/POI"
)

func TestWeekdayFromTime(t *testing.T) {
	tests := []struct {
		goDay time.Weekday
		want  POI.Weekday
	}{
		{time.Monday, POI.DateMonday},
		{time.Wednesday, POI.DateWednesday},
		{time.Saturday, POI.DateSaturday},
		{time.Sunday, POI.DateSunday},
	}
	for _, tt := range tests {
		if got := POI.WeekdayFromTime(tt.goDay); got != tt.want {
			t.Errorf("WeekdayFromTime(%v) = %v, want %v", tt.goDay, got, tt.want)
		}
	}
}

func TestKnownClosedOnDay(t *testing.T) {
	place := POI.Place{Hours: [7]string{
		"Monday: 9:00 AM – 5:00 PM",
		"Tuesday: 9:00 AM – 5:00 PM",
		"Wednesday: 9:00 AM – 5:00 PM",
		"Thursday: 9:00 AM – 5:00 PM",
		"Friday: 9:00 AM – 5:00 PM",
		"Saturday: Closed",
		"Sunday: Closed",
	}}

	if place.KnownClosedOnDay(POI.DateFriday) {
		t.Error("expect place to be open on Friday")
	}
	if !place.KnownClosedOnDay(POI.DateSaturday) {
		t.Error("expect place to be closed on Saturday")
	}
	if !place.KnownClosedOnDay(POI.DateSunday) {
		t.Error("expect place to be closed on Sunday")
	}

	// unknown hours must not be treated as closed
	unknown := POI.Place{}
	if unknown.KnownClosedOnDay(POI.DateSunday) {
		t.Error("expect place with unknown hours to not be reported as closed")
	}
}
