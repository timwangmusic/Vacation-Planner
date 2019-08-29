package solution

import (
	"Vacation-planner/matching"
	"strings"
	"time"
)

const (
	EventEatery = iota + 10 // avoid default 0s
	EventVisit
	EventTravel
)
const EateryLimitPerSlot = 1
const VisitLimitPerSlot = 3

type TripEvents struct {
	tag        uint8
	starttime  time.Time
	endtime    time.Time
	startplace matching.Place
	endplace   matching.Place
	//For T events, start place and end place are different
	//For E events, start place and end place are same
}

/*
* Multi-Dimentional Tag iterator
* implemented to iterate candidate solutions over the places
* according to valid tags
 */

type SlotSolution struct {
	slotTag  string
	Solution []SlotSolutionCandidate
}
type SlotSolutionCandidate struct {
	PlaceNames      []string
	PlaceIDS        []string
	PlaceLocations  [][2]float64
	Candidate       []TripEvents
	EndPlaceDefault matching.Place
	Score           float64
	IsSet           bool
}

func (slotSolution *SlotSolution) SetTag(tag string) {
	slotSolution.slotTag = tag
}

/*
*This function checks if the slots in the solution fits the
*solution requirement
 */
func (slotSolution *SlotSolution) IsSlotagValid() bool {
	if slotSolution.slotTag == "" {
		return false
	} else {
		var eatcount uint8 = 0
		var vstcount uint8 = 0
		for _, c := range slotSolution.slotTag {
			if c == 'e' || c == 'E' {
				eatcount++
			} else if c == 'v' || c == 'V' {
				if eatcount == 0 {
					//visit before eat, no valid at slotSolution time
					return false
				}
				vstcount++
			} else {
				return false
			}
			if eatcount > EateryLimitPerSlot || vstcount > VisitLimitPerSlot {
				return false
			}
		}
		return true
	}
}

/*
* This function matches the slot tag and those of its solutions
 */
func (slotSolution *SlotSolution) IsCandidateTagValid(slotCandidate SlotSolutionCandidate) bool {
	if len(slotSolution.slotTag) == 0 || len(slotSolution.Solution) == 0 {
		return false
	}
	solutag := ""
	var count = 0
	for _, cand := range slotCandidate.Candidate {
		if cand.tag == EventEatery {
			solutag += "E"
			count++
		} else if cand.tag == EventVisit {
			solutag += "V"
			count++
		}
	}
	if count != len(slotSolution.slotTag) {
		return false
	}
	if strings.EqualFold(solutag, slotSolution.slotTag) {
		return false
	}
	return true
}

func (slotSolution *SlotSolution) CreateCandidate(iter MDtagIter, cplaces CategorizedPlaces) SlotSolutionCandidate {
	res := SlotSolutionCandidate{}
	res.IsSet = false
	if len(iter.Status) != len(slotSolution.slotTag) {
		//incorrect return
		return res
	}
	//create a hashtable and iterate through place clusters
	record := make(map[string]bool)
	//check form
	//ASSUME E&V POIs have different placeID
	ecluster := cplaces.EateryPlaces
	vcluster := cplaces.VisitPlaces
	places := make([]matching.Place, len(iter.Status))
	for i, num := range iter.Status {
		if slotSolution.slotTag[i] == 'E' || slotSolution.slotTag[i] == 'e' {
			_, ok := record[ecluster[num].PlaceId]
			if ok == true {
				return res
			} else {
				record[ecluster[num].PlaceId] = true
				places[i] = ecluster[num]
				res.PlaceIDS = append(res.PlaceIDS, places[i].PlaceId)
				res.PlaceNames = append(res.PlaceNames, places[i].Name)
				res.PlaceLocations = append(res.PlaceLocations, places[i].Location)
			}
		} else if slotSolution.slotTag[i] == 'V' || slotSolution.slotTag[i] == 'v' {
			_, ok := record[vcluster[num].PlaceId]
			if ok == true {
				return res
			} else {
				record[vcluster[num].PlaceId] = true
				places[i] = vcluster[num]
				res.PlaceIDS = append(res.PlaceIDS, places[i].PlaceId)
				res.PlaceNames = append(res.PlaceNames, places[i].Name)
				res.PlaceLocations = append(res.PlaceLocations, places[i].Location)
			}
		} else {
			return res
		}
	}
	res.Score = matching.Score(places)
	res.IsSet = true
	return res
}
