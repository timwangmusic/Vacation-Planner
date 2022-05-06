package test

import (
	"testing"

	"github.com/weihesdlegend/Vacation-planner/POI"
)

func TestInsertInterval(t *testing.T) {
	gt := POI.GoogleMapsTimeIntervals{}
	gt.InsertTimeInterval(POI.TimeInterval{Start: 10, End: 20})
	gt.InsertTimeInterval(POI.TimeInterval{Start: 20, End: 23})
	gt.InsertTimeInterval(POI.TimeInterval{Start: 0, End: 7})
	gt.InsertTimeInterval(POI.TimeInterval{Start: 7, End: 10})
	expected := [][2]uint{{0, 7}, {7, 10}, {10, 20}, {20, 23}}
	if len(expected) != gt.NumIntervals() {
		t.Errorf("Incorrect number of intervals. Expected: %d, got: %d", len(expected), gt.NumIntervals())
	}
	for idx, interval := range *gt.GetAllIntervals() {
		if interval.Start != POI.Hour(expected[idx][0]) || interval.End != POI.Hour(expected[idx][1]) {
			t.Errorf("Interval setting for %d-th interval is wrong", idx)
		}
	}
}
