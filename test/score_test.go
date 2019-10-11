package test

import (
	"Vacation-planner/matching"
	"math"
	"testing"
)

func TestScoreFunction(t *testing.T) {
	// test regular non-zero-price place
	place1 := matching.Place{Price: 40.0, Rating: 4.0, Location: [2]float64{40.730610, -73.935242}}

	score := matching.Score([]matching.Place{place1})
	if score != 0.1 {
		t.Errorf("Expected score %f, got %f", 0.1, score)
	}

	// test zero-price place
	place2 := matching.Place{Price: 0.0, Rating: 3.5, Location: [2]float64{40.730610, -73.935242}}

	score = matching.Score([]matching.Place{place2})
	expected := matching.AvgRating / matching.AvgPricing
	if score != expected {
		t.Errorf("Expected score %f, got %f", expected, score)
	}

	// test multiple places with same location (NYC)
	places := []matching.Place{place1, place2}
	score = matching.Score(places)
	expected = (matching.AvgRating / matching.AvgPricing + 0.1) / 2 / math.Max(matching.AvgRating / matching.AvgPricing, 0.1)
	if score != expected {
		t.Errorf("Expected score %f, got %f", expected, score)
	}
}
