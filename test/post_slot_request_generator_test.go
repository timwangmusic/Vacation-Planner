package test

import (
	"Vacation-planner/POI"
	"Vacation-planner/matching"
	"Vacation-planner/planner"
	"testing"
)

func TestPostSlotRequestGenerator(t *testing.T) {
	req := planner.PlanningPostRequest{
		Country:   "USA",
		City:      "Seattle",
		Weekday:   0,
		StartTime: 8,
		EndTime:   20,
		NumVisit:  4,
		NumEatery: 3,
	}

	slotRequests := planner.GenSlotRequests(req)

	if len(slotRequests) != 3 {
		t.Errorf("wrong number of slot requests generated. expected: %d, got: %d", 3, len(slotRequests))
	}

	expectedEvOptions := []string{"EVEV", "EV", "V"}
	for idx, expectedEvOption := range expectedEvOptions {
		if slotRequests[idx].EvOption != expectedEvOption {
			t.Errorf("expected EV option does not match. expected: %s, got: %s",
				expectedEvOption, slotRequests[idx].EvOption)
		}
	}

	var expectedFirstSlotStayTimes []matching.TimeSlot
	expectedFirstSlotStayTimes = append(expectedFirstSlotStayTimes, matching.TimeSlot{Slot: POI.TimeInterval{8, 9}})
	expectedFirstSlotStayTimes = append(expectedFirstSlotStayTimes, matching.TimeSlot{Slot: POI.TimeInterval{9, 12}})
	expectedFirstSlotStayTimes = append(expectedFirstSlotStayTimes, matching.TimeSlot{Slot: POI.TimeInterval{12, 13}})
	expectedFirstSlotStayTimes = append(expectedFirstSlotStayTimes, matching.TimeSlot{Slot: POI.TimeInterval{13, 15}})

	for idx, expectedFirstSlotStayTime := range expectedFirstSlotStayTimes {
		if slotRequests[0].StayTimes[idx] != expectedFirstSlotStayTime {
			t.Errorf("expected stay times does not match. expected: %v, got: %v",
				expectedFirstSlotStayTime, slotRequests[0].StayTimes[idx])
		}
	}
}
