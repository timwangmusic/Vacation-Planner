package test

import (
	"github.com/weihesdlegend/Vacation-planner/solution"
	"testing"
)

// test if priority queue gives top solution candidates
func TestSolutionCandidateSelection(t *testing.T) {
	candidates := make([]solution.PlanningSolution, 0)

	for i := 0; i < 100; i++ {
		candidates = append(candidates, solution.PlanningSolution{Score: float64(i)})
	}

	res := solution.FindBestPlanningSolutions(candidates, 0)

	expectation := []float64{95, 96, 97, 98, 99}
	for idx, r := range res {
		if r.Score != expectation[idx] {
			t.Errorf("expected %f, got %f", expectation[idx], r.Score)
		}
	}
}
