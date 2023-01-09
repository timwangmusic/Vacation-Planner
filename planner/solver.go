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

func (s *Solver) Init(poiSearcher *iowrappers.PoiSearcher) {
	s.Searcher = poiSearcher
	s.TimeMatcher = matching.MatcherForTime{Searcher: poiSearcher}
	s.PriceRangeMatcher = matching.MatcherForPriceRange{Searcher: poiSearcher}
}

func (s *Solver) ValidateLocation(context context.Context, location *POI.Location) bool {
	geoQuery := iowrappers.GeocodeQuery{
		City:              location.City,
		AdminAreaLevelOne: location.AdminAreaLevelOne,
		Country:           location.Country,
	}
	_, _, err := s.Searcher.Geocode(context, &geoQuery)
	if err != nil {
		return false
	}
	location.City = geoQuery.City
	location.Country = geoQuery.Country
	return true
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

	redisRequest := toRedisRequest(req)

	cacheResponse, cacheErr := redisClient.PlanningSolutions(ctx, redisRequest)

	var resp PlanningResp
	if cacheErr != nil || len(cacheResponse.PlanningSolutionRecords) < req.NumPlans {
		resp = s.generateSolutions(ctx, req, s.TimeMatcher, s.PriceRangeMatcher)
		if resp.Err == nil {
			if err := saveSolutions(ctx, redisClient, req, &resp.Solutions); err != nil {
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

	priorityQueue := &graph.MinPriorityQueueVertex{}
	deduplicatedPlans := make(map[string]bool)
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
			if !isDuplicatedPlan(deduplicatedPlans, candidate) {
				continue
			}
			newVertex := graph.Vertex{Name: candidate.ID, Key: candidate.Score, Object: candidate}
			if priorityQueue.Len() == topSolutionsCount {
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
	}

	res := make([]PlanningSolution, 0)

	for priorityQueue.Len() > 0 {
		top := heap.Pop(priorityQueue).(graph.Vertex)
		res = append(res, top.Object.(PlanningSolution))
	}
	// min-heap, res needs to be reversed to get the descending order
	resp <- PlanningResp{Solutions: reversePlans(res)}
	return
}

func reversePlans(plans []PlanningSolution) []PlanningSolution {
	for i, j := 0, len(plans)-1; i < j; i, j = i+1, j-1 {
		plans[i], plans[j] = plans[j], plans[i]
	}
	return plans
}

func (s *Solver) generateSolutions(ctx context.Context, req *PlanningReq, timeMatcher matching.Matcher, priceRangeMatcher matching.Matcher) (resp PlanningResp) {
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
		})
		if err != nil {
			resp.ErrorCode = InternalError
			return resp
		}

		placesByTime, err := timeMatcher.Match(&matching.FilterRequest{
			Places:   places,
			Criteria: matching.FilterByTimePeriod,
			Params:   filterParams,
		})
		if err != nil {
			resp.ErrorCode = InternalError
			return resp
		}

		placesByPrice, err := priceRangeMatcher.Match(&matching.FilterRequest{
			Places:   placesByTime,
			Criteria: matching.FilterByPriceRange,
			Params:   filterParams,
		})
		if err != nil {
			resp.ErrorCode = InternalError
			return resp
		}

		iowrappers.Logger.Infof("number of places by price matcher is %d", len(placesByPrice))
		placeClusters = append(placeClusters, placesByPrice)
	}

	placeCategories := toPlaceCategories(req.Slots)

	mdIter := &MultiDimIterator{}
	if err := mdIter.Init(placeCategories, placeClusters); err != nil {
		resp.ErrorCode = NoValidSolution
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

func saveSolutions(ctx context.Context, c *iowrappers.RedisClient, req *PlanningReq, solutions *[]PlanningSolution) error {
	planningSolutionsResponse := &iowrappers.PlanningSolutionsResponse{}
	planningSolutionsResponse.PlanningSolutionRecords = make([]iowrappers.PlanningSolutionRecord, len(*solutions))

	for idx, candidate := range *solutions {
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
			Destination:     req.Location,
		}
		planningSolutionsResponse.PlanningSolutionRecords[idx] = record
	}

	err := c.SavePlanningSolutions(ctx, toRedisRequest(req), planningSolutionsResponse)
	if err != nil {
		return err
	}
	return nil
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
