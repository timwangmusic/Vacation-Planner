package redis_client_mocks

import (
	"context"
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
				Score:      200,
				PlaceNames: []string{"Tian Tan Park", "Yuan Ming Yuan"},
				Weekdays:   []string{"Wednesday", "Friday"},
				TimeSlots:  []string{"from 8 to 10", "from 11 to 13"},
			},
			{
				ID:         "33523-32533",
				PlaceIDs:   []string{"3", "2"},
				Score:      100,
				PlaceNames: []string{"Summer Palace", "Yuan Ming Yuan"},
				Weekdays:   []string{"Wednesday", "Friday"},
				TimeSlots:  []string{"from 8 to 10", "from 11 to 13"},
			},
		},
		NumPlans: 2,
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

	ctx := context.WithValue(RedisContext, iowrappers.ContextRequestUserId, "test_user")
	planningSolutions, err := RedisClient.PlanningSolutions(ctx, request1)

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
		assert.Equal(t, record.TimeSlots, planningSolution.TimeSlots)
		assert.Equal(t, record.Weekdays, planningSolution.Weekdays)
	}

	planningSolutions, err = RedisClient.PlanningSolutions(ctx, request2)
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
