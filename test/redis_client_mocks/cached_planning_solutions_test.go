package redis_client_mocks

import (
	"github.com/go-playground/assert/v2"
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"testing"
)

func TestGetCachedPlanningSolutions(t *testing.T) {
	cacheRequest1 := &iowrappers.PlanningSolutionsCacheRequest{
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
		PlaceCategories: []POI.PlaceCategory{POI.PlaceCategoryVisit, POI.PlaceCategoryEatery},
	}
	expectedCacheResponse1 := &iowrappers.PlanningSolutionsResponse{}
	expectedCacheResponse1.PlanningSolutionRecords = make([]iowrappers.PlanningSolutionRecord, 1)
	expectedCacheResponse1.PlanningSolutionRecords[0].PlaceIDs = []string{"1", "2", "3"}
	expectedCacheResponse1.PlanningSolutionRecords[0].ID = "33521-12533"

	var err error
	err = RedisClient.SavePlanningSolutions(RedisContext, cacheRequest1, expectedCacheResponse1)
	if err != nil {
		t.Error(err)
		return
	}

	cacheRequest2 := &iowrappers.PlanningSolutionsCacheRequest{
		Location: POI.Location{City: "San Francisco", Country: "United States"},
	}
	expectedCacheResponse2 := &iowrappers.PlanningSolutionsResponse{}
	expectedCacheResponse2.PlanningSolutionRecords = make([]iowrappers.PlanningSolutionRecord, 1)
	expectedCacheResponse2.PlanningSolutionRecords[0].PlaceIDs = []string{"111", "222", "333"}
	expectedCacheResponse2.PlanningSolutionRecords[0].ID = "33522-22533"

	err = RedisClient.SavePlanningSolutions(RedisContext, cacheRequest2, expectedCacheResponse2)
	if err != nil {
		t.Error(err)
		return
	}

	cacheResponse, err := RedisClient.PlanningSolutions(RedisContext, cacheRequest1)

	if err != nil {
		t.Error(err)
		return
	}

	for idx, planningSolution := range cacheResponse.PlanningSolutionRecords {
		assert.Equal(t, expectedCacheResponse1.PlanningSolutionRecords[idx].PlaceIDs, planningSolution.PlaceIDs)
	}
}
