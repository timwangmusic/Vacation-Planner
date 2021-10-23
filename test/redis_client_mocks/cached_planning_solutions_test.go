package redis_client_mocks

import (
	"github.com/go-playground/assert/v2"
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"testing"
)

func TestGetCachedPlanningSolutions(t *testing.T) {
	cacheRequest1 := iowrappers.PlanningSolutionsCacheRequest{
		Location: POI.Location{City: "Beijing", Country: "China"},
	}
	cacheResponse1 := iowrappers.PlanningSolutionsCacheResponse{}
	cacheResponse1.CachedPlanningSolutions = make([]iowrappers.SlotSolutionCandidateCache, 1)
	cacheResponse1.CachedPlanningSolutions[0].PlaceIds = []string{"1", "2", "3"}

	RedisClient.CachePlanningSolutions(RedisContext, cacheRequest1, cacheResponse1)

	cacheRequest2 := iowrappers.PlanningSolutionsCacheRequest{
		Location: POI.Location{City: "San Francisco", Country: "United States"},
	}
	cacheResponse2 := iowrappers.PlanningSolutionsCacheResponse{}
	cacheResponse2.CachedPlanningSolutions = make([]iowrappers.SlotSolutionCandidateCache, 1)
	cacheResponse2.CachedPlanningSolutions[0].PlaceIds = []string{"11", "22", "33"}

	RedisClient.CachePlanningSolutions(RedisContext, cacheRequest2, cacheResponse2)

	expectedCacheResponse := []iowrappers.PlanningSolutionsCacheResponse{cacheResponse1, cacheResponse2}
	cacheResponse, err := RedisClient.PlanningSolutions(RedisContext, cacheRequest1)

	if err != nil {
		t.Error(err)
		return
	}

	for idx, planningSolution := range cacheResponse.CachedPlanningSolutions {
		assert.Equal(t, expectedCacheResponse[idx].CachedPlanningSolutions[0].PlaceIds, planningSolution.PlaceIds)
	}
}
