package test

import (
	"reflect"
	"testing"

	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/matching"
	"github.com/weihesdlegend/Vacation-planner/planner"
)

func TestFindOptimalPlan_shouldReturnOptimalPlaceIDs(t *testing.T) {
	s := &planner.Solver{}
	cluster1 := []matching.Place{
		{
			Price: 40,
			Place: &POI.Place{
				ID:               "1a",
				Rating:           4.0,
				UserRatingsTotal: 100,
			},
		},
		{
			Price: 35,
			Place: &POI.Place{
				ID:               "2b",
				Rating:           3.5,
				UserRatingsTotal: 1000,
			},
		},
		{
			Price: 45,
			Place: &POI.Place{
				ID:               "3c",
				Rating:           4.5,
				UserRatingsTotal: 10000,
			},
		},
	}

	cluster2 := []matching.Place{
		{
			Price: 10,
			Place: &POI.Place{
				ID:               "4d",
				Rating:           1.0,
				UserRatingsTotal: 100,
			},
		},
		{
			Price: 40,
			Place: &POI.Place{
				ID:               "5e",
				Rating:           4.0,
				UserRatingsTotal: 1000,
			},
		},
		{
			Price: 35,
			Place: &POI.Place{
				ID:               "2b",
				Rating:           3.5,
				UserRatingsTotal: 5000,
			},
		},
	}

	cluster3 := []matching.Place{
		{
			Price: 40,
			Place: &POI.Place{
				ID:               "1a",
				Rating:           4.0,
				UserRatingsTotal: 100,
			},
		},
	}

	clusters := [][]matching.Place{cluster1, cluster2, cluster3}

	result, err := s.FindOptimalPlan(clusters)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(result[:3], []string{"3c", "2b", "1a"}) {
		t.Errorf("expected result equals %+v, got %v", []string{"3c", "2b", "1a"}, result[:3])
	}
}
