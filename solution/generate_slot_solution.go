package solution

import (
	"container/heap"
	"errors"
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/graph"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"github.com/weihesdlegend/Vacation-planner/matching"
	"strconv"
	"strings"
	"time"
)

const (
	CandidateQueueLength                  = 15
	ReqTimeSlotsTagMismatchErrMsg         = "user designated stay times list length does not match tag length"
	CategorizedPlaceIterInitFailureErrMsg = "categorized places iterator init failure"
)

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
	priorityQueue := &graph.MinPriorityQueueVertex{}
	for _, vertex := range vertexes {
		if priorityQueue.Len() == CandidateQueueLength {
			top := (*priorityQueue)[0]
			if vertex.Key > top.Key {
				heap.Pop(priorityQueue)
			} else {
				continue
			}
		}
		heap.Push(priorityQueue, vertex)
	}

	res := make([]SlotSolutionCandidate, 0)

	for priorityQueue.Len() > 0 {
		top := heap.Pop(priorityQueue).(graph.Vertex)
		res = append(res, m[top.Name])
	}

	return res
}

// Generate slot solution candidates
// Parameter list matches slot request
func GenerateSlotSolution(timeMatcher *matching.TimeMatcher, location string, evTag string, stayTimes []matching.TimeSlot,
	radius uint, weekday POI.Weekday, redisClient iowrappers.RedisClient) (slotSolution SlotSolution, slotSolutionRedisKey string, err error) {
	if len(stayTimes) != len(evTag) {
		err = errors.New(ReqTimeSlotsTagMismatchErrMsg)
		return
	}

	err = slotSolution.SetTag(evTag)
	if err != nil {
		return
	}

	intervals := make([]POI.TimeInterval, len(stayTimes))
	for idx, stayTime := range stayTimes {
		intervals[idx] = stayTime.Slot
	}

	cityCountry := strings.Split(location, ",")
	evTags := make([]string, len(stayTimes))
	for idx, c := range evTag {
		evTags[idx] = string(c)
	}

	redisReq := iowrappers.SlotSolutionCacheRequest{
		Country:   cityCountry[1],
		City:      cityCountry[0],
		Radius:    uint64(radius),
		EVTags:    evTags,
		Intervals: intervals,
		Weekday:   weekday,
	}

	slotSolutionCacheResp, slotSolutionRedisKey, cacheErr := redisClient.GetSlotSolution(redisReq)
	if cacheErr == nil { // cache hit
		for _, candidate := range slotSolutionCacheResp.SlotSolutionCandidate {
			slotSolutionCandidate := SlotSolutionCandidate{
				PlaceNames:      candidate.PlaceNames,
				PlaceIDS:        candidate.PlaceIds,
				PlaceLocations:  candidate.PlaceLocations,
				PlaceAddresses:  candidate.PlaceAddresses,
				EndPlaceDefault: matching.Place{},
				Score:           candidate.Score,
				IsSet:           true,
			}
			slotSolution.SlotSolutionCandidates = append(slotSolution.SlotSolutionCandidates, slotSolutionCandidate)
		}
		return
	}

	slotSolution.SlotSolutionCandidates = make([]SlotSolutionCandidate, 0)
	slotCandidates := make([]SlotSolutionCandidate, 0)

	req := matching.TimeMatchingRequest{}

	req.Location = location
	if radius <= 0 {
		radius = 2000
	}
	req.Radius = radius

	req.TimeSlots = stayTimes

	req.Weekday = weekday

	placeClusters := timeMatcher.Matching(&req)

	categorizedPlaces := make([]CategorizedPlaces, len(placeClusters))

	for idx, placeCluster := range placeClusters {
		categorizedPlaces[idx] = Categorize(placeCluster)
	}

	minuteLimit := GetTimeSlotLengthInMin(placeClusters)

	mdIter := MDtagIter{}
	if !mdIter.Init(evTag, categorizedPlaces) {
		err = errors.New(CategorizedPlaceIterInitFailureErrMsg)
		return
	}

	for mdIter.HasNext() {
		curCandidate := slotSolution.CreateCandidate(mdIter, categorizedPlaces)

		if curCandidate.IsSet {
			_, travelTimeInMin := GetTravelTimeByDistance(categorizedPlaces, mdIter)
			if travelTimeInMin <= float64(minuteLimit) {
				slotCandidates = append(slotCandidates, curCandidate)
			}
		}
		mdIter.Next()
	}
	bestCandidates := FindBestCandidates(slotCandidates)
	slotSolution.SlotSolutionCandidates = append(slotSolution.SlotSolutionCandidates, bestCandidates...)

	// cache slot solution calculation results
	slotSolutionToCache := iowrappers.SlotSolutionCacheResponse{}
	slotSolutionToCache.SlotSolutionCandidate = make([]iowrappers.SlotSolutionCandidateCache, len(slotSolution.SlotSolutionCandidates))

	for idx, slotSolutionCandidate := range slotSolution.SlotSolutionCandidates {
		candidateCache := iowrappers.SlotSolutionCandidateCache{
			PlaceIds:       slotSolutionCandidate.PlaceIDS,
			Score:          slotSolutionCandidate.Score,
			PlaceNames:     slotSolutionCandidate.PlaceNames,
			PlaceLocations: slotSolutionCandidate.PlaceLocations,
			PlaceAddresses: slotSolutionCandidate.PlaceAddresses,
		}
		slotSolutionToCache.SlotSolutionCandidate[idx] = candidateCache
	}

	redisClient.CacheSlotSolution(redisReq, slotSolutionToCache)

	return
}
