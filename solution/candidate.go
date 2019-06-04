package solution

import (
	"Vacation-planner/matching"
	"Vacation-planner/planner"
	"strings"
	"time"
)

const (
	EVENT_EATERY = iota + 10 // avoid default 0s
	EVENT_VISIT
	EVENT_TRAVEL
)
const EATARY_LIMIT_PER_SLOT = 1
const VISIT_LIMIT_PER_SLOT = 3

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
	slotag   string
	Solution []SlotSolutionCandidate
}
type SlotSolutionCandidate struct {
	Candidate       []TripEvents
	EndPlaceDefault matching.Place
	Score           float64
	IsSet           bool
}

func (this *SlotSolution) SetTag(tag string) {
	this.slotag = tag
}

/*
*This function checks if the slots in the solution fits the
*solution requirement
 */
func (this *SlotSolution) IsSlotagValid() bool {
	if this.slotag == "" {
		return false
	} else {
		var eatcount uint8 = 0
		var vstcount uint8 = 0
		for _, c := range this.slotag {
			if c == 'e' || c == 'E' {
				eatcount++
			} else if c == 'v' || c == 'V' {
				if eatcount == 0 {
					//visit before eat, no valid at this time
					return false
				}
				vstcount++
			} else {
				return false
			}
			if eatcount > EATARY_LIMIT_PER_SLOT || vstcount > VISIT_LIMIT_PER_SLOT {
				return false
			}
		}
		return true
	}
}

/*
* This function matches the slot tag and those of its solutions
 */
func (this *SlotSolution) IsCandidateTagValid(slotCandidate SlotSolutionCandidate) bool {
	if len(this.slotag) == 0 || len(this.Solution) == 0 {
		return false
	}
	solutag := ""
	var count = 0
	for _, cand := range slotCandidate.Candidate {
		if cand.tag == EVENT_EATERY {
			solutag += "E"
			count++
		} else if cand.tag == EVENT_VISIT {
			solutag += "V"
			count++
		}
	}
	if count != len(this.slotag) {
		return false
	}
	if strings.EqualFold(solutag, this.slotag) {
		return false
	}
	return true
}
func (this *SlotSolution) CreateCandidate(iter planner.MDtagIter, cplaces planner.CategorizedPlaces) SlotSolutionCandidate {
	res := SlotSolutionCandidate{}
	res.IsSet = false
	if len(iter.Status) != len(this.slotag) {
		//incorrect return
		// return res
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
		if this.slotag[i] == 'E' || this.slotag[i] == 'e' {
			_, ok := record[ecluster[num].PlaceId]
			if ok == true {
				return res
			} else {
				record[ecluster[num].PlaceId] = true
				places[i] = ecluster[num]
			}
		} else if this.slotag[i] == 'V' || this.slotag[i] == 'v' {
			_, ok := record[vcluster[num].PlaceId]
			if ok == true {
				return res
			} else {
				record[vcluster[num].PlaceId] = true
				places[i] = vcluster[num]
			}
		} else {
			return res
		}
	}
	//get and set score
	//res.Score = getScore(res.Candidate)
	res.Score = matching.Score(places)
	res.IsSet = true
	//fmt.Println(iter.Status)
	//fmt.Println(res.Score)
	return res
}
