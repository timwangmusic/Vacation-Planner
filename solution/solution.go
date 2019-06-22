package solution

import (
	"Vacation-planner/graph"
	"Vacation-planner/matching"
	"Vacation-planner/utils"
	"github.com/sirupsen/logrus"
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

func FindBestCandidates(candidates []SlotSolutionCandidate) []SlotSolutionCandidate {
	m := make(map[string]SlotSolutionCandidate) // map for result extraction
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
		if priorityQueue.Size() == CANDIDATE_QUEUE_LENGTH {
			top := priorityQueue.GetRoot()
			if vertex.Key > top.Key {
				priorityQueue.ExtractTop()
			} else {
				continue
			}
		}
		priorityQueue.Insert(vertex)
	}

	// remove extra vertexes from priority queue
	for priorityQueue.Size() > CANDIDATE_QUEUE_DISPLAY {
		priorityQueue.ExtractTop()
	}

	res := make([]SlotSolutionCandidate, 0)

	for priorityQueue.Size() > 0 {
		res = append(res, m[priorityQueue.ExtractTop()])
	}

	return res
}

/*
 *	filename: the input json file
 *	tag: defines the travel patterns in a slot
 *	staytime: the estimated stay time at each POI
 */
func GenerateSlotSolutionFromFile(filename string, tag string, staytime []int, slotIndex int) SlotSolution {
	var pclusters []matching.PlaceCluster
	var sCandidate []SlotSolutionCandidate

	err := utils.ReadFromFile(filename, &pclusters)
	if err != nil {
		logrus.Fatal("position cluster file reading error")
		return SlotSolution{}
	}
	if len(staytime) != len(tag) {
		logrus.Fatal("Stay time does not match tag")
		return SlotSolution{}
	}
	cclusters := Categorize(&pclusters[slotIndex])
	minutelimit := GetSlotLengthinMin(&pclusters[slotIndex])
	if minutelimit == 0 {
		logrus.Fatal("Slot time setting invalid")
		return SlotSolution{}
	}
	slotSolution1 := SlotSolution{}
	slotSolution1.SetTag(tag)
	if !slotSolution1.IsSlotagValid() {
		logrus.Fatal("tag format not supported")
		return SlotSolution{}
	}
	mdti := MDtagIter{}
	mdti.Init(tag, cclusters)

	for mdti.HasNext() {
		//iterate through combinations of places according to the tag.
		//fmt.Printf("len=%d cap=%d %v\n", len(mdti.status), cap(mdti.status), mdti.status)
		tempCandidate := slotSolution1.CreateCandidate(mdti, cclusters)
		if tempCandidate.IsSet {
			//check time, generate events
			_, sumtime := GetTravelTimeByDistance(cclusters, mdti)
			//fmt.Printf("len=%d cap=%d %v\n", len(traveltime), cap(traveltime), traveltime)
			if sumtime <= float64(minutelimit) {
				sCandidate = append(sCandidate, tempCandidate)
			}
		}
		//save to priority queue
		mdti.Next()
	}
	slotSolution1.Solution = FindBestCandidates(sCandidate)
	return slotSolution1
}
