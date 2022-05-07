package test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/matching"
)

var testSinglePlaces = []matching.Place{
	matching.CreatePlace(POI.Place{
		PriceLevel:       POI.PriceLevelThree,
		Rating:           5.0,
		UserRatingsTotal: 99,
	}, POI.PlaceCategoryVisit),
	matching.CreatePlace(POI.Place{
		PriceLevel:       POI.PriceLevelZero,
		Rating:           3.0,
		UserRatingsTotal: 0,
	}, POI.PlaceCategoryVisit),
}

func TestLegacyScoreSinglePlacePlan(t *testing.T) {
	// test regular non-zero-price place
	expectedScore := 0.2
	score := matching.ScoreOld([]matching.Place{testSinglePlaces[0]})
	if score != expectedScore {
		t.Errorf("Expected score %f, got %f", expectedScore, score)
		return
	}

	// test zero-price place
	expectedScore = 0.0
	score = matching.ScoreOld([]matching.Place{testSinglePlaces[1]})
	if score != expectedScore {
		t.Errorf("Expected score %f, got %f", expectedScore, score)
		return
	}
}

func TestScoreSinglePlacePlan(t *testing.T) {
	// place1 := matching.CreatePlace(POI.Place{
	// 	PriceLevel:       POI.PriceLevelThree,
	// 	Rating:           5.0,
	// 	UserRatingsTotal: 99,
	// }, POI.PlaceCategoryVisit)
	// place2 := matching.CreatePlace(POI.Place{
	// 	PriceLevel:       POI.PriceLevelZero,
	// 	Rating:           3.0,
	// 	UserRatingsTotal: 0,
	// }, POI.PlaceCategoryVisit)

	tests := []struct {
		place        []matching.Place
		expectScore  float64
		searchRadius int
	}{
		{[]matching.Place{testSinglePlaces[0]}, 0.2, 10000},
		{[]matching.Place{testSinglePlaces[1]}, 0.0, 1000},
	}

	for _, test := range tests {
		score := matching.Score(test.place, test.searchRadius)
		assert.Equal(t, score, test.expectScore)
	}
}
