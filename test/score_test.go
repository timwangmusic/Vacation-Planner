package test

import (
	"math"
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
	}, POI.PlaceCategoryEatery),
	matching.CreatePlace(POI.Place{
		PriceLevel:       POI.PriceLevelZero,
		Rating:           3.0,
		UserRatingsTotal: 999,
	}, POI.PlaceCategoryVisit),
}

func TestScoreSinglePlacePlan(t *testing.T) {
	tests := []struct {
		place        []matching.Place
		expectScore  float64
		searchRadius int
	}{
		{[]matching.Place{testSinglePlaces[0]}, 0.2, 10000},
		{[]matching.Place{testSinglePlaces[1]}, 1.8, 10000},
	}

	for _, test := range tests {
		score := matching.Score(test.place, test.searchRadius)
		assert.Equal(t, test.expectScore, math.Ceil(score*100)/100)
	}
}
