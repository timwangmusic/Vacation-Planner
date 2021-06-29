package solution

import (
	"container/heap"
	"context"
	"errors"
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/graph"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"github.com/weihesdlegend/Vacation-planner/matching"
	"github.com/yourbasic/radix"
	"strconv"
	"strings"
)

const (
	TopSolutionsCountDefault              = 5
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

func generateCategorizedPlaces(context context.Context, timeMatcher *matching.TimeMatcher, location string, radius uint, weekday POI.Weekday, timeSlots []matching.TimeSlot) ([]CategorizedPlaces, int) {
	matchingRequest := &matching.TimeMatchingRequest{
		Location:  location,
		Radius:    radius,
		TimeSlots: timeSlots,
		Weekday:   weekday,
	}
	timePlaceClusters := timeMatcher.Matching(context, matchingRequest)
	// one set of categorized places for each TimePlaceCluster
	categorizedPlaces := make([]CategorizedPlaces, len(timePlaceClusters))

	// place clusters are clustered by time slot
	// now cluster by place category
	for idx, timePlaceCluster := range timePlaceClusters {
		categorizedPlaces[idx] = Categorize(timePlaceCluster)
	}
	return categorizedPlaces, GetTimeSlotLengthInMin(timePlaceClusters)
}

// GenerateSolutions generates multi-slot solutions and cache them
func GenerateSolutions(context context.Context, timeMatcher *matching.TimeMatcher, redisClient iowrappers.RedisClient, redisReq iowrappers.SlotSolutionCacheRequest, request PlanningRequest) (solutions []PlanningSolution, slotSolutionRedisKey string, err error) {
	solutions = make([]PlanningSolution, 0)

	categorizedPlaces, _ := generateCategorizedPlaces(context, timeMatcher, request.Location, request.SearchRadius, request.Weekday, ToTimeSlots(request.Slots))

	placeCategories := ToSlotCategories(request.Slots)
	mdIter := MultiDimIterator{}
	if err = mdIter.Init(placeCategories, categorizedPlaces); err != nil {
		return
	}

	for mdIter.HasNext() {
		curCandidate := CreateCandidate(placeCategories, mdIter, categorizedPlaces)

		if curCandidate.IsSet {
			solutions = append(solutions, curCandidate)
		}
		mdIter.Next()
	}

	solutions = TravelPlansDeduplication(solutions)

	bestCandidates := FindBestPlanningSolutions(solutions, request.NumPlans)
	solutions = bestCandidates

	// cache slot solution calculation results
	slotSolutionToCache := iowrappers.SlotSolutionCacheResponse{}
	slotSolutionToCache.SlotSolutionCandidate = make([]iowrappers.SlotSolutionCandidateCache, len(bestCandidates))

	for idx, slotSolutionCandidate := range bestCandidates {
		candidateCache := iowrappers.SlotSolutionCandidateCache{
			PlaceIds:       slotSolutionCandidate.PlaceIDS,
			Score:          slotSolutionCandidate.Score,
			PlaceNames:     slotSolutionCandidate.PlaceNames,
			PlaceLocations: slotSolutionCandidate.PlaceLocations,
			PlaceAddresses: slotSolutionCandidate.PlaceAddresses,
			PlaceURLs:      slotSolutionCandidate.PlaceURLs,
		}
		slotSolutionToCache.SlotSolutionCandidate[idx] = candidateCache
	}

	redisClient.CacheSlotSolution(context, redisReq, slotSolutionToCache)

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

// NearbySearchWithPlaceView returns PlaceView results for single day nearby search with a fixed time slot range
func NearbySearchWithPlaceView(context context.Context, timeMatcher *matching.TimeMatcher, location string,
	weekday POI.Weekday, radius uint, timeSlot matching.TimeSlot, category POI.PlaceCategory) ([]matching.PlaceView, error) {
	timeSlots := []matching.TimeSlot{timeSlot}
	categorizedPlaces, _ := generateCategorizedPlaces(context, timeMatcher, location, radius, weekday, timeSlots)
	if len(categorizedPlaces) != 1 {
		return nil, errors.New("we should only get one set of categorized places")
	}

	var places []matching.Place
	switch category {
	case POI.PlaceCategoryEatery:
		places = categorizedPlaces[0].EateryPlaces
	case POI.PlaceCategoryVisit:
		places = categorizedPlaces[0].VisitPlaces
	}

	var placesView = make([]matching.PlaceView, len(places))
	for idx, place := range places {
		placesView[idx] = matching.ToPlaceView(place)
	}
	return placesView, nil
}
