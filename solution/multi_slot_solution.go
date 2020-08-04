package solution

import (
	"container/heap"
	"errors"
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/graph"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"github.com/weihesdlegend/Vacation-planner/matching"
	"github.com/weihesdlegend/Vacation-planner/utils"
	"strconv"
	"strings"
)

const (
	NumSolutions             = 5
	TravelSpeed              = 50 // km/h
	TimeLimitBetweenClusters = 60 // minutes
)

// Solvers are used by planners to solve the planning problem
type Solver struct {
	matcher *matching.TimeMatcher
}

// mapping from status to standard http status codes
const (
	ValidSolutionFound           = 200
	InvalidSolverReqTimeInterval = 400
	InvalidRequestLocation       = 400
	ReqTimeSlotsTagMismatch      = 400
	ReqTagInvalid                = 400
	CatPlaceIterInitFailure      = 404
	NoValidSolution              = 404
)

func (solver *Solver) Init(poiSearcher *iowrappers.PoiSearcher) {
	solver.matcher = &matching.TimeMatcher{}
	solver.matcher.Init(poiSearcher)
}

func (solver *Solver) ValidateLocation(slotRequestLocation *string) bool {
	countryCity := strings.Split(*slotRequestLocation, ",")
	geoQuery := iowrappers.GeocodeQuery{
		City:    countryCity[0],
		Country: countryCity[1],
	}
	_, _, err := solver.matcher.PoiSearcher.Geocode(&geoQuery)
	if err != nil {
		return false
	}
	*slotRequestLocation = strings.Join([]string{geoQuery.City, geoQuery.Country}, ",")
	return true
}

func GenerateSlotSolutionRedisRequest(location string, evTag string, stayTimes []matching.TimeSlot, radius uint, weekday POI.Weekday) iowrappers.SlotSolutionCacheRequest {
	intervals := make([]POI.TimeInterval, len(stayTimes))
	for idx, stayTime := range stayTimes {
		intervals[idx] = stayTime.Slot
	}

	cityCountry := strings.Split(location, ",")
	evTags := make([]string, len(evTag))
	for idx, c := range evTag {
		evTags[idx] = string(c)
	}

	req := iowrappers.SlotSolutionCacheRequest{
		Country:   cityCountry[1],
		City:      cityCountry[0],
		Radius:    uint64(radius),
		EVTags:    evTags,
		Intervals: intervals,
		Weekday:   weekday,
	}
	return req
}

func (solver *Solver) Solve(req PlanningRequest, redisCli iowrappers.RedisClient) (resp PlanningResponse, err error) {
	if !travelTimeValidation(req) {
		err = errors.New("travel time limit exceeded for current selection")
		resp.Errcode = InvalidSolverReqTimeInterval
		return
	}

	// validate location with poiSearcher of the time matcher
	for idx := range req.SlotRequests {
		if !solver.ValidateLocation(&req.SlotRequests[idx].Location) {
			err = errors.New("invalid travel destination")
			resp.Errcode = InvalidRequestLocation
			return
		}
	}

	// set default number of planning results
	if req.NumResults == 0 {
		req.NumResults = NumSolutions
	}

	// each row contains candidates in one slot
	candidates := make([][]SlotSolutionCandidate, len(req.SlotRequests))
	for idx := range req.SlotRequests {
		candidates[idx] = make([]SlotSolutionCandidate, 0)
	}

	redisRequests := make([]iowrappers.SlotSolutionCacheRequest, len(req.SlotRequests))
	for idx, slotRequest := range req.SlotRequests {
		location, evTag, stayTimes := slotRequest.Location, slotRequest.EvOption, slotRequest.StayTimes
		redisRequests[idx] = GenerateSlotSolutionRedisRequest(location, evTag, stayTimes, req.SearchRadius, req.Weekday)
	}

	slotSolutionCacheResponses := redisCli.GetMultiSlotSolution(redisRequests)

	slotSolutionRedisKeys := make([]string, len(req.SlotRequests))
	for idx, slotRequest := range req.SlotRequests {
		solution := slotSolutionCacheResponses[idx]
		var slotSolution SlotSolution
		if solution.Err == nil {
			for _, candidate := range solution.SlotSolutionCandidate {
				slotSolutionCandidate := SlotSolutionCandidate{
					PlaceNames:      candidate.PlaceNames,
					PlaceIDS:        candidate.PlaceIds,
					PlaceLocations:  candidate.PlaceLocations,
					PlaceAddresses:  candidate.PlaceAddresses,
					PlaceURLs:       candidate.PlaceURLs,
					EndPlaceDefault: matching.Place{},
					Score:           candidate.Score,
					IsSet:           true,
				}
				slotSolution.SlotSolutionCandidates = append(slotSolution.SlotSolutionCandidates, slotSolutionCandidate)
			}
			candidates[idx] = append(candidates[idx], slotSolution.SlotSolutionCandidates...)
			continue
		}
		location, evTag, stayTimes := slotRequest.Location, slotRequest.EvOption, slotRequest.StayTimes
		slotSolution, slotSolutionRedisKey, err := GenerateSlotSolution(solver.matcher, location, evTag, stayTimes, req.SearchRadius, req.Weekday, redisCli, redisRequests[idx])
		// The candidates in each slot should satisfy the travel time constraints and inter-slot constraint
		if err != nil {
			if err.Error() == ReqTimeSlotsTagMismatchErrMsg {
				resp.Errcode = ReqTimeSlotsTagMismatch
			} else if err.Error() == CategorizedPlaceIterInitFailureErrMsg {
				resp.Errcode = CatPlaceIterInitFailure
			} else {
				resp.Errcode = ReqTagInvalid
			}
			return resp, err
		}
		candidates[idx] = append(candidates[idx], slotSolution.SlotSolutionCandidates...)
		slotSolutionRedisKeys[idx] = slotSolutionRedisKey
	}

	resp.Solutions = genBestMultiSlotSolutions(candidates, req.NumResults)
	if len(resp.Solutions) == 0 {
		invalidateSlotSolutionCache(&redisCli, slotSolutionRedisKeys)
	}
	return
}

