package planner

import (
	"container/heap"
	"context"
	"errors"
	"slices"
	"sort"
	"strings"

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
	DefaultPlaceSearchRadius              = 10000
	CategorizedPlaceIterInitFailureErrMsg = "categorized places iterator init failure"
	ComputationTimedOutErrMsg             = "computation timed out"
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
	RequestTimeOut         = 408
	InternalError          = 500
)

type PlacePlanningDetails struct {
	Name     string            `json:"name"`
	URL      string            `json:"url"`
	Category string            `json:"category"`
	TimeSlot matching.TimeSlot `json:"time_slot"`
}

type PlanningReq struct {
	Location        POI.Location  `json:"location"`
	Slots           []SlotRequest `json:"slots"`
	Weekday         POI.Weekday   `json:"weekday"`
	TravelDate      string
	NumPlans        int
	SearchRadius    uint           `json:"radius"`
	PriceLevel      POI.PriceLevel `json:"price_level"`
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

func (s *Solver) Init(poiSearcher *iowrappers.PoiSearcher) {
	s.Searcher = poiSearcher
	s.TimeMatcher = matching.MatcherForTime{Searcher: poiSearcher}
	s.PriceRangeMatcher = matching.MatcherForPriceRange{Searcher: poiSearcher}
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

func (s *Solver) SolveHungarianOptimal(ctx context.Context, req *PlanningReq) ([]PlacePlanningDetails, error) {
	clusters, err := s.generatePlacesForSlots(ctx, req, s.TimeMatcher, s.PriceRangeMatcher)
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
		err := s.Searcher.GetRedisClient().FetchSingleRecord(ctx, "place_details:place_ID:"+id, &places[idx].Place)
		if err != nil {
			return nil, err
		}
		results[idx] = toPlacePlanningDetails(places[idx].Place.Name, req.Slots[idx], places[idx].Place.URL)
	}
	return results, nil
}

func (s *Solver) Solve(ctx context.Context, req *PlanningReq) *PlanningResp {
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

	var resp PlanningResp
	if cacheErr != nil || len(cacheResponse.PlanningSolutionRecords) < req.NumPlans {
		resp = s.generateSolutions(ctx, req, s.TimeMatcher, s.PriceRangeMatcher)
		if resp.Err == nil {
			if err := saveSolutions(ctx, redisClient, req, resp.Solutions); err != nil {
				logger.Error(err)
			}
		}
		return &resp
	}
	logger.Debugf("[request_id: %s]Found planning solutions in Redis for req %+v.", ctx.Value(iowrappers.ContextRequestIdKey), *req)
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
		}
		resp.Solutions = append(resp.Solutions, planningSolution)
	}
	logger.Debugf("[request_id: %s]Retrieved %d cached plans from Redis for req %+v.", ctx.Value(iowrappers.ContextRequestIdKey), len(resp.Solutions), *req)
	return &resp
}

// generates a request for normal template used by the regular search
func standardRequest(travelDate string, weekday POI.Weekday, numResults int, priceLevel POI.PriceLevel) (req PlanningReq) {
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
	// TODO: replace default search radius with user search input
	res.Score = matching.Score(places, DefaultPlaceSearchRadius)
	res.ScoreOld = matching.ScoreOld(places)
	res.ID = uuid.NewString()
	return res, nil
}

