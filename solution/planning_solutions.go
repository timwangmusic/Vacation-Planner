package solution

import (
	"container/heap"
	"context"
	"errors"
	log "github.com/sirupsen/logrus"
	"strings"

	"github.com/google/uuid"
	"github.com/yourbasic/radix"

	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/graph"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"github.com/weihesdlegend/Vacation-planner/matching"
)

const (
	TopSolutionsCountDefault              = 5
	DefaultPlaceSearchRadius              = 10000
	CategorizedPlaceIterInitFailureErrMsg = "categorized places iterator init failure"
	ErrMsgMismatchIterAndPlace            = "mismatch in iterator status vector length"
	ErrMsgRepeatedPlaceInSameTrip         = "repeated places in the same trip"
)

type PlanningSolution struct {
	ID              string              `json:"id"`
	PlaceNames      []string            `json:"place_names"`
	PlaceIDS        []string            `json:"place_ids"`
	PlaceLocations  [][2]float64        `json:"place_locations"` // lat,lng
	PlaceAddresses  []string            `json:"place_addresses"`
	PlaceURLs       []string            `json:"place_urls"`
	PlaceCategories []POI.PlaceCategory `json:"place_categories"`
	Score           float64             `json:"score"`
}

func createPlanningSolutionCandidate(placeIndexes []int, placeClusters [][]matching.Place) (PlanningSolution, error) {
	var res PlanningSolution
	if len(placeIndexes) != len(placeClusters) {
		return res, errors.New(ErrMsgMismatchIterAndPlace)
	}
	// deduplication of repeating places in the result

	record := make(map[string]bool)
	places := make([]matching.Place, len(placeIndexes))
	for idx, placeIdx := range placeIndexes {
		placesByCategory := placeClusters[idx]

		if placeIdx >= len(placesByCategory) {
			return res, errors.New("place index out of bound")
		}
		var place = placesByCategory[placeIdx]

		// if the same place appears in two indexes, return incomplete result
		if _, exist := record[place.GetPlaceId()]; exist {
			return res, errors.New(ErrMsgRepeatedPlaceInSameTrip)
		}

		record[place.GetPlaceId()] = true
		places[idx] = place
		res.PlaceIDS = append(res.PlaceIDS, place.GetPlaceId())
		res.PlaceNames = append(res.PlaceNames, place.GetPlaceName())
		res.PlaceLocations = append(res.PlaceLocations, [2]float64{place.GetLocation().Latitude, place.GetLocation().Longitude})
		res.PlaceAddresses = append(res.PlaceAddresses, place.GetPlaceFormattedAddress())
		res.PlaceCategories = append(res.PlaceCategories, place.GetPlaceCategory())
		if len(strings.TrimSpace(place.GetURL())) == 0 {
			place.SetURL(iowrappers.GoogleSearchHomePageURL)
		}
		res.PlaceURLs = append(res.PlaceURLs, place.GetURL())
	}
	res.Score = matching.Score(places)
	res.ID = uuid.NewString()
	return res, nil
}

func FindBestPlanningSolutions(placeClusters [][]matching.Place, topSolutionsCount int64, iterator *MultiDimIterator) []PlanningSolution {
	if topSolutionsCount <= 0 {
		topSolutionsCount = TopSolutionsCountDefault
	}

	priorityQueue := &graph.MinPriorityQueueVertex{}
	deduplicatedPlans := make(map[string]bool)

	for iterator.HasNext() {
		var candidate PlanningSolution
		var err error
		candidate, err = createPlanningSolutionCandidate(iterator.Status, placeClusters)
		iterator.Next()
		if err != nil {
			log.Debug(err)
			continue
		}
		if !isDuplicatedPlan(deduplicatedPlans, candidate) {
			continue
		}
		newVertex := graph.Vertex{Name: candidate.ID, Key: candidate.Score, Object: candidate}
		if priorityQueue.Len() == int(topSolutionsCount) {
			topVertex := (*priorityQueue)[0]
			if topVertex.Key < newVertex.Key {
				heap.Pop(priorityQueue)
				delete(deduplicatedPlans, jointPlaceIdsForPlan(topVertex.Object.(PlanningSolution)))
				heap.Push(priorityQueue, newVertex)
			}
		} else {
			heap.Push(priorityQueue, newVertex)
		}
	}

	res := make([]PlanningSolution, 0)

	for priorityQueue.Len() > 0 {
		top := heap.Pop(priorityQueue).(graph.Vertex)
		res = append(res, top.Object.(PlanningSolution))
	}

	return res
}

