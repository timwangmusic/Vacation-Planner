package test

import (
	"testing"

	"github.com/weihesdlegend/Vacation-planner/POI"
)

func TestParseTimeIntervals(t *testing.T) {
	cases := []struct {
		name             string
		openHour         string
		expectedInterval POI.TimeInterval
	}{
		{
			name:     "should parse closed correctly",
			openHour: "Wednesday: Closed",
			expectedInterval: POI.TimeInterval{
				Start: 255,
				End:   255,
			},
		},
		{
			name:     "should parse regular opening hours correctly",
			openHour: "Saturday: 10:30AM - 10:00PM",
			expectedInterval: POI.TimeInterval{
				Start: 10,
				End:   22,
			},
		},
		{
			name:     "should parse overnight hours correctly",
			openHour: "Friday: 7:30PM - 4:30AM",
			expectedInterval: POI.TimeInterval{
				Start: 19,
				End:   24,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name,
			func(t *testing.T) {
				actualInterval, err := POI.ParseTimeInterval(tc.openHour)
				if err != nil {
					t.Fatal(err)
				}
				if actualInterval.Start != tc.expectedInterval.Start {
					t.Errorf("expected expectedInterval start %d, got %d", tc.expectedInterval.Start, actualInterval.Start)
				}
				if actualInterval.End != tc.expectedInterval.End {
					t.Errorf("expected expectedInterval end %d, got %d", tc.expectedInterval.End, actualInterval.End)
				}
			})
	}
}
