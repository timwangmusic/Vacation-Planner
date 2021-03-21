package test

import (
	"github.com/weihesdlegend/Vacation-planner/solution"
	"testing"
)

func TestFindBestSolutions(t *testing.T) {
	solutionCandidates := make([]solution.MultiSlotSolution, 200)
	for idx := range solutionCandidates {
		solutionCandidates[idx] = solution.MultiSlotSolution{Score: float64(idx) + 1.0}
	}

	// test default
	numResults := uint64(solution.NumSolutions)
	bestSolutions := solution.SortMultiSlotSolutions(solutionCandidates, numResults, 1000, 1)

	if len(bestSolutions) != solution.NumSolutions {
		t.Errorf("Expected number of solutions %d, got %d", solution.NumSolutions, len(bestSolutions))
	}

	// test regular case
	expected := make(map[float64]bool)
	for score := 200; score > 190; score-- {
		expected[float64(score)] = true
	}

	numResults = uint64(10)
	bestSolutions = solution.SortMultiSlotSolutions(solutionCandidates, numResults, 1000, 1)

	if len(bestSolutions) != 10 {
		t.Errorf("Expected number of solutions %d, got %d", 10, len(bestSolutions))
	}

	for _, bestSolution := range bestSolutions {
		if _, exist := expected[bestSolution.Score]; !exist {
			t.Errorf("does not expect best solutions to contain solution with score %.2f", bestSolution.Score)
		}
	}

	// test extreme
	numResults = uint64(10000)
	bestSolutions = solution.SortMultiSlotSolutions(solutionCandidates, numResults, 1000, 1)

	if len(bestSolutions) != 200 {
		t.Errorf("Expected number of solutions %d, got %d", 200, len(bestSolutions))
	}
}
