package test

import (
	"testing"

	"github.com/weihesdlegend/Vacation-planner/matching"
)

func TestMatchClusterCenters(t *testing.T) {
	eateryCenters := [][]float64{
		{37.773972, -122.431297},
		{32.715736, -117.161087},
		{40.712776, -74.005974},
	}
	visitCenters := [][]float64{
		{34.052235, -118.243683},
		{36.169941, -115.139832},
		{40.779079, -73.962578},
	}

	clusterPairs, _ := matching.MatchClusterCenters(eateryCenters, visitCenters)

	expected := [][]int{{0, 1}, {1, 0}, {2, 2}}

	for k, pair := range clusterPairs {
		if pair.EateryIdx != expected[k][0] || pair.VisitIdx != expected[k][1] {
			t.Errorf("Incorrect cluster matching result. Expected %T, got %T", expected[k], pair)
		}
	}
}
