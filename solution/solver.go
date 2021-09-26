package solution

import (
	"context"
	"errors"
	"strings"

	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"github.com/weihesdlegend/Vacation-planner/matching"
)

const (
	NumPlansDefault = 5
)

type Solver struct {
	Searcher    *iowrappers.PoiSearcher
	TimeMatcher matching.Matcher
}

// HTTP status codes
const (
	ValidSolutionFound      = 200
	InvalidRequestLocation  = 400
	ReqTagInvalid           = 400
	CatPlaceIterInitFailure = 404
	NoValidSolution         = 404
)

type PlanningRequest struct {
	Location     POI.Location // city,country
	Slots        []SlotRequest
	Weekday      POI.Weekday
	NumPlans     int64
	SearchRadius uint
}

type PlanningResponse struct {
	Solutions []PlanningSolution
	Err       error
	ErrorCode uint
}

//SlotRequest represents the properties of each row in the tabular travel plan, although not all of these are displayed to users
type SlotRequest struct {
	TimeSlot matching.TimeSlot
	Category POI.PlaceCategory
}

func (solver *Solver) Init(poiSearcher *iowrappers.PoiSearcher) {
	solver.Searcher = poiSearcher
	solver.TimeMatcher = matching.MatcherForTime{Searcher: poiSearcher}
}

func (solver *Solver) ValidateLocation(context context.Context, location *POI.Location) bool {
	geoQuery := iowrappers.GeocodeQuery{
		City:    location.City,
		Country: location.Country,
	}
	_, _, err := solver.Searcher.Geocode(context, &geoQuery)
	if err != nil {
		return false
	}
	location.City = geoQuery.City
	location.Country = geoQuery.Country
	return true
}

func GenerateSlotSolutionRedisRequest(location POI.Location, evTag string, stayTimes []matching.TimeSlot, radius uint, weekday POI.Weekday) iowrappers.SlotSolutionCacheRequest {
	intervals := make([]POI.TimeInterval, len(stayTimes))
	for idx, stayTime := range stayTimes {
		intervals[idx] = stayTime.Slot
	}

	evTags := make([]string, len(evTag))
	for idx, c := range evTag {
		evTags[idx] = string(c)
	}

	req := iowrappers.SlotSolutionCacheRequest{
		Country:   location.Country,
		City:      location.City,
		Radius:    uint64(radius),
		EVTags:    evTags,
		Intervals: intervals,
		Weekday:   weekday,
	}
	return req
}

func (solver *Solver) Solve(context context.Context, redisClient iowrappers.RedisClient, req *PlanningRequest, resp *PlanningResponse) {
	if !solver.ValidateLocation(context, &req.Location) {
		resp.Err = errors.New("invalid travel destination")
		resp.ErrorCode = InvalidRequestLocation
		return
	}

	// set default planning results count
	if req.NumPlans == 0 {
		req.NumPlans = NumPlansDefault
	}

	redisRequests := make([]iowrappers.SlotSolutionCacheRequest, 1)

	var sb strings.Builder
	for _, cat := range ToSlotCategories(req.Slots) {
		switch cat {
		case POI.PlaceCategoryEatery:
			sb.WriteString("e")
		case POI.PlaceCategoryVisit:
			sb.WriteString("v")
		}
	}
	redisRequests[0] = GenerateSlotSolutionRedisRequest(req.Location, sb.String(), ToTimeSlots(req.Slots), req.SearchRadius, req.Weekday)

	// TODO: Refactor Redis client to take single iowrappers.SlotSolutionCacheRequest
	slotSolutionCacheResponses := redisClient.GetMultiSlotSolutions(context, redisRequests)

	slotSolutionRedisKeys := make([]string, len(redisRequests))

	cacheResponse := slotSolutionCacheResponses[0]

	if cacheResponse.Err == nil {
		iowrappers.Logger.Debugf("Found slot cacheResponse in Redis.")
		for _, candidate := range cacheResponse.SlotSolutionCandidate {
			planningSolution := PlanningSolution{
				PlaceNames:     candidate.PlaceNames,
				PlaceIDS:       candidate.PlaceIds,
				PlaceLocations: candidate.PlaceLocations,
				PlaceAddresses: candidate.PlaceAddresses,
				PlaceURLs:      candidate.PlaceURLs,
				Score:          candidate.Score,
				IsSet:          true,
			}
			resp.Solutions = append(resp.Solutions, planningSolution)
		}
		iowrappers.Logger.Debugf("Retrieved %d cached plans from Redis.", len(resp.Solutions))
		return
	}

	iowrappers.Logger.Infof("Solution cache miss!")
	solutions, slotSolutionRedisKey, err := GenerateSolutions(context, solver.TimeMatcher, redisClient, redisRequests[0], *req)
	if err != nil {
		if err.Error() == CategorizedPlaceIterInitFailureErrMsg {
			resp.ErrorCode = CatPlaceIterInitFailure
		} else {
			resp.ErrorCode = ReqTagInvalid
		}
		return
	}
	resp.Solutions = solutions

	slotSolutionRedisKeys[0] = slotSolutionRedisKey

	if len(resp.Solutions) == 0 {
		invalidateSlotSolutionCache(context, &redisClient, slotSolutionRedisKeys)
	}
}

func invalidateSlotSolutionCache(context context.Context, redisCli *iowrappers.RedisClient, slotSolutionRedisKeys []string) {
	redisCli.RemoveKeys(context, slotSolutionRedisKeys)
}

// GetStandardRequest generates a standard request while we seek a better way to represent complex REST requests
func GetStandardRequest(weekday POI.Weekday, numResults int64) (req PlanningRequest) {
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
	req.NumPlans = numResults
	return
}
