package test

import (
	"Vacation-planner/POI"
	"testing"
)

func TestInsertInterval(t *testing.T){
	gt := POI.GoogleMapsTimeIntervals{}
	gt.InsertTimeInterval(POI.TimeInterval{10,20})
	gt.InsertTimeInterval(POI.TimeInterval{20, 23})
	gt.InsertTimeInterval(POI.TimeInterval{0, 7})
	gt.InsertTimeInterval(POI.TimeInterval{7,10})
	expected := [][2]uint{{0, 7}, {7, 10}, {10, 20}, {20, 23}}
	if len(expected) != gt.NumIntervals(){
		t.Errorf("Incorrect number of intervals. Expected: %d, got: %d", len(expected), gt.NumIntervals())
	}
	for idx, interval := range *gt.GetAllIntervals(){
		if interval.Start != POI.Hour(expected[idx][0]) || interval.End != POI.Hour(expected[idx][1]){
			t.Errorf("Interval setting for %d-th interval is wrong", idx)
		}
	}
}
