package planner

import (
	"container/heap"
	"context"
	"errors"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/graph"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"github.com/weihesdlegend/Vacation-planner/matching"
	"github.com/yourbasic/radix"
	"strings"
)

const (
	NumPlansDefault                       = 5
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
	ScoreOld        float64             `json:"score_old"`
}

type Solver struct {
	Searcher          *iowrappers.PoiSearcher
	TimeMatcher       matching.Matcher
	PriceRangeMatcher matching.Matcher
}

const (
	ValidSolutionFound     = 200
	InvalidRequestLocation = 400
	NoValidSolution        = 404
	InternalError          = 500
)

type PlanningReq struct {
	Location        POI.Location  `json:"location"`
	Slots           []SlotRequest `json:"slots"`
	Weekday         POI.Weekday
	TravelDate      string
	NumPlans        int
	SearchRadius    uint
	PriceLevel      POI.PriceLevel
	PreciseLocation bool
}

type PlanningResp struct {
	Solutions []PlanningSolution
	Err       error
	ErrorCode int
}

// SlotRequest represents the properties of each row in the tabular travel plan, although not all of these are displayed to users
type SlotRequest struct {
	TimeSlot matching.TimeSlot `json:"time_slot"`
	Category POI.PlaceCategory `json:"category"`
}

func (solver *Solver) Init(poiSearcher *iowrappers.PoiSearcher) {
	solver.Searcher = poiSearcher
	solver.TimeMatcher = matching.MatcherForTime{Searcher: poiSearcher}
	solver.PriceRangeMatcher = matching.MatcherForPriceRange{Searcher: poiSearcher}
}

func (solver *Solver) ValidateLocation(context context.Context, location *POI.Location) bool {
	geoQuery := iowrappers.GeocodeQuery{
		City:              location.City,
		AdminAreaLevelOne: location.AdminAreaLevelOne,
		Country:           location.Country,
	}
	_, _, err := solver.Searcher.Geocode(context, &geoQuery)
	if err != nil {
		return false
	}
	location.City = geoQuery.City
	location.Country = geoQuery.Country
	return true
}

func PlanningSolutionsRedisRequest(location POI.Location, placeCategories []POI.PlaceCategory, stayTimes []matching.TimeSlot, radius uint, weekday POI.Weekday, priceLevel POI.PriceLevel) iowrappers.PlanningSolutionsCacheRequest {
	intervals := make([]POI.TimeInterval, len(stayTimes))
	for idx, stayTime := range stayTimes {
		intervals[idx] = stayTime.Slot
	}

	req := iowrappers.PlanningSolutionsCacheRequest{
		Location:        location,
		Radius:          uint64(radius),
		PriceLevel:      priceLevel,
		PlaceCategories: placeCategories,
		Intervals:       intervals,
		Weekday:         weekday,
	}
	return req
}

