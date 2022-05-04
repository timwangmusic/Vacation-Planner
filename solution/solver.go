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

type PlanningRequest struct {
	Location        POI.Location  `json:"location"`
	Slots           []SlotRequest `json:"slots"`
	Weekday         POI.Weekday
	TravelDate      string
	NumPlans        int64
	SearchRadius    uint
	PriceLevel      POI.PriceLevel
	PreciseLocation bool
}

type PlanningResponse struct {
	Solutions []PlanningSolution
	Err       error
	ErrorCode int
}

//SlotRequest represents the properties of each row in the tabular travel plan, although not all of these are displayed to users
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

func (solver *Solver) Solve(context context.Context, redisClient iowrappers.RedisClient, request *PlanningRequest, response *PlanningResponse) {
	iowrappers.Logger.Debugf("->Solve(context.Context, iowrappers.RedisClient, %v, *PlanningResponse)", request)
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

	redisRequest := PlanningSolutionsRedisRequest(request.Location, ToPlaceCategories(request.Slots), ToTimeSlots(request.Slots), request.SearchRadius, request.Weekday, request.PriceLevel)

	cacheResponse, cacheErr := redisClient.PlanningSolutions(context, redisRequest)

	if cacheErr != nil {
		iowrappers.Logger.Debugf("Solution cache miss for request %+v with error %s", *request, cacheErr.Error())
		solutions, slotSolutionRedisKey, err := GenerateSolutions(context, solver.TimeMatcher, redisClient, redisRequest, *request, solver.PriceRangeMatcher)
		if err != nil {
			response.Err = err
			if err.Error() == CategorizedPlaceIterInitFailureErrMsg || len(solutions) == 0 {
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
	iowrappers.Logger.Debugf("[request_id: %s]Found planning solutions in Redis for request %+v.", context.Value(iowrappers.ContextRequestIdKey), *request)
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
	iowrappers.Logger.Debugf("[request_id: %s]Retrieved %d cached plans from Redis for request %+v.", context.Value(iowrappers.ContextRequestIdKey), len(response.Solutions), *request)
}

func invalidatePlanningSolutionsCache(context context.Context, redisClient *iowrappers.RedisClient, slotSolutionRedisKeys []string) {
	if err := redisClient.RemoveKeys(context, slotSolutionRedisKeys); err != nil {
		iowrappers.Logger.Error(err)
	}
	return
}

// GetStandardRequest generates a standard request while we seek a better way to represent complex REST requests
func GetStandardRequest(travelDate string, weekday POI.Weekday, numResults int64, priceLevel POI.PriceLevel) (req PlanningRequest) {
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
