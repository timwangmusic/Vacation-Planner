package test

import (
	"github.com/weihesdlegend/Vacation-planner/solution"
	"reflect"
	"testing"
)

func TestTravelPlanDeduplication(t *testing.T) {
	travelPlanA := solution.PlanningSolution{
			PlaceIDS: []string{"a123x", "b456y", "c789z"},
		}

	travelPlanB := solution.PlanningSolution{
		PlaceIDS: []string{"b456y", "c789z", "a123x"},
	}

	travelPlans := []solution.PlanningSolution{travelPlanA, travelPlanB}

	filteredTravelPlans := solution.TravelPlansDeduplication(travelPlans)

	if len(filteredTravelPlans) != 1 {
		t.Errorf("Expected number of travel plans equals 1, got %d", len(filteredTravelPlans))
		return
	}

	if !reflect.DeepEqual(filteredTravelPlans[0].PlaceIDS, travelPlanA.PlaceIDS) {
		t.Errorf("Wrong place IDs, expected %v, got %v", travelPlanA.PlaceIDS, filteredTravelPlans[0].PlaceIDS)
	}
}