func (solver *Solver) Solve(context context.Context, redisClient *iowrappers.RedisClient, request *PlanningReq, response *PlanningResp) {
	iowrappers.Logger.Debugf("->Solve(context.Context, iowrappers.RedisClient, %v, *PlanningResp)", request)
	if !request.PreciseLocation && !solver.ValidateLocation(context, &request.Location) {
		response.Err = errors.New("invalid travel destination")
		response.ErrorCode = InvalidRequestLocation
		return
	}

	if request.PreciseLocation {
		geocode, err := solver.Searcher.ReverseGeocode(context, request.Location.Latitude, request.Location.Longitude)
		if err != nil {
			response.Err = err
			response.ErrorCode = InvalidRequestLocation
			return
		}
		request.Location.City = geocode.City
		request.Location.AdminAreaLevelOne = geocode.AdminAreaLevelOne
		request.Location.Country = geocode.Country
	}

	// set default planning results count
	if request.NumPlans == 0 {
		request.NumPlans = NumPlansDefault
	}

	redisRequest := PlanningSolutionsRedisRequest(request.Location, toPlaceCategories(request.Slots), toTimeSlots(request.Slots), request.SearchRadius, request.Weekday, request.PriceLevel)

	cacheResponse, cacheErr := redisClient.PlanningSolutions(context, redisRequest)

	if cacheErr != nil || len(cacheResponse.PlanningSolutionRecords) < request.NumPlans {
		solutions, slotSolutionRedisKey, err := GenerateSolutions(context, solver.TimeMatcher, redisClient, redisRequest, *request, solver.PriceRangeMatcher)
		if err != nil {
			response.Err = err
			if err.Error() == CategorizedPlaceIterInitFailureErrMsg || len(solutions) == 0 {
				response.ErrorCode = NoValidSolution
				invalidatePlanningSolutionsCache(context, redisClient, []string{slotSolutionRedisKey})
			} else {
				response.ErrorCode = InternalError
			}
			return
		}
		response.Solutions = solutions
		return
	}
	iowrappers.Logger.Debugf("[request_id: %s]Found planning solutions in Redis for request %+v.", context.Value(iowrappers.ContextRequestIdKey), *request)
	for idx, candidate := range cacheResponse.PlanningSolutionRecords {
		// deal with cases where there are more cached solutions than requested
		if idx >= request.NumPlans {
			break
		}
		planningSolution := PlanningSolution{
			ID:              candidate.ID,
			PlaceNames:      candidate.PlaceNames,
			PlaceIDS:        candidate.PlaceIDs,
			PlaceLocations:  candidate.PlaceLocations,
			PlaceAddresses:  candidate.PlaceAddresses,
			PlaceURLs:       candidate.PlaceURLs,
			PlaceCategories: candidate.PlaceCategories,
			Score:           candidate.Score,
			ScoreOld:        candidate.ScoreOld,
		}
		response.Solutions = append(response.Solutions, planningSolution)
	}
	iowrappers.Logger.Debugf("[request_id: %s]Retrieved %d cached plans from Redis for request %+v.", context.Value(iowrappers.ContextRequestIdKey), len(response.Solutions), *request)
}

func invalidatePlanningSolutionsCache(context context.Context, redisClient *iowrappers.RedisClient, slotSolutionRedisKeys []string) {
	if err := redisClient.RemoveKeys(context, slotSolutionRedisKeys); err != nil {
		iowrappers.Logger.Error(err)
	}
}

// GetStandardRequest generates a standard request while we seek a better way to represent complex REST requests
func GetStandardRequest(travelDate string, weekday POI.Weekday, numResults int, priceLevel POI.PriceLevel) (req PlanningReq) {
	timeSlot1 := matching.TimeSlot{Slot: POI.TimeInterval{Start: 10, End: 12}}
	slotReq1 := SlotRequest{
		TimeSlot: timeSlot1,
		Category: POI.PlaceCategoryVisit,
	}

	timeSlot2 := matching.TimeSlot{Slot: POI.TimeInterval{Start: 12, End: 13}}
	slotReq2 := SlotRequest{
		TimeSlot: timeSlot2,
		Category: POI.PlaceCategoryEatery,
	}

	timeSlot3 := matching.TimeSlot{Slot: POI.TimeInterval{Start: 13, End: 17}}

	slotReq3 := SlotRequest{
		TimeSlot: timeSlot3,
		Category: POI.PlaceCategoryVisit,
	}

	req.Slots = append(req.Slots, []SlotRequest{slotReq1, slotReq2, slotReq3}...)
	req.Weekday = weekday
	req.TravelDate = travelDate
	req.NumPlans = numResults
	req.PriceLevel = priceLevel
	return
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
	// TODO: replace default search radius with user search input
	res.Score = matching.Score(places, DefaultPlaceSearchRadius)
	res.ScoreOld = matching.ScoreOld(places)
	res.ID = uuid.NewString()
	return res, nil
}

func FindBestPlanningSolutions(placeClusters [][]matching.Place, topSolutionsCount int, iterator *MultiDimIterator) []PlanningSolution {
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
	// min-heap, res needs to be reversed to get the descending order
	return reversePlans(res)
}

func reversePlans(plans []PlanningSolution) []PlanningSolution {
	for i, j := 0, len(plans)-1; i < j; i, j = i+1, j-1 {
		plans[i], plans[j] = plans[j], plans[i]
	}
	return plans
}

func GenerateSolutions(context context.Context, timeMatcher matching.Matcher, redisClient *iowrappers.RedisClient, redisRequest iowrappers.PlanningSolutionsCacheRequest, request PlanningReq, priceRangeMatcher matching.Matcher) (solutions []PlanningSolution, solutionRedisKey string, err error) {
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

	placeCategories := toPlaceCategories(request.Slots)

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
			ScoreOld:        candidate.ScoreOld,
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
