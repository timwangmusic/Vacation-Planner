package solution

import (
	"context"
	"errors"
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

const (
	ValidSolutionFound      = 200
	InvalidRequestLocation  = 400
	NoValidSolution         = 404
	CatPlaceIterInitFailure = 500
	InternalError           = 500
)

type PlanningRequest struct {
	Location     POI.Location
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

func PlanningSolutionsRedisRequest(location POI.Location, placeCategories []POI.PlaceCategory, stayTimes []matching.TimeSlot, radius uint, weekday POI.Weekday) iowrappers.PlanningSolutionsCacheRequest {
	intervals := make([]POI.TimeInterval, len(stayTimes))
	for idx, stayTime := range stayTimes {
		intervals[idx] = stayTime.Slot
	}

	req := iowrappers.PlanningSolutionsCacheRequest{
		Location:        location,
		Radius:          uint64(radius),
		PlaceCategories: placeCategories,
		Intervals:       intervals,
		Weekday:         weekday,
	}
	return req
}

func (solver *Solver) Solve(context context.Context, redisClient iowrappers.RedisClient, request *PlanningRequest, response *PlanningResponse) {
	if !solver.ValidateLocation(context, &request.Location) {
		response.Err = errors.New("invalid travel destination")
		response.ErrorCode = InvalidRequestLocation
		return
	}

	// set default planning results count
	if request.NumPlans == 0 {
		request.NumPlans = NumPlansDefault
	}

	redisRequest := PlanningSolutionsRedisRequest(request.Location, ToPlaceCategories(request.Slots), ToTimeSlots(request.Slots), request.SearchRadius, request.Weekday)

	cacheResponse, cacheErr := redisClient.PlanningSolutions(context, redisRequest)

	if cacheErr != nil {
		iowrappers.Logger.Debugf("Solution cache miss for request %+v with error %s", *request, cacheErr.Error())
		solutions, slotSolutionRedisKey, err := GenerateSolutions(context, solver.TimeMatcher, redisClient, redisRequest, *request)
		if err != nil {
			response.Err = err
			if err.Error() == CategorizedPlaceIterInitFailureErrMsg {
				response.ErrorCode = CatPlaceIterInitFailure
			} else if len(solutions) == 0 {
				response.ErrorCode = NoValidSolution
				invalidatePlanningSolutionsCache(context, &redisClient, []string{slotSolutionRedisKey})
			} else {
				response.ErrorCode = InternalError
			}
			return
		}
		response.Solutions = solutions
		return
	}
	iowrappers.Logger.Debugf("Found planning solutions in Redis for request %+v.", *request)
	for _, candidate := range cacheResponse.PlanningSolutionRecords {
		planningSolution := PlanningSolution{
			ID:              candidate.ID,
			PlaceNames:      candidate.PlaceNames,
			PlaceIDS:        candidate.PlaceIDs,
			PlaceLocations:  candidate.PlaceLocations,
			PlaceAddresses:  candidate.PlaceAddresses,
			PlaceURLs:       candidate.PlaceURLs,
			PlaceCategories: candidate.PlaceCategories,
			Score:           candidate.Score,
		}
		response.Solutions = append(response.Solutions, planningSolution)
	}
	iowrappers.Logger.Debugf("Retrieved %d cached plans from Redis for request %+v.", len(response.Solutions), *request)
}

func invalidatePlanningSolutionsCache(context context.Context, redisClient *iowrappers.RedisClient, slotSolutionRedisKeys []string) {
	redisClient.RemoveKeys(context, slotSolutionRedisKeys)
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
