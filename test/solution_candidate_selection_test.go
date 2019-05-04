package test

import (
	"Vacation-planner/solution"
	"testing"
)

// test if priority queue gives top solution candidates
func TestSolutionCandidateSelection (t *testing.T){
	candidates := make([]solution.SolutionCandidate, 0)

	for i := 0; i < 100; i++{
		candidates = append(candidates, solution.SolutionCandidate{Score: float64(i)})
	}

	res := solution.FindBestCandidates(candidates)

	expectation := []float64{95, 96, 97, 98, 99}
	for idx, r := range res{
		if r.Score != expectation[idx]{
			t.Errorf("expected %f, got %f", expectation[idx], r.Score)
		}
	}
}
