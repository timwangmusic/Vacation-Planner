package solution

import (
	"Vacation-planner/POI"
	"Vacation-planner/graph"
	"Vacation-planner/iowrappers"
	"Vacation-planner/matching"
	"Vacation-planner/utils"
	"strconv"
)

const (
	NUM_SOLUTIONS               = 5
	TRAVEL_SPEED                = 50 // km/h
	TIME_LIMIT_BETWEEN_CLUSTERS = 60 // minutes
)

// Solvers are used by planners to solve the planning problem
type Solver struct {
	matcher *matching.TimeMatcher
}
const (
	SOLVER_NO_ERROR = iota
	SOLVER_REQ_TIME_INTERVAL_INVALID
	SOLVER_ERROR_MAX
)


func (solver *Solver) SolverProcessError(errstring string, errorcode uint, resp * PlanningResponse)(err error){
	err = utils.GenerateErr("Travel time limit exceeded for current selection")
	resp.Err = err
	resp.Errcode = errorcode
	return
}

func (solver *Solver) Init(apiKey string, dbName string, dbUrl string, redis_addr string, redis_psw string, redis_idx int) {
	solver.matcher = &matching.TimeMatcher{}
	poiSearcher := &iowrappers.PoiSearcher{}
	mapsClient := &iowrappers.MapsClient{}
	utils.CheckErr(mapsClient.Create(apiKey))
	poiSearcher.Init(mapsClient, dbName, dbUrl, redis_addr, redis_psw, redis_idx)
	solver.matcher.Init(poiSearcher)
}

func (solver *Solver) Solve(req PlanningRequest) (resp PlanningResponse, err error) {
	if !travelTimeValidation(req) {
		solver.SolverProcessError("Travel time limit exceeded for current selection", SOLVER_REQ_TIME_INTERVAL_INVALID, &resp)
		return
	}
	// each row contains candidates in one slot
	candidates := make([][]SlotSolutionCandidate, len(req.SlotRequests))
	for idx := range req.SlotRequests {
		candidates[idx] = make([]SlotSolutionCandidate, 0)
	}

	for idx, slotRequest := range req.SlotRequests {
		location, evTag, stayTimes := slotRequest.Location, slotRequest.EvOption, slotRequest.StayTimes
		slotSolution := GenerateSlotSolution(solver.matcher, location, evTag, stayTimes, req.SearchRadius,
			req.Weekday)
		// The candidates in each slot should satisfy the travel time constraints and inter-slot constraint
		for _, candidate := range slotSolution.Solution {
			candidates[idx] = append(candidates[idx], candidate)
		}
	}

	multiSlotSolution := genMultiSlotSolutionCandidates(&candidates)

	resp.Solution = multiSlotSolution
	return
}

// return false if travel time between clusters exceed limit
// use upper-bound of the sum of radius plus distance between cluster centers
func travelTimeValidation(req PlanningRequest) bool {
	numTimeSlots := len(req.SlotRequests)

	for i := 0; i < numTimeSlots-1; i++ {
		prevRequest := req.SlotRequests[i]
		nextRequest := req.SlotRequests[i+1]
		if travelTime(prevRequest.Location, nextRequest.Location, req.SearchRadius, req.SearchRadius) > TIME_LIMIT_BETWEEN_CLUSTERS {
			return false
		}
	}
	return true
}

func travelTime(from_loc string, to_loc string, from_loc_radius uint, to_loc_radius uint) uint {
	latlng1 := utils.ParseLocation(from_loc)
	latlng2 := utils.ParseLocation(to_loc)

	distance := utils.HaversineDist(latlng1, latlng2) + float64(from_loc_radius+to_loc_radius)

	return uint(distance / (TRAVEL_SPEED * 16.67)) // 16.67 is the ratio of m/minute and km/hour
}

func genMultiSlotSolutionCandidates(candidates *[][]SlotSolutionCandidate) []MultiSlotSolution {
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

		startLatlng, endLatlng := make([]float64, 2), make([]float64, 2)
		startLatlng[0], startLatlng[1] = startPlace[0], startPlace[1]
		endLatlng[0], endLatlng[1] = endPlace[0], endPlace[1]

		distance := utils.HaversineDist(startLatlng, endLatlng)
		intervalTime := uint(distance / (TRAVEL_SPEED * 16.67))

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
	Location     string              // city,country
	TimeInterval matching.TimeSlot   // e.g. "8AM-12PM"
	EvOption     string              // e.g. "EVV", "VEV"
	StayTimes    []matching.TimeSlot // e.g. ["8AM-10AM", "10AM-11AM", "11AM-12PM"]
}

type PlanningResponse struct {
	Solution []MultiSlotSolution
	Err error
	Errcode uint
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
		if priorityQueue.Size() == NUM_SOLUTIONS {
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
	slot11 := matching.TimeSlot{POI.TimeInterval{8, 9}}
	slot12 := matching.TimeSlot{POI.TimeInterval{9, 11}}
	slot13 := matching.TimeSlot{POI.TimeInterval{11, 12}}
	stayTimes1 := []matching.TimeSlot{slot11, slot12, slot13}
	timeslot_1 := matching.TimeSlot{POI.TimeInterval{8, 12}}
	slotReq1 := SlotRequest{
		Location:     "",
		TimeInterval: timeslot_1,
		EvOption:     "EVV",
		StayTimes:    stayTimes1,
	}

	slot21 := matching.TimeSlot{POI.TimeInterval{12, 13}}
	slot22 := matching.TimeSlot{POI.TimeInterval{13, 17}}
	slot23 := matching.TimeSlot{POI.TimeInterval{17, 19}}
	stayTimes2 := []matching.TimeSlot{slot21, slot22, slot23}
	timeslot2 := matching.TimeSlot{POI.TimeInterval{12, 19}}
	slotReq2 := SlotRequest{
		Location:     "",
		TimeInterval: timeslot2,
		EvOption:     "EVV",
		StayTimes:    stayTimes2,
	}

	slot31 := matching.TimeSlot{POI.TimeInterval{19, 21}}
	slot32 := matching.TimeSlot{POI.TimeInterval{21, 23}}
	stayTimes3 := []matching.TimeSlot{slot31, slot32}
	timeslot3 := matching.TimeSlot{POI.TimeInterval{19, 23}}
	slotReq3 := SlotRequest{
		Location:     "",
		TimeInterval: timeslot3,
		EvOption:     "EV",
		StayTimes:    stayTimes3,
	}

	req.SlotRequests = append(req.SlotRequests, []SlotRequest{slotReq1, slotReq2, slotReq3}...)
	req.Weekday = POI.DATE_FRIDAY
	return
}
