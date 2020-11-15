package test

import (
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/matching"
	"testing"
)

func TestScoreFunction(t *testing.T) {
	// test regular non-zero-price place
	place1 := matching.CreatePlace(POI.Place{
		PriceLevel: 3,
		Rating:     50.0,
	}, POI.PlaceCategoryVisit)

	expectedScore := 1.0
	score := matching.Score([]matching.Place{place1})
	if score != expectedScore {
		t.Errorf("Expected score %f, got %f", expectedScore, score)
	}

	// test zero-price place
	place2 := matching.CreatePlace(POI.Place{
		PriceLevel: 0,
		Rating:     50.0,
	}, POI.PlaceCategoryVisit)

	expectedScore = 0.1
	score = matching.Score([]matching.Place{place2})
	if score != expectedScore {
		t.Errorf("Expected score %f, got %f", expectedScore, score)
	}

	// test multiple places with same location (NYC)
	places := []matching.Place{place1, place2}
	score = matching.Score(places)
	expectedScore = 0.55
	if score != expectedScore {
		t.Errorf("Expected score %f, got %f", expectedScore, score)
	}
}
