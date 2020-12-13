package test

import (
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/matching"
	"testing"
)

func TestScoreFunction(t *testing.T) {
	// test regular non-zero-price place
	place1 := matching.CreatePlace(POI.Place{
		PriceLevel:       3,
		Rating:           5.0,
		UserRatingsTotal: 99,
	}, POI.PlaceCategoryVisit)

	expectedScore := 0.2
	score := matching.Score([]matching.Place{place1})
	if score != expectedScore {
		t.Errorf("Expected score %f, got %f", expectedScore, score)
		return
	}

	// test zero-price place
	place2 := matching.CreatePlace(POI.Place{
		PriceLevel:       0,
		Rating:           3.0,
		UserRatingsTotal: 0,
	}, POI.PlaceCategoryVisit)

	expectedScore = 0.0
	score = matching.Score([]matching.Place{place2})
	if score != expectedScore {
		t.Errorf("Expected score %f, got %f", expectedScore, score)
		return
	}
}
