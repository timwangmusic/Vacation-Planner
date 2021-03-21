package test

import (
	"github.com/weihesdlegend/Vacation-planner/solution"
	"reflect"
	"testing"
)

func TestSolutionDiversityFilter(t *testing.T) {
	placeIds := []string{"33521", "33522", "33523", "33524"}

	// selected place ID combinations
	placeIDGroup1 := []string{placeIds[0], placeIds[1]}
	placeIDGroup2 := []string{placeIds[0], placeIds[2]}
	placeIDGroup3 := []string{placeIds[2], placeIds[3]}

	slotSolutionCandidate1 := solution.SlotSolutionCandidate{PlaceIDS: placeIDGroup1}
	slotSolutionCandidate2 := solution.SlotSolutionCandidate{PlaceIDS: placeIDGroup2}
	slotSolutionCandidate3 := solution.SlotSolutionCandidate{PlaceIDS: placeIDGroup3}

	m1 := solution.MultiSlotSolution{
		SlotSolutions: []solution.SlotSolutionCandidate{slotSolutionCandidate1, slotSolutionCandidate3},
		Score:         20.0,
	}

	m2 := solution.MultiSlotSolution{
		SlotSolutions: []solution.SlotSolutionCandidate{slotSolutionCandidate1, slotSolutionCandidate2},
		Score:         30.0,
	}

	m3 := solution.MultiSlotSolution{
		SlotSolutions: []solution.SlotSolutionCandidate{slotSolutionCandidate2, slotSolutionCandidate3},
		Score:         40.0,
	}

	multiSlotSolutions := []solution.MultiSlotSolution{m1, m2, m3}
	sortedMultiSlotSolutions := solution.SortMultiSlotSolutions(multiSlotSolutions, 2, 1, 1)
	if len(sortedMultiSlotSolutions) != 1 {
		t.Errorf("expected number of multi-slot solutions to be 1, got %d", len(sortedMultiSlotSolutions))
		return
	}
	for idx, slotSolution := range sortedMultiSlotSolutions[0].SlotSolutions {
		if !reflect.DeepEqual(m1.SlotSolutions[idx].PlaceIDS, slotSolution.PlaceIDS) {
			t.Errorf("place IDs do not match")
		}
	}
}