func invalidateSlotSolutionCache(redisCli *iowrappers.RedisClient, slotSolutionRedisKeys []string) {
	redisCli.RemoveKeys(slotSolutionRedisKeys)
}

// return false if travel time between clusters exceed limit
// use upper-bound of the sum of radius plus distance between cluster centers
func travelTimeValidation(req PlanningRequest) bool {
	numTimeSlots := len(req.SlotRequests)

	for i := 0; i < numTimeSlots-1; i++ {
		prevRequest := req.SlotRequests[i]
		nextRequest := req.SlotRequests[i+1]
		if travelTime(prevRequest.Location, nextRequest.Location, req.SearchRadius, req.SearchRadius) > TimeLimitBetweenClusters {
			return false
		}
	}
	return true
}

func travelTime(fromLoc string, toLoc string, fromLocRadius uint, toLocRadius uint) uint {
	latLng1, _ := utils.ParseLocation(fromLoc)
	latLng2, _ := utils.ParseLocation(toLoc)

	distance := utils.HaversineDist(latLng1, latLng2) + float64(fromLocRadius+toLocRadius)

	return uint(distance / (TravelSpeed * 16.67)) // 16.67 is the ratio of m/minute and km/hour
}

func genBestMultiSlotSolutions(candidates [][]SlotSolutionCandidate, numResults uint64) []MultiSlotSolution {
	res := make([]MultiSlotSolution, 0)
	slotSolutionResults := make([][]SlotSolutionCandidate, 0)
	path := make([]SlotSolutionCandidate, 0)
	placeMap := make(map[string]bool)
	dfs(candidates, 0, path, &slotSolutionResults, placeMap)

	// after dfs, slot solution results are in the shape of number of multi-slot results by number of slots
	// i.e. each row is ready to fill one multi slot solution
	for _, result := range slotSolutionResults {
		multiSlotSolutionScore := totalScore(result)

		multiSlotSolution := MultiSlotSolution{
			Score:         multiSlotSolutionScore,
			SlotSolutions: result,
		}
		res = append(res, multiSlotSolution)
	}
	bestSolutions := FindBestSolutions(res, numResults)
	for solutionIdx := range bestSolutions {
		calTravelTime(&bestSolutions[solutionIdx])
	}
	return bestSolutions
}

func calTravelTime(solution *MultiSlotSolution) {
	numTimeSlots := len(solution.SlotSolutions)

	for slotIdx := 0; slotIdx < numTimeSlots-1; slotIdx++ {
		startPlace := solution.SlotSolutions[slotIdx].PlaceLocations[len(solution.SlotSolutions[slotIdx].PlaceLocations)-1]
		endPlace := solution.SlotSolutions[slotIdx].PlaceLocations[0]

		startLatLng, endLatLng := make([]float64, 2), make([]float64, 2)
		startLatLng[0], startLatLng[1] = startPlace[0], startPlace[1]
		endLatLng[0], endLatLng[1] = endPlace[0], endPlace[1]

		distance := utils.HaversineDist(startLatLng, endLatLng)
		intervalTime := uint(distance / (TravelSpeed * 16.67))

		solution.TravelTimes = append(solution.TravelTimes, intervalTime)
		solution.TotalTime += intervalTime
	}
}

