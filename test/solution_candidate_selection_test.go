package test

import (
	"strconv"
	"testing"

	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/matching"
	"github.com/weihesdlegend/Vacation-planner/solution"
)

// test if priority queue gives top solution candidates
func TestSolutionCandidateSelection(t *testing.T) {
	places := make([]matching.Place, 0)

	for i := 0; i < 100; i++ {
		places = append(places, matching.Place{Place: &POI.Place{ID: strconv.Itoa(i), Rating: float32(i), UserRatingsTotal: 9}, Price: 1})
	}

	iterator := &solution.MultiDimIterator{}

	clusters := make([][]matching.Place, 1)
	clusters[0] = places
	err := iterator.Init([]POI.PlaceCategory{POI.PlaceCategoryEatery}, clusters)
	if err != nil {
		t.Error(err)
		return
	}
	topSolutionsCount := int64(5)
	res := solution.FindBestPlanningSolutions(clusters, topSolutionsCount, iterator)

	if int64(len(res)) != topSolutionsCount {
		t.Errorf("expected number of solutions equals %d, got %d", topSolutionsCount, len(res))
		return
	}

	expectation := []float64{98, 97, 96, 95, 94}
	for idx, r := range res {
		if r.Score != expectation[idx] {
			t.Errorf("expected %f, got %f", expectation[idx], r.Score)
		}
	}
}
