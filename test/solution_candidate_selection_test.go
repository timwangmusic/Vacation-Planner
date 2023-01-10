package test

import (
	"context"
	"github.com/weihesdlegend/Vacation-planner/planner"
	"strconv"
	"testing"

	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/matching"
)

// test if priority queue gives top solution candidates
func TestSolutionCandidateSelection(t *testing.T) {
	places := make([]matching.Place, 0)

	for i := 0; i < 100; i++ {
		places = append(places, matching.Place{Place: &POI.Place{ID: strconv.Itoa(i), Rating: float32(i), UserRatingsTotal: 9}, Price: 1})
	}

	iterator := &planner.MultiDimIterator{}

	clusters := make([][]matching.Place, 1)
	clusters[0] = places
	err := iterator.Init([]POI.PlaceCategory{POI.PlaceCategoryEatery}, clusters)
	if err != nil {
		t.Fatal(err)
	}
	topSolutionsCount := 5
	res := planner.FindBestPlanningSolutions(context.Background(), clusters, topSolutionsCount, iterator)
	resp := <-res
	if err != nil {
		t.Fatal(err)
	}

	if len(resp.Solutions) != topSolutionsCount {
		t.Fatalf("expected number of solutions equals %d, got %d", topSolutionsCount, len(res))
	}

	expectation := []float64{98, 97, 96, 95, 94}
	for idx, r := range resp.Solutions {
		if r.Score != expectation[idx] {
			t.Errorf("expected %f, got %f", expectation[idx], r.Score)
		}
	}
}
