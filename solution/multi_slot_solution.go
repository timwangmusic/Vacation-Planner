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
	NUM_SOLUTIONS = 5
)

// Solvers are used by planners to solve the planning problem
type Solver struct {
	matcher *matching.TimeMatcher
}

func (solver *Solver) Init(apiKey string, dbName string, dbUrl string) {
	solver.matcher = &matching.TimeMatcher{}
	poiSearcher := &iowrappers.PoiSearcher{}
	mapsClient := &iowrappers.MapsClient{}
	utils.CheckErr(mapsClient.Create(apiKey))
	poiSearcher.Init(mapsClient, dbName, dbUrl)
	solver.matcher.Init(poiSearcher)
}

func (solver *Solver) Solve(req *PlanningRequest) (resp PlanningResponse) {
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

	multiSlotSolution := gen_multi_slot_solution_candidates(&candidates)
	resp.Solution = multiSlotSolution
	return
}

func gen_multi_slot_solution_candidates(candidates *[][]SlotSolutionCandidate) []MultiSlotSolution {
	res := make([]MultiSlotSolution, 0)
	slotSolutionResults := make([][]SlotSolutionCandidate, 0)
	path := make([]SlotSolutionCandidate, 0)
	dfs(candidates, 0, &path, &slotSolutionResults)

	// slot solution results are in the shape of number of multi-slot results by number of slots
	// i.e. each row is ready to fill one multi slot solution
	for _, result := range slotSolutionResults {
		multiSlotSolutionScore := totalScore(&result)
		// TODO: ADD TOTAL TIME CALCULATION USING UPPER-BOUND METHOD
		multiSlotSolution := MultiSlotSolution{
			Score:         multiSlotSolutionScore,
			SlotSolutions: result,
		}
		res = append(res, multiSlotSolution)
	}
	return FindBestSolutions(res)
}

func dfs(candidates *[][]SlotSolutionCandidate, depth int, path *[]SlotSolutionCandidate,
	results *[][]SlotSolutionCandidate) {
	if depth == len(*candidates) {
		tmp := make([]SlotSolutionCandidate, depth)
		copy(tmp, *path)
		*results = append(*results, tmp)
		return
	}
	candidates_ := *candidates
	for idx := 0; idx < len(candidates_[depth]); idx++ {
		*path = append((*path), candidates_[depth][idx])
		dfs(candidates, depth+1, path, results)
		*path = (*path)[:len(*path)-1]
	}
}

func totalScore(candidates *[]SlotSolutionCandidate) float64 {
	score := 0.0
	for _, candidate := range *candidates {
		score += candidate.Score
	}
	return score
}

type MultiSlotSolution struct {
	SlotSolutions []SlotSolutionCandidate
	TotalTime     uint
	Score         float64
}

type PlanningRequest struct {
	SlotRequests []SlotRequest
	SearchRadius uint
	Weekday      POI.Weekday
}

type SlotRequest struct {
	Location     string              // lat,lng
	TimeInterval matching.TimeSlot   // e.g. "8AM-12PM"
	EvOption     string              // e.g. "EVV", "VEV"
	StayTimes    []matching.TimeSlot // e.g. ["8AM-10AM", "10AM-11AM", "11AM-12PM"]
}

type PlanningResponse struct {
	Solution []MultiSlotSolution
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
