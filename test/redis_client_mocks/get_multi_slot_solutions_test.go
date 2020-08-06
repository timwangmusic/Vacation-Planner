package redis_client_mocks

import (
	"github.com/go-playground/assert/v2"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"testing"
)

func TestGetMultiSlotSolutions(t *testing.T) {
	cacheRequest1 := iowrappers.SlotSolutionCacheRequest {
		City: "Beijing",
		Country: "China",
	}
	cacheResponse1 := iowrappers.SlotSolutionCacheResponse{}
	cacheResponse1.SlotSolutionCandidate = make([]iowrappers.SlotSolutionCandidateCache, 1)
	cacheResponse1.SlotSolutionCandidate[0].PlaceIds = []string{"1","2","3"}

	RedisClient.CacheSlotSolution(cacheRequest1, cacheResponse1)

	cacheRequest2 := iowrappers.SlotSolutionCacheRequest {
		City: "San Francisco",
		Country: "USA",
	}
	cacheResponse2 := iowrappers.SlotSolutionCacheResponse{}
	cacheResponse2.SlotSolutionCandidate = make([]iowrappers.SlotSolutionCandidateCache, 1)
	cacheResponse2.SlotSolutionCandidate[0].PlaceIds = []string{"11","22","33"}

	RedisClient.CacheSlotSolution(cacheRequest2, cacheResponse2)

	cacheResponses := []iowrappers.SlotSolutionCacheResponse{cacheResponse1, cacheResponse2}
	multiSlotSolutions := RedisClient.GetMultiSlotSolutions([]iowrappers.SlotSolutionCacheRequest{cacheRequest1, cacheRequest2})

	if len(multiSlotSolutions) != 2 {
		t.Errorf("expected to get 2 results from Redis, got %d", len(multiSlotSolutions))
	}

	for idx, slotSolution := range multiSlotSolutions {
		if slotSolution.Err != nil {
			t.Error(slotSolution.Err)
			return
		}
		assert.Equal(t, cacheResponses[idx].SlotSolutionCandidate[0].PlaceIds, slotSolution.SlotSolutionCandidate[0].PlaceIds)
	}
}