func dfs(candidates [][]SlotSolutionCandidate, depth int, path []SlotSolutionCandidate,
	results *[][]SlotSolutionCandidate, placeMap map[string]bool) {
	if depth == len(candidates) {
		tmp := make([]SlotSolutionCandidate, depth)
		copy(tmp, path)
		*results = append(*results, tmp)
		return
	}

	for idx := 0; idx < len(candidates[depth]); idx++ {
		if !checkDuplication(placeMap, candidates[depth][idx]) {
			continue
		}
		path = append(path, candidates[depth][idx])
		dfs(candidates, depth+1, path, results, placeMap)
		path = path[:len(path)-1]
		removePlaceIds(placeMap, candidates[depth][idx])
	}
	return
}

func removePlaceIds(placesMap map[string]bool, slotSolutionCandidate SlotSolutionCandidate) {
	for _, placeId := range slotSolutionCandidate.PlaceIDS {
		placesMap[placeId] = false
	}
}

func checkDuplication(placesMap map[string]bool, slotSolutionCandidate SlotSolutionCandidate) bool {
	for _, placeId := range slotSolutionCandidate.PlaceIDS {
		if placesMap[placeId] {
			return false
		}
	}

	for _, placeId := range slotSolutionCandidate.PlaceIDS {
		placesMap[placeId] = true
	}

	return true
}

func totalScore(candidates []SlotSolutionCandidate) float64 {
	score := 0.0
	for _, candidate := range candidates {
		score += candidate.Score
	}
	return score
}

type MultiSlotSolution struct {
	SlotSolutions []SlotSolutionCandidate
	TravelTimes   []uint
	TotalTime     uint
	Score         float64
}

type PlanningRequest struct {
	SlotRequests []SlotRequest
	SearchRadius uint
	Weekday      POI.Weekday
	NumResults   uint64
}

type SlotRequest struct {
	Location  string              // city,country
	EvOption  string              // e.g. "EVV", "VEV"
	StayTimes []matching.TimeSlot // e.g. ["8AM-10AM", "10AM-11AM", "11AM-12PM"]
}

type PlanningResponse struct {
	Solutions []MultiSlotSolution
	Err       error
	Errcode   uint
}

// Find top multi-slot solutions
func FindBestSolutions(candidates []MultiSlotSolution, numResults uint64) []MultiSlotSolution {
	res := make([]MultiSlotSolution, 0)

	if numResults == 0 {
		return res
	}

	m := make(map[string]MultiSlotSolution) // map for result extraction
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
		if priorityQueue.Len() == int(numResults) {
			top := (*priorityQueue)[0]
			if vertex.Key > top.Key {
				heap.Pop(priorityQueue)
			} else {
				continue
			}
		}
		heap.Push(priorityQueue, vertex)
	}

	for priorityQueue.Len() > 0 {
		top := heap.Pop(priorityQueue).(graph.Vertex)
		res = append(res, m[top.Name])
	}

	return res
}

// Generate a standard request while we seek a better way to represent complex REST requests
func GetStandardRequest(weekday POI.Weekday, numResults uint64) (req PlanningRequest) {
	slot12 := matching.TimeSlot{Slot: POI.TimeInterval{Start: 9, End: 10}}
	slot13 := matching.TimeSlot{Slot: POI.TimeInterval{Start: 10, End: 12}}
	stayTimes1 := []matching.TimeSlot{slot12, slot13}
	slotReq1 := SlotRequest{
		Location:  "",
		EvOption:  "EV",
		StayTimes: stayTimes1,
	}
	slot21 := matching.TimeSlot{Slot: POI.TimeInterval{Start: 12, End: 13}}
	slot22 := matching.TimeSlot{Slot: POI.TimeInterval{Start: 13, End: 17}}
	stayTimes2 := []matching.TimeSlot{slot21, slot22}
	slotReq2 := SlotRequest{
		Location:  "",
		EvOption:  "EV",
		StayTimes: stayTimes2,
	}

	slot31 := matching.TimeSlot{Slot: POI.TimeInterval{Start: 18, End: 20}}
	stayTimes3 := []matching.TimeSlot{slot31}
	slotReq3 := SlotRequest{
		Location:  "",
		EvOption:  "E",
		StayTimes: stayTimes3,
	}

	req.SlotRequests = append(req.SlotRequests, []SlotRequest{slotReq1, slotReq2, slotReq3}...)
	req.Weekday = weekday
	req.NumResults = numResults
	return
}
