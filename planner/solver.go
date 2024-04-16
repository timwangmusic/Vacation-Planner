package planner

import (
	"cmp"
	"container/heap"
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/google/uuid"
	hungarianAlgorithm "github.com/oddg/hungarian-algorithm"
	log "github.com/sirupsen/logrus"
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"github.com/weihesdlegend/Vacation-planner/matching"
	"golang.org/x/exp/maps"
)

const (
	NumPlansDefault                       = 5
	TopSolutionsCountDefault              = 5
	DefaultPlaceSearchRadius              = 20000 // default to 20km (~12.43 miles)
	MaxSolutionsToSaveCount               = 100
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
	PlanSpec        string              `json:"plan_spec"`
}

func (ps PlanningSolution) Key() float64 {
	return ps.Score
}

type Solver struct {
	Searcher               *iowrappers.PoiSearcher
	placeMatcher           *PlaceMatcher
	concreteMatchers       []matching.Matcher
	placeDedupeCountLimit  int
	nearbyCitiesCountLimit int
}

const (
	ValidSolutionFound     = 200
	InvalidRequestLocation = 400
	NoValidSolution        = 404
	RequestTimeOut         = 408
	InternalError          = 500
)

type PlacePlanningDetails struct {
	Name     string            `json:"name"`
	URL      string            `json:"url"`
	Category string            `json:"category"`
	TimeSlot matching.TimeSlot `json:"time_slot"`
}

// MultiPlanningReq can be used to represent a multi-day planning request for a single location or a group of requests for different locations
type MultiPlanningReq struct {
	requests []*PlanningRequest
	numPlans int
}

type PlanningRequest struct {
	Location         POI.Location  `json:"location"`
	Slots            []SlotRequest `json:"slots"`
	TravelDate       string
	NumPlans         int
	SearchRadius     uint           `json:"radius"`
	PriceLevel       POI.PriceLevel `json:"price_level"`
	PreciseLocation  bool
	WithNearbyCities bool
	spec             string
}

type PlanningResp struct {
	Solutions []PlanningSolution
	Err       error
	ErrorCode int
}

// SlotRequest represents the properties of each row in the tabular travel plan, although not all of these are displayed to users
type SlotRequest struct {
	Weekday  POI.Weekday       `json:"weekday"`
	TimeSlot matching.TimeSlot `json:"time_slot"`
	Category POI.PlaceCategory `json:"category"`
}

func (s *Solver) Init(poiSearcher *iowrappers.PoiSearcher, placeDedupeCountLimit int, nearbyCitiesCountLimit int) {
	s.Searcher = poiSearcher
	s.placeDedupeCountLimit = placeDedupeCountLimit
	s.nearbyCitiesCountLimit = nearbyCitiesCountLimit
	s.concreteMatchers = make([]matching.Matcher, 0)
	s.concreteMatchers = append(s.concreteMatchers, &matching.MatcherForUserRatings{})
	s.concreteMatchers = append(s.concreteMatchers, &matching.MatcherForTime{})
	s.concreteMatchers = append(s.concreteMatchers, &matching.MatcherForPriceRange{})
	s.placeMatcher = NewPlaceMatcher()
}

func (s *Solver) ValidateLocation(ctx context.Context, location *POI.Location) bool {
	geoQuery := iowrappers.GeocodeQuery{
		City:              location.City,
		AdminAreaLevelOne: location.AdminAreaLevelOne,
		Country:           location.Country,
	}
	_, _, err := s.Searcher.Geocode(ctx, &geoQuery)
	if err != nil {
		return false
	}
	location.City = geoQuery.City
	location.Country = geoQuery.Country
	return true
}

func (s *Solver) SolveHungarianOptimal(ctx context.Context, req *PlanningRequest) ([]PlacePlanningDetails, error) {
	clusters, err := s.generatePlacesForSlots(ctx, req)
	if err != nil {
		return nil, err
	}

	placeIDs, err := s.FindOptimalPlan(clusters)
	if err != nil {
		return nil, err
	}

	places := make([]matching.Place, len(placeIDs))
	results := make([]PlacePlanningDetails, len(placeIDs))
	for idx, id := range placeIDs {
		err = s.Searcher.GetRedisClient().FetchSingleRecord(ctx, iowrappers.PlaceDetailsRedisKeyPrefix+id, &places[idx].Place)
		if err != nil {
			return nil, err
		}
		results[idx] = toPlacePlanningDetails(places[idx].Place.Name, req.Slots[idx], places[idx].Place.URL)
	}
	return results, nil
}

