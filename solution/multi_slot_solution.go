package solution

import (
	"Vacation-planner/POI"
	"Vacation-planner/graph"
	"Vacation-planner/iowrappers"
	"Vacation-planner/matching"
	"Vacation-planner/utils"
	"errors"
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
	NoValidSolution              = 404
)

func (solver *Solver) Init(apiKey string, dbName string, dbUrl string, redisAddr string, redisPsw string, redisIdx int) {
	solver.matcher = &matching.TimeMatcher{}
	poiSearcher := &iowrappers.PoiSearcher{}
	mapsClient := &iowrappers.MapsClient{}
	utils.CheckErrImmediate(mapsClient.Create(apiKey), utils.LogFatal)
	poiSearcher.Init(mapsClient, dbName, dbUrl, redisAddr, redisPsw, redisIdx)
	solver.matcher.Init(poiSearcher)
}

func (solver *Solver) ValidateLocation(location string) bool {
	countryCity := strings.Split(location, ",")
	_, _, err := solver.matcher.PoiSearcher.Geocode(iowrappers.GeocodeQuery{
		City:    countryCity[0],
		Country: countryCity[1],
	})
	if err != nil {
		return false
	}
	return true
}

func (solver *Solver) Solve(req PlanningRequest, redisCli iowrappers.RedisClient) (resp PlanningResponse, err error) {
	if !travelTimeValidation(req) {
		err = errors.New("travel time limit exceeded for current selection")
		resp.Errcode = InvalidSolverReqTimeInterval
		return
	}

	// validate location with poiSearcher of the time matcher
	for _, slotRequest := range req.SlotRequests {
		location := slotRequest.Location
		if !solver.ValidateLocation(location) {
			err = errors.New("invalid travel destination")
			resp.Errcode = InvalidRequestLocation
			return
		}
	}
	// each row contains candidates in one slot
	candidates := make([][]SlotSolutionCandidate, len(req.SlotRequests))
	for idx := range req.SlotRequests {
		candidates[idx] = make([]SlotSolutionCandidate, 0)
	}

	for idx, slotRequest := range req.SlotRequests {
		location, evTag, stayTimes := slotRequest.Location, slotRequest.EvOption, slotRequest.StayTimes
		slotSolution := GenerateSlotSolution(solver.matcher, location, evTag, stayTimes, req.SearchRadius, req.Weekday, redisCli)
		// The candidates in each slot should satisfy the travel time constraints and inter-slot constraint
		for _, candidate := range slotSolution.SlotSolutionCandidates {
			candidates[idx] = append(candidates[idx], candidate)
		}
	}

	resp.Solutions = genBestMultiSlotSolutions(&candidates)
	return
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

func genBestMultiSlotSolutions(candidates *[][]SlotSolutionCandidate) []MultiSlotSolution {
	res := make([]MultiSlotSolution, 0)
	slotSolutionResults := make([][]SlotSolutionCandidate, 0)
	path := make([]SlotSolutionCandidate, 0)
	placeMap := make(map[string]bool)
	dfs(candidates, 0, &path, &slotSolutionResults, placeMap)

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
	bestSolutions := FindBestSolutions(res)
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

func dfs(candidates *[][]SlotSolutionCandidate, depth int, path *[]SlotSolutionCandidate,
	results *[][]SlotSolutionCandidate, placeMap map[string]bool) {
	if depth == len(*candidates) {
		tmp := make([]SlotSolutionCandidate, depth)
		copy(tmp, *path)
		*results = append(*results, tmp)
		return
	}
	candidates_ := *candidates
	for idx := 0; idx < len(candidates_[depth]); idx++ {
		if !checkDuplication(placeMap, &candidates_[depth][idx]) {
			continue
		}
		*path = append((*path), candidates_[depth][idx])
		dfs(candidates, depth+1, path, results, placeMap)
		*path = (*path)[:len(*path)-1]
		removePlaceIds(placeMap, &candidates_[depth][idx])
	}
}

func removePlaceIds(placesMap map[string]bool, slotSolutionCandidate *SlotSolutionCandidate) {
	for _, placeId := range slotSolutionCandidate.PlaceIDS {
		placesMap[placeId] = false
	}
}

func checkDuplication(placesMap map[string]bool, slotSolutionCandidate *SlotSolutionCandidate) bool {
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
func FindBestSolutions(candidates []MultiSlotSolution) []MultiSlotSolution {
	m := make(map[string]MultiSlotSolution) // map for result extraction
	vertexes := make([]graph.Vertex, len(candidates))
	for idx, candidate := range candidates {
		candidateKey := strconv.FormatInt(int64(idx), 10)
		vertex := graph.Vertex{Name: candidateKey, Key: candidate.Score}
		vertexes[idx] = vertex
		m[candidateKey] = candidate
	}
	// use limited-size minimum priority queue
	priorityQueue := graph.MinPriorityQueue{Nodes: make([]graph.Vertex, 0)}
	for _, vertex := range vertexes {
		if priorityQueue.Size() == NumSolutions {
			top := priorityQueue.GetRoot()
			if vertex.Key > top.Key {
				priorityQueue.ExtractTop()
			} else {
				continue
			}
		}
		priorityQueue.Insert(vertex)
	}

	res := make([]MultiSlotSolution, 0)

	for priorityQueue.Size() > 0 {
		res = append(res, m[priorityQueue.ExtractTop()])
	}

	return res
}

// Generate a standard request while we seek a better way to represent complex REST requests
func GetStandardRequest() (req PlanningRequest) {
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
	req.Weekday = POI.DATE_FRIDAY
	return
}
