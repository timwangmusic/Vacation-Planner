package solution

import (
	"Vacation-planner/graph"
	"Vacation-planner/matching"
	"strconv"
	"time"
)

const CANDIDATE_QUEUE_LENGTH = 20
const CANDIDATE_QUEUE_DISPLAY = 5

type TripEvent struct {
	tag        uint8
	startTime  time.Time
	endTime    time.Time
	startPlace matching.Place
	endPlace   matching.Place
}

type SolutionCandidate struct {
	Candidate       []TripEvent
	EndPlaceDefault matching.Place
	Score           float64
	IsSet           bool
}

// Find top solution candidates
func FindBestCandidates(candidates []SolutionCandidate)[]SolutionCandidate{
	m := make(map[string]SolutionCandidate)	// map for result extraction
	vertexes := make([]graph.Vertex, len(candidates))
	for idx, candidate := range candidates{
		candidateKey := strconv.FormatInt(int64(idx), 10)
		vertex := graph.Vertex{Name: candidateKey, Key: candidate.Score}
		vertexes[idx] = vertex
		m[candidateKey] = candidate
	}

	// use limited-size minimum priority queue
	priorityQueue := graph.MinPriorityQueue{Nodes: make([]graph.Vertex, 0)}
	for _, vertex := range vertexes{
		if priorityQueue.Size() == CANDIDATE_QUEUE_LENGTH{
			top := priorityQueue.GetRoot()
			if vertex.Key > top.Key{
				priorityQueue.ExtractTop()
			} else{
				continue
			}
		}
		priorityQueue.Insert(vertex)
	}

	// remove extra vertexes from priority queue
	for priorityQueue.Size() > CANDIDATE_QUEUE_DISPLAY{
		priorityQueue.ExtractTop()
	}

	res := make([]SolutionCandidate, 0)

	for priorityQueue.Size() > 0{
		res = append(res, m[priorityQueue.ExtractTop()])
	}

	return res
}