func (s *Solver) SolveWithNearbyCities(ctx context.Context, req *MultiPlanningReq) *PlanningResp {
	wg := sync.WaitGroup{}
	wg.Add(len(req.requests))
	responses := make(chan *PlanningResp)

	for _, request := range req.requests {
		go func(r *PlanningRequest) {
			defer wg.Done()
			responses <- s.Solve(ctx, r)
		}(request)
	}

	go func() {
		wg.Wait()
		close(responses)
	}()

	// int8 is enough for place deduplication limit
	includedPlaces := make(map[string]int8)

	pq := MinPriorityQueue[PlanningSolution]{}
	errs := make([]error, 0)
	for resp := range responses {
		if resp.Err != nil {
			errs = append(errs, resp.Err)
			continue
		}
		for _, solution := range resp.Solutions {
			if s.isPlanDuplicate(includedPlaces, solution) {
				continue
			}

			if pq.Len() >= req.numPlans {
				if pq.items[0].Key() < solution.Key() {
					top := pq.Pop().(PlanningSolution)
					removePlaces(includedPlaces, top)
					pq.Push(solution)
				} else {
					removePlaces(includedPlaces, solution)
				}
			} else {
				pq.Push(solution)
			}
		}
	}

	for _, err := range errs {
		iowrappers.Logger.Debugf("->SolveWithNearbyCities: encountered error: %v", err)
	}
	if pq.Len() > 0 {
		return &PlanningResp{Solutions: pq.items}
	}
	return &PlanningResp{Err: fmt.Errorf("cannot find solutions"), ErrorCode: NoValidSolution}
}

func (s *Solver) Solve(ctx context.Context, req *PlanningRequest) *PlanningResp {
	redisClient := s.Searcher.GetRedisClient()
	logger := iowrappers.Logger
	logger.Debugf("->Solve(ctx.Context, iowrappers.RedisClient, %v, *PlanningResp)", req)
	if !req.PreciseLocation && !s.ValidateLocation(ctx, &req.Location) {
		return &PlanningResp{Err: errors.New("invalid travel destination"), ErrorCode: InvalidRequestLocation}
	}

	if req.PreciseLocation {
		geocode, err := s.Searcher.ReverseGeocode(ctx, req.Location.Latitude, req.Location.Longitude)
		if err != nil {
			return &PlanningResp{Err: err, ErrorCode: InvalidRequestLocation}
		}
		req.Location.City = geocode.City
		req.Location.AdminAreaLevelOne = geocode.AdminAreaLevelOne
		req.Location.Country = geocode.Country
	}

	// set default planning results count
	if req.NumPlans == 0 {
		req.NumPlans = NumPlansDefault
	}

	cacheRequest := toSolutionsSaveRequest(req, nil)

	cacheResponse, cacheErr := redisClient.PlanningSolutions(ctx, cacheRequest)
	if cacheErr == nil {
		req.spec = cacheResponse.PlanningSpec
	}

	var resp = &PlanningResp{}
	if cacheErr != nil || len(cacheResponse.PlanningSolutionRecords) == 0 {
		resp = s.generateSolutions(ctx, req)
		if resp.Err == nil {
			if err := saveSolutions(ctx, redisClient, req, resp.Solutions); err != nil {
				logger.Error(err)
			}
		}
		resp.Solutions = resp.Solutions[:min(req.NumPlans, len(resp.Solutions))]
		return resp
	}

	logger.Debugf("[request_id: %s]Found %d planning solutions in Redis for req %+v.", ctx.Value(iowrappers.ContextRequestIdKey), len(cacheResponse.PlanningSolutionRecords), *req)
	for idx, candidate := range cacheResponse.PlanningSolutionRecords {
		// deal with cases where there are more saved solutions than requested
		if idx >= req.NumPlans {
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
			PlanSpec:        req.spec,
		}
		resp.Solutions = append(resp.Solutions, planningSolution)
	}
	logger.Debugf("[request_id: %s]Using %d cached plans from Redis for req %+v.", ctx.Value(iowrappers.ContextRequestIdKey), len(resp.Solutions), *req)
	return resp
}

