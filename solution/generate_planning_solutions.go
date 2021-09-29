package solution

import (
	"container/heap"
	"context"
	"strconv"
	"strings"

	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/graph"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"github.com/weihesdlegend/Vacation-planner/matching"
	"github.com/yourbasic/radix"
)

const (
	TopSolutionsCountDefault              = 5
	DefaultPlaceSearchRadius              = 10000
	CategorizedPlaceIterInitFailureErrMsg = "categorized places iterator init failure"
)

func FindBestPlanningSolutions(candidates []PlanningSolution, topSolutionsCount int64) []PlanningSolution {
	if topSolutionsCount <= 0 {
		topSolutionsCount = TopSolutionsCountDefault
	}
	m := make(map[string]PlanningSolution) // map for result extraction
	vertexes := make([]graph.Vertex, len(candidates))
	for idx, candidate := range candidates {
		candidateKey := strconv.FormatInt(int64(idx), 10)
		vertex := graph.Vertex{Name: candidateKey, Key: candidate.Score}
		vertexes[idx] = vertex
		m[candidateKey] = candidate
	}
	// use limited-size minimum priority queue
	priorityQueue := &graph.MinPriorityQueueVertex{}
	for _, vertex := range vertexes {
		if priorityQueue.Len() == int(topSolutionsCount) {
			top := (*priorityQueue)[0]
			if vertex.Key > top.Key {
				heap.Pop(priorityQueue)
			} else {
				continue
			}
		}
		heap.Push(priorityQueue, vertex)
	}

	res := make([]PlanningSolution, 0)

	for priorityQueue.Len() > 0 {
		top := heap.Pop(priorityQueue).(graph.Vertex)
		res = append(res, m[top.Name])
	}

	return res
}

func GenerateSolutions(context context.Context, matcher matching.Matcher, redisClient iowrappers.RedisClient, redisReq iowrappers.PlanningSolutionsCacheRequest, request PlanningRequest) (solutions []PlanningSolution, slotSolutionRedisKey string, err error) {
	solutions = make([]PlanningSolution, 0)

	var placeClusters [][]matching.Place
	for _, slot := range request.Slots {
		var filterParams = make(map[matching.FilterCriteria]interface{})
		filterParams[matching.FilterByTimePeriod] = matching.TimeFilterParams{
			Category:     slot.Category,
			Day:          request.Weekday,
			TimeInterval: slot.TimeSlot.Slot,
		}
		places, err_ := matcher.Match(context, matching.Request{
			Radius:   DefaultPlaceSearchRadius,
			Location: request.Location,
			Criteria: matching.FilterByTimePeriod,
			Params:   filterParams,
		})
		if err_ != nil {
			iowrappers.Logger.Error(err)
			err = err_
			return
		}
		placeClusters = append(placeClusters, places)
	}

	placeCategories := ToSlotCategories(request.Slots)

	mdIter := MultiDimIterator{}
	if err = mdIter.Init(placeCategories, placeClusters); err != nil {
		return
	}

	for mdIter.HasNext() {
		curCandidate := CreateCandidate(mdIter, placeClusters)

		solutions = append(solutions, curCandidate)

		mdIter.Next()
	}

	solutions = TravelPlansDeduplication(solutions)

	bestCandidates := FindBestPlanningSolutions(solutions, request.NumPlans)
	solutions = bestCandidates

	// cache slot solution calculation results
	slotSolutionToCache := iowrappers.PlanningSolutionsCacheResponse{}
	slotSolutionToCache.CachedPlanningSolutions = make([]iowrappers.SlotSolutionCandidateCache, len(bestCandidates))

	for idx, slotSolutionCandidate := range bestCandidates {
		candidateCache := iowrappers.SlotSolutionCandidateCache{
			PlaceIds:        slotSolutionCandidate.PlaceIDS,
			Score:           slotSolutionCandidate.Score,
			PlaceNames:      slotSolutionCandidate.PlaceNames,
			PlaceLocations:  slotSolutionCandidate.PlaceLocations,
			PlaceAddresses:  slotSolutionCandidate.PlaceAddresses,
			PlaceURLs:       slotSolutionCandidate.PlaceURLs,
			PlaceCategories: slotSolutionCandidate.PlaceCategories,
		}
		slotSolutionToCache.CachedPlanningSolutions[idx] = candidateCache
	}

	redisClient.CachePlanningSolutions(context, redisReq, slotSolutionToCache)

	return
}

//TravelPlansDeduplication removes travel plans contain places that are permutations of each other
func TravelPlansDeduplication(travelPlans []PlanningSolution) []PlanningSolution {
	duplicatedPlans := make(map[string]bool)
	results := make([]PlanningSolution, 0)

	for _, travelPlan := range travelPlans {
		placeIds := travelPlan.PlaceIDS
		radix.Sort(placeIds)
		jointPlanIds := strings.Join(placeIds, "_")
		if _, exists := duplicatedPlans[jointPlanIds]; !exists {
			results = append(results, travelPlan)
			duplicatedPlans[jointPlanIds] = true
		}
	}
	return results
}

func NearbySearchWithPlaceView(context context.Context, matcher matching.Matcher, location POI.Location, weekday POI.Weekday, radius uint, timeSlot matching.TimeSlot, category POI.PlaceCategory) ([]matching.PlaceView, error) {
	var filterParams = make(map[matching.FilterCriteria]interface{})
	filterParams[matching.FilterByTimePeriod] = matching.TimeFilterParams{
		Category:     category,
		Day:          weekday,
		TimeInterval: timeSlot.Slot,
	}

	var placesView []matching.PlaceView

	places, err := matcher.Match(context, matching.Request{
		Radius:   radius,
		Location: location,
		Criteria: matching.FilterByTimePeriod,
		Params:   filterParams,
	})

	if err != nil {
		return placesView, err
	}

	for _, place := range places {
		placesView = append(placesView, matching.ToPlaceView(place))
	}
	return placesView, nil
}
