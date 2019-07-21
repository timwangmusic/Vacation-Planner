package solution

import (
	"Vacation-planner/POI"
	"Vacation-planner/graph"
	"Vacation-planner/matching"
	"Vacation-planner/utils"
	log "github.com/sirupsen/logrus"
	"strconv"
	"time"
)

const CANDIDATE_QUEUE_LENGTH = 20
const CANDIDATE_QUEUE_DISPLAY = 15

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
		log.Fatal("position cluster file reading error")
		return SlotSolution{}
	}
	if len(staytime) != len(tag) {
		log.Fatal("Stay time does not match tag")
		return SlotSolution{}
	}
	cclusters := Categorize(&pclusters[slotIndex])
	minutelimit := GetSlotLengthinMin(&pclusters[slotIndex])
	if minutelimit == 0 {
		log.Fatal("Slot time setting invalid")
		return SlotSolution{}
	}
	slotSolution1 := SlotSolution{}
	slotSolution1.SetTag(tag)
	if !slotSolution1.IsSlotagValid() {
		log.Fatal("tag format not supported")
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

	query_time_slot := matching.TimeSlot{
		POI.TimeInterval{
			stayTimes[0].Slot.Start,
			stayTimes[len(stayTimes)-1].Slot.End,
		},
	}
	// only one big time slot
	req.TimeSlots = []matching.TimeSlot{query_time_slot}

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