// generates a request for normal template used by the regular search
func standardRequest(travelDate string, weekday POI.Weekday, numResults int, priceLevel POI.PriceLevel) (req PlanningRequest) {
	timeSlot1 := matching.TimeSlot{Slot: POI.TimeInterval{Start: 10, End: 12}}
	slotReq1 := SlotRequest{
		TimeSlot: timeSlot1,
		Category: POI.PlaceCategoryVisit,
		Weekday:  weekday,
	}

	timeSlot2 := matching.TimeSlot{Slot: POI.TimeInterval{Start: 12, End: 13}}
	slotReq2 := SlotRequest{
		TimeSlot: timeSlot2,
		Category: POI.PlaceCategoryEatery,
		Weekday:  weekday,
	}

	timeSlot3 := matching.TimeSlot{Slot: POI.TimeInterval{Start: 14, End: 17}}

	slotReq3 := SlotRequest{
		TimeSlot: timeSlot3,
		Category: POI.PlaceCategoryVisit,
		Weekday:  weekday,
	}

	timeSlot4 := matching.TimeSlot{Slot: POI.TimeInterval{Start: 18, End: 20}}
	slotReq4 := SlotRequest{
		TimeSlot: timeSlot4,
		Category: POI.PlaceCategoryEatery,
		Weekday:  weekday,
	}

	req.Slots = append(req.Slots, []SlotRequest{slotReq1, slotReq2, slotReq3, slotReq4}...)
	req.TravelDate = travelDate
	req.NumPlans = numResults
	req.PriceLevel = priceLevel
	return
}

func createPlanningSolutionCandidate(placeIndexes []int, placeClusters [][]matching.Place, radius uint, spec string) (PlanningSolution, error) {
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
		if _, exist := record[place.Id()]; exist {
			return res, errors.New(ErrMsgRepeatedPlaceInSameTrip)
		}

		record[place.Id()] = true
		places[idx] = place
		res.PlaceIDS = append(res.PlaceIDS, place.Id())
		res.PlaceNames = append(res.PlaceNames, place.Name())
		res.PlaceLocations = append(res.PlaceLocations, [2]float64{place.Location().Latitude, place.Location().Longitude})
		res.PlaceAddresses = append(res.PlaceAddresses, place.PlaceAddress())
		res.PlaceCategories = append(res.PlaceCategories, place.PlaceCategory())
		if len(strings.TrimSpace(place.Url())) == 0 {
			place.SetURL(iowrappers.GoogleSearchHomePageURL)
		}
		res.PlaceURLs = append(res.PlaceURLs, place.Url())
	}

	res.Score = matching.Score(places, int(radius))
	res.ScoreOld = matching.ScoreOld(places)
	res.ID = uuid.NewString()
	res.PlanSpec = spec
	return res, nil
}

func (s *Solver) FindBestPlanningSolutions(ctx context.Context, placeClusters [][]matching.Place, maxSolutionsToSaveCount int, iterator *MultiDimIterator, radius uint, spec string) (resp *PlanningResp) {
	if maxSolutionsToSaveCount <= 0 {
		maxSolutionsToSaveCount = TopSolutionsCountDefault
	}

	priorityQueue := &MinPriorityQueue[Vertex]{}
	// int8 is enough for place deduplication limit
	includedPlaces := make(map[string]int8)
	resp = &PlanningResp{}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, SolverTimeout)
	defer cancel()
	for iterator.HasNext() {
		select {
		case <-ctxWithTimeout.Done():
			iowrappers.Logger.Errorf("(Solver)FindBestPlanningSolutions -> computation timeout with iterator sizes %+v", iterator.Size)
			return &PlanningResp{Solutions: solutions(priorityQueue)}
		default:
			var candidate PlanningSolution
			var err error
			candidate, err = createPlanningSolutionCandidate(iterator.Status, placeClusters, radius, spec)
			iterator.Next()
			if err != nil {
				log.Debug(err)
				continue
			}
			if s.isPlanDuplicate(includedPlaces, candidate) {
				continue
			}
			newVertex := Vertex{Name: candidate.ID, K: candidate.Score, Object: candidate}
			if priorityQueue.Len() == maxSolutionsToSaveCount {
				topVertex := priorityQueue.items[0]
				if topVertex.Key() < newVertex.Key() {
					heap.Pop(priorityQueue)
					removePlaces(includedPlaces, topVertex.Object.(PlanningSolution))
					heap.Push(priorityQueue, newVertex)
				}
			} else {
				heap.Push(priorityQueue, newVertex)
			}
		}
	}

	res := solutions(priorityQueue)
	return &PlanningResp{Solutions: res}
}