func FindBestPlanningSolutions(ctx context.Context, placeClusters [][]matching.Place, topSolutionsCount int, iterator *MultiDimIterator) (resp chan PlanningResp) {
	if topSolutionsCount <= 0 {
		topSolutionsCount = TopSolutionsCountDefault
	}

	priorityQueue := &MinPriorityQueue[Vertex]{}
	includedPlaces := make(map[string]bool)
	resp = make(chan PlanningResp, 1)

	for iterator.HasNext() {
		select {
		case <-ctx.Done():
			resp <- PlanningResp{Err: errors.New(ComputationTimedOutErrMsg), ErrorCode: RequestTimeOut}
			return
		default:
			var candidate PlanningSolution
			var err error
			candidate, err = createPlanningSolutionCandidate(iterator.Status, placeClusters)
			iterator.Next()
			if err != nil {
				log.Debug(err)
				continue
			}
			if isPlanDuplicate(includedPlaces, candidate) {
				continue
			}
			newVertex := Vertex{Name: candidate.ID, K: candidate.Score, Object: candidate}
			if priorityQueue.Len() == topSolutionsCount {
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

	res := make([]PlanningSolution, 0)

	for priorityQueue.Len() > 0 {
		top := heap.Pop(priorityQueue).(Vertex)
		res = append(res, top.Object.(PlanningSolution))
	}
	// outputs from min-heap needs to be reversed to get the descending order by score
	reversePlans(res)
	resp <- PlanningResp{Solutions: res}
	return
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

func (s *Solver) generatePlacesForSlots(ctx context.Context, req *PlanningReq, timeMatcher matching.Matcher, priceRangeMatcher matching.Matcher) ([][]matching.Place, error) {
	var placeClusters [][]matching.Place
	for _, slot := range req.Slots {
		var filterParams = make(map[matching.FilterCriteria]interface{})
		filterParams[matching.FilterByTimePeriod] = matching.TimeFilterParams{
			Category:     slot.Category,
			Day:          req.Weekday,
			TimeInterval: slot.TimeSlot.Slot,
		}

		filterParams[matching.FilterByPriceRange] = matching.PriceRangeFilterParams{
			Category:   slot.Category,
			PriceLevel: req.PriceLevel,
		}

		places, err := matching.NearbySearchForCategory(ctx, s.Searcher, &matching.Request{
			Radius:             DefaultPlaceSearchRadius,
			Location:           req.Location,
			Category:           slot.Category,
			UsePreciseLocation: req.PreciseLocation,
			PriceLevel:         req.PriceLevel,
		})
		if err != nil {
			return nil, err
		}

		placesByTime, err := timeMatcher.Match(&matching.FilterRequest{
			Places:   places,
			Criteria: matching.FilterByTimePeriod,
			Params:   filterParams,
		})
		if err != nil {
			return nil, err
		}

		placesByPrice, err := priceRangeMatcher.Match(&matching.FilterRequest{
			Places:   placesByTime,
			Criteria: matching.FilterByPriceRange,
			Params:   filterParams,
		})
		if err != nil {
			return nil, err
		}

		iowrappers.Logger.Debugf("Before filtering, the number of places is %d", len(places))
		iowrappers.Logger.Debugf("Filtering by time, the number of places is %d", len(placesByTime))
		iowrappers.Logger.Debugf("Filtering by price, the number of places is %d", len(placesByPrice))
		placeClusters = append(placeClusters, placesByPrice)
	}
	return placeClusters, nil
}

func (s *Solver) generateSolutions(ctx context.Context, req *PlanningReq, timeMatcher matching.Matcher, priceRangeMatcher matching.Matcher) (resp PlanningResp) {
	placeClusters, err := s.generatePlacesForSlots(ctx, req, timeMatcher, priceRangeMatcher)
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
		return resp
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, SolverTimeout)
	defer cancel()

	select {
	case <-ctxWithTimeout.Done():
		resp.Err = errors.New("cannot complete computation in time")
		resp.ErrorCode = RequestTimeOut
		return
	case r := <-FindBestPlanningSolutions(ctxWithTimeout, placeClusters, req.NumPlans, mdIter):
		return r
	}
}

func saveSolutions(ctx context.Context, c *iowrappers.RedisClient, req *PlanningReq, solutions []PlanningSolution) error {
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

// isPlanDuplicate checks if ANY place in the plan is seen in previous results.
func isPlanDuplicate(seenPlaces map[string]bool, newPlan PlanningSolution) bool {
	for _, placeID := range newPlan.PlaceIDS {
		if _, exists := seenPlaces[placeID]; exists {
			return true
		}
	}
	// mark all places in the plan as seen
	for _, placeID := range newPlan.PlaceIDS {
		seenPlaces[placeID] = true
	}
	return false
}

// removePlaces deletes all places of the input plan potentially stored in the input hashMap.
// This function can allow the main algorithm to re-introduce new plans into top candidates.
func removePlaces(seenPlaces map[string]bool, plan PlanningSolution) {
	for _, placeID := range plan.PlaceIDS {
		if _, exists := seenPlaces[placeID]; !exists {
			continue
		}
		delete(seenPlaces, placeID)
	}
}
