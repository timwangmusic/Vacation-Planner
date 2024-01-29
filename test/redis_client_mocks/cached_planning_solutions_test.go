package redis_client_mocks

import (
	"github.com/go-playground/assert/v2"
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"testing"
)

func TestGetSavedPlanningSolutions_shouldReturnCorrectResults(t *testing.T) {
	request1 := &iowrappers.PlanningSolutionsSaveRequest{
		Location: POI.Location{City: "Beijing", Country: "China"},
		Intervals: []POI.TimeInterval{
			{
				Start: 8,
				End:   10,
			},
			{
				Start: 11,
				End:   13,
			},
		},
		Weekdays: []POI.Weekday{
			POI.DateWednesday,
			POI.DateFriday,
		},
		PlaceCategories: []POI.PlaceCategory{
			POI.PlaceCategoryVisit,
			POI.PlaceCategoryEatery,
		},
		PlanningSolutionRecords: []iowrappers.PlanningSolutionRecord{
			{
				ID:         "33521-12533",
				PlaceIDs:   []string{"1", "2"},
				Score:      100,
				PlaceNames: []string{"Tian Tan Park", "Yuan Ming Yuan"},
			},
		},
	}

	var err error
	err = RedisClient.SavePlanningSolutions(RedisContext, request1)
	if err != nil {
		t.Error(err)
		return
	}

	request2 := &iowrappers.PlanningSolutionsSaveRequest{
		Location: POI.Location{City: "San Francisco", Country: "United States"},
		PlanningSolutionRecords: []iowrappers.PlanningSolutionRecord{
			{
				ID:       "33522-22533",
				PlaceIDs: []string{"111", "222", "333"},
			},
		},
	}

	err = RedisClient.SavePlanningSolutions(RedisContext, request2)
	if err != nil {
		t.Error(err)
		return
	}

	planningSolutions, err := RedisClient.PlanningSolutions(RedisContext, request1)

	if err != nil {
		t.Error(err)
		return
	}

	for idx, planningSolution := range planningSolutions.PlanningSolutionRecords {
		record := request1.PlanningSolutionRecords[idx]
		assert.Equal(t, record.ID, planningSolution.ID)
		assert.Equal(t, record.PlaceIDs, planningSolution.PlaceIDs)
		assert.Equal(t, record.Score, planningSolution.Score)
		assert.Equal(t, record.PlaceNames, planningSolution.PlaceNames)
	}

	planningSolutions, err = RedisClient.PlanningSolutions(RedisContext, request2)
	if err != nil {
		t.Error(err)
		return
	}

	for idx, planningSolution := range planningSolutions.PlanningSolutionRecords {
		record := request2.PlanningSolutionRecords[idx]
		assert.Equal(t, record.ID, planningSolution.ID)
		assert.Equal(t, record.PlaceIDs, planningSolution.PlaceIDs)
	}
}