func solutions(pq *MinPriorityQueue[Vertex]) []PlanningSolution {
	res := make([]PlanningSolution, 0)

	for pq.Len() > 0 {
		top := heap.Pop(pq).(Vertex)
		res = append(res, top.Object.(PlanningSolution))
	}
	// sort plans by score descending
	reversePlans(res)
	return res
}

func reversePlans(plans []PlanningSolution) {
	slices.Reverse(plans)
}

func (s *Solver) FindOptimalPlan(placeClusters [][]matching.Place) ([]string, error) {
	placeIds, weights, err := s.weightMatrix(placeClusters)
	if err != nil {
		return nil, err
	}
	solve, err := hungarianAlgorithm.Solve(fixWeights(weights))
	if err != nil {
		return nil, err
	}
	var result []string
	for _, idx := range solve[:len(placeClusters)] {
		result = append(result, placeIds[idx])
	}
	return result, nil
}

// We are solving a maximization problem instead of the default minimization problem Hungarian algorithm is designed for.
// Therefore, needs to use the maximum value to minus all the values in the matrix.
func fixWeights(weights [][]int) [][]int {
	if len(weights) == 0 {
		return nil
	}
	fixedWeights := make([][]int, len(weights))
	for idx := range fixedWeights {
		fixedWeights[idx] = make([]int, len(weights[0]))
	}
	// assume the values are non-negative
	var maxValue int
	for _, ws := range weights {
		for _, val := range ws {
			if val > maxValue {
				maxValue = val
			}
		}
	}

	for i, ws := range weights {
		for j, val := range ws {
			fixedWeights[i][j] = maxValue - val
		}
	}
	return fixedWeights
}

func (s *Solver) weightMatrix(placeClusters [][]matching.Place) ([]string, [][]int, error) {
	uniquePlaces := make(map[string]bool)
	for _, places := range placeClusters {
		for _, place := range places {
			uniquePlaces[place.Id()] = true
		}
	}

	placeIds := maps.Keys(uniquePlaces)
	sort.Strings(placeIds)

	// maps place ID to index
	placeIdsMap := make(map[string]int)
	for idx, id := range placeIds {
		placeIdsMap[id] = idx
	}

	weights := make([][]int, len(placeIds))

	// initialize weights to zero
	for idx := range weights {
		weights[idx] = make([]int, len(placeIds))
	}

	for idx, places := range placeClusters {
		for _, place := range places {
			weights[idx][placeIdsMap[place.Id()]] = int(100 * matching.Score([]matching.Place{place}, 1))
		}
	}

	return placeIds, weights, nil
}

type PlaceMatcher struct {
	m matching.Matcher
}

func NewPlaceMatcher() *PlaceMatcher {
	return &PlaceMatcher{}
}

func (pm *PlaceMatcher) setMatcher(matcher matching.Matcher) {
	pm.m = matcher
}

// MatchPlaces uses strategy pattern to match places at runtime
func (pm *PlaceMatcher) MatchPlaces(req *matching.FilterRequest, m matching.Matcher) ([]matching.Place, error) {
	pm.setMatcher(m)

	return pm.m.Match(req)
}