func GenerateSolutions(context context.Context, timeMatcher matching.Matcher, redisClient iowrappers.RedisClient, redisRequest iowrappers.PlanningSolutionsCacheRequest, request PlanningRequest, priceRangeMatcher matching.Matcher) (solutions []PlanningSolution, solutionRedisKey string, err error) {
	solutions = make([]PlanningSolution, 0)

	var placeClusters [][]matching.Place
	for _, slot := range request.Slots {
		var filterParams = make(map[matching.FilterCriteria]interface{})
		filterParams[matching.FilterByTimePeriod] = matching.TimeFilterParams{
			Category:     slot.Category,
			Day:          request.Weekday,
			TimeInterval: slot.TimeSlot.Slot,
		}

		filterParams[matching.FilterByPriceRange] = matching.PriceRangeFilterParams{
			Category:   slot.Category,
			PriceLevel: request.PriceLevel,
		}

		placesByTime, err_ := timeMatcher.Match(context, matching.Request{
			Radius:             DefaultPlaceSearchRadius,
			Location:           request.Location,
			Criteria:           matching.FilterByTimePeriod,
			Params:             filterParams,
			UsePreciseLocation: request.PreciseLocation,
		})
		if err_ != nil {
			iowrappers.Logger.Error(err_)
			err = err_
			return
		}

		placesByPrice, err_ := priceRangeMatcher.Match(context, matching.Request{
			Radius:             DefaultPlaceSearchRadius,
			Location:           request.Location,
			Criteria:           matching.FilterByPriceRange,
			Params:             filterParams,
			UsePreciseLocation: request.PreciseLocation,
		})
		if err_ != nil {
			iowrappers.Logger.Error(err_)
			err = err_
			return
		}

		iowrappers.Logger.Infof("number of places by price matcher is %d", len(placesByPrice))
		placeClusters = append(placeClusters, mergePlaceClusters(placesByTime, placesByPrice))
	}

	placeCategories := ToPlaceCategories(request.Slots)

	mdIter := MultiDimIterator{}
	if err = mdIter.Init(placeCategories, placeClusters); err != nil {
		return
	}

	bestCandidates := FindBestPlanningSolutions(placeClusters, request.NumPlans, &mdIter)
	solutions = bestCandidates

	// cache slot solution calculation results
	planningSolutionsResponse := iowrappers.PlanningSolutionsResponse{}
	planningSolutionsResponse.PlanningSolutionRecords = make([]iowrappers.PlanningSolutionRecord, len(bestCandidates))

	for idx, candidate := range bestCandidates {
		record := iowrappers.PlanningSolutionRecord{
			ID:              candidate.ID,
			PlaceIDs:        candidate.PlaceIDS,
			Score:           candidate.Score,
			PlaceNames:      candidate.PlaceNames,
			PlaceLocations:  candidate.PlaceLocations,
			PlaceAddresses:  candidate.PlaceAddresses,
			PlaceURLs:       candidate.PlaceURLs,
			PlaceCategories: candidate.PlaceCategories,
			Destination:     request.Location,
		}
		planningSolutionsResponse.PlanningSolutionRecords[idx] = record
		solutions[idx].ID = record.ID
	}

	redisKey, saveSolutionsErr := redisClient.SavePlanningSolutions(context, redisRequest, planningSolutionsResponse)
	if saveSolutionsErr != nil {
		return solutions, redisKey, saveSolutionsErr
	}

	return
}

// returns true if the new travel plan can be added to priority queue
func isDuplicatedPlan(plans map[string]bool, newPlan PlanningSolution) bool {
	jointPlaceIDs := jointPlaceIdsForPlan(newPlan)
	if _, exists := plans[jointPlaceIDs]; !exists {
		plans[jointPlaceIDs] = true
		return true
	}
	return false
}

func jointPlaceIdsForPlan(newPlan PlanningSolution) string {
	placeIDs := make([]string, len(newPlan.PlaceIDS))
	copy(placeIDs, newPlan.PlaceIDS)
	radix.Sort(placeIDs)
	jointPlaceIDs := strings.Join(placeIDs, "_")
	return jointPlaceIDs
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

// selects mutual places
func mergePlaceClusters(placesA []matching.Place, placesB []matching.Place) []matching.Place {
	var results []matching.Place
	placeMap := make(map[string]bool)
	for _, place := range placesA {
		placeMap[place.GetPlaceId()] = true
	}

	for _, place := range placesB {
		if _, ok := placeMap[place.GetPlaceId()]; ok {
			results = append(results, place)
		}
	}
	return results
}
