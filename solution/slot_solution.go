package solution

import (
	"Vacation-planner/POI"
	"Vacation-planner/graph"
	"Vacation-planner/matching"
	log "github.com/sirupsen/logrus"
	"strconv"
	"time"
)

const CandidateQueueLength = 20
const CandidateQueueDisplay = 15

type TripEvent struct {
	tag        uint8
	startTime  time.Time
	endTime    time.Time
	startPlace matching.Place
	endPlace   matching.Place
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
		if priorityQueue.Size() == CandidateQueueLength {
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
	for priorityQueue.Size() > CandidateQueueDisplay {
		priorityQueue.ExtractTop()
	}

	res := make([]SlotSolutionCandidate, 0)

	for priorityQueue.Size() > 0 {
		res = append(res, m[priorityQueue.ExtractTop()])
	}

	return res
}

// Generate slot solution candidates
// Parameter list matches slot request
func GenerateSlotSolution(timeMatcher *matching.TimeMatcher, location string, EVtag string,
	stayTimes []matching.TimeSlot, radius uint, weekday POI.Weekday) (slotSolution SlotSolution) {
	if len(stayTimes) != len(EVtag) {
		log.Fatal("User designated stay time does not match tag.")
		return
	}

	slotSolution.SetTag(EVtag)
	if !slotSolution.IsSlotagValid() {
		log.Fatalf("Slot tag %s is invalid.", EVtag)
		return
	}

	slotSolution.Solution = make([]SlotSolutionCandidate, 0)
	slotCandidates := make([]SlotSolutionCandidate, 0)

	req := matching.TimeMatchingRequest{}

	req.Location = location
	if radius <= 0 {
		radius = 2000
	}
	req.Radius = radius

	queryTimeSlot := matching.TimeSlot{
		Slot: POI.TimeInterval{
			Start: stayTimes[0].Slot.Start,
			End:   stayTimes[len(stayTimes)-1].Slot.End,
		},
	}
	// only one big time slot
	req.TimeSlots = []matching.TimeSlot{queryTimeSlot}

	if weekday < POI.DATE_MONDAY || weekday > POI.DATE_SUNDAY {
		weekday = POI.DATE_SATURDAY
	}
	req.Weekday = weekday

	placeClusters := timeMatcher.Matching(&req)

	categorizedPlaces := Categorize(&placeClusters[0])
	minuteLimit := GetSlotLengthinMin(&placeClusters[0])

	mdIter := MDtagIter{}
	mdIter.Init(EVtag, categorizedPlaces)

	for mdIter.HasNext() {
		curCandidate := slotSolution.CreateCandidate(mdIter, categorizedPlaces)

		if curCandidate.IsSet {
			_, travelTimeInMin := GetTravelTimeByDistance(categorizedPlaces, mdIter)
			if travelTimeInMin <= float64(minuteLimit) {
				//FIXME: ADD TRIP EVENT GENERATION FUNCTION CALL
				slotCandidates = append(slotCandidates, curCandidate)
			}
		}
		mdIter.Next()
	}
	bestCandidates := FindBestCandidates(slotCandidates)
	slotSolution.Solution = append(slotSolution.Solution, bestCandidates...)

	return
}