func (s *Solver) generatePlacesForSlots(ctx context.Context, req *PlanningRequest) ([][]matching.Place, error) {
	logger := iowrappers.Logger
	var placeClusters [][]matching.Place
	for _, slot := range req.Slots {
		var filterParams = make(map[matching.FilterCriteria]interface{})
		filterParams[matching.FilterByUserRating] = matching.UserRatingFilterParams{
			MinUserRatings: 1,
		}

		filterParams[matching.FilterByTimePeriod] = matching.TimeFilterParams{
			Day:          slot.Weekday,
			TimeInterval: slot.TimeSlot.Slot,
		}

		filterParams[matching.FilterByPriceRange] = matching.PriceRangeFilterParams{
			Category:   slot.Category,
			PriceLevel: req.PriceLevel,
		}

		places, err := matching.NearbySearchForCategory(ctx, s.Searcher, &matching.Request{
			Radius:             req.SearchRadius,
			Location:           req.Location,
			Category:           slot.Category,
			UsePreciseLocation: req.PreciseLocation,
			PriceLevel:         req.PriceLevel,
		})
		if err != nil {
			return nil, err
		}
		logger.Debugf("Before filtering, the number of places for category %s is %d", slot.Category, len(places))

		places, err = s.filterPlaces(places, filterParams, slot.Category)
		if err != nil {
			return nil, err
		}

		if len(places) == 0 {
			return nil, fmt.Errorf("failed to find any place for category %s at slot %s for location %+v", slot.Category, slot.TimeSlot.ToString(), req.Location)
		}
		// sort places by score descending so the solver checks places with higher score first
		slices.SortFunc(places, func(a, b matching.Place) int { return cmp.Compare(matching.PlaceScore(b), matching.PlaceScore(a)) })
		placeClusters = append(placeClusters, places)
	}
	return placeClusters, nil
}

func (s *Solver) filterPlaces(places []matching.Place, params map[matching.FilterCriteria]interface{}, c POI.PlaceCategory) ([]matching.Place, error) {
	logger := iowrappers.Logger
	var res = places
	var err error
	for _, m := range s.concreteMatchers {
		res, err = s.placeMatcher.MatchPlaces(&matching.FilterRequest{
			Places: res,
			Params: params,
		}, m)
		if err != nil {
			return nil, err
		}
		logger.Debugf("Filtered by %s, the number of places for category %s is %d", m.MatcherName(), c, len(res))
	}

	return res, nil
}

func (s *Solver) generateSolutions(ctx context.Context, req *PlanningRequest) (resp *PlanningResp) {
	resp = &PlanningResp{}
	placeClusters, err := s.generatePlacesForSlots(ctx, req)
	if err != nil {
		resp.ErrorCode = InternalError
		resp.Err = err
		return
	}

	placeCategories := toPlaceCategories(req.Slots)

	mdIter := &MultiDimIterator{}
	if err = mdIter.Init(placeCategories, placeClusters); err != nil {
		resp.ErrorCode = NoValidSolution
		resp.Err = err
		return
	}

	return s.FindBestPlanningSolutions(ctx, placeClusters, MaxSolutionsToSaveCount, mdIter, req.SearchRadius, req.spec)
}

func saveSolutions(ctx context.Context, c *iowrappers.RedisClient, req *PlanningRequest, solutions []PlanningSolution) error {
	planningSolutionRecords := make([]iowrappers.PlanningSolutionRecord, len(solutions))

	for idx, candidate := range solutions {
		record := toPlanningSolutionRecord(candidate, req.Location)
		planningSolutionRecords[idx] = record
	}

	err := c.SavePlanningSolutions(ctx, toSolutionsSaveRequest(req, planningSolutionRecords))
	if err != nil {
		return err
	}
	return nil
}

// isPlanDuplicate checks if ANY place in the plan appears in previous results more than Solver.placeDedupeCountLimit times.
func (s *Solver) isPlanDuplicate(includedPlaces map[string]int8, newPlan PlanningSolution) bool {
	for _, placeID := range newPlan.PlaceIDS {
		if count, exists := includedPlaces[placeID]; exists && (int(count) >= s.placeDedupeCountLimit) {
			return true
		}
	}
	// mark all places in the plan as seen
	for _, placeID := range newPlan.PlaceIDS {
		includedPlaces[placeID]++
	}
	return false
}

// removePlaces reduces counts of places of the input plan potentially stored in the input hashMap.
// This function can allow the main algorithm to re-introduce new plans into top candidates.
func removePlaces(includedPlaces map[string]int8, plan PlanningSolution) {
	for _, placeID := range plan.PlaceIDS {
		count, exists := includedPlaces[placeID]
		if !exists || (count == 0) {
			continue
		}
		includedPlaces[placeID]--
	}
}
