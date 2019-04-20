package solution

import (
	"Vacation-planner/matching"
	"fmt"
	"strings"
	"time"
)

const(
	EVENT_EATERY = iota + 10// avoid default 0s
	EVENT_VISIT
	EVENT_TRAVEL
)
const EATARY_LIMIT_PER_SLOT = 1
const VISIT_LIMIT_PER_SLOT = 3
const CANDIDATE_QUEUE_LENGTH = 20
const CANDIDATE_QUEUE_DISPLAY = 5
type TripEvent struct{
	tag uint8
	starttime time.Time
	endtime time.Time
	startplace matching.Place
	endplace matching.Place
//For T events, start place and end place are different
//For E events, start place and end place are same
}
/*
* Multi-Dimentional Tag iterator
* implemented to iterate candidate solutions over the places
* according to valid tags
 */
type MDtagIter struct {
	tag string
	status []int
	size []int
}

type SlotSolution struct{
	slotag string
	Solution []SlotSoluCandidate
}
type SlotSoluCandidate struct{
	Candidate []TripEvent
	EndPlaceDefault matching.Place
	Score float64
	IsSet bool
}
func (this *MDtagIter) Init(tag string, sizeE int, sizeV int) bool {
	if tag == "" {
	return false
	}
	this.tag = tag
	this.status = make([]int, len(tag))
	this.size = make([]int, len(tag))
	for pos, char := range tag {
		this.status[pos] = 0
		if char =='E' || char == 'e' {
		this.size[pos] =  sizeE
		} else if char =='V' || char == 'v'{
			this.size[pos] = sizeV
		}
		if this.size[pos] == 0 {
		return false
		}
	}
	return true
}
func (iterator *MDtagIter) HasNext() bool {
	for i, _ := range iterator.tag {
		if iterator.status[i] < iterator.size[i] - 1 {
			return true
		}
	}
	return false
}

func (this *MDtagIter) Next() bool{
	l := len(this.tag)
	return this.plusone(l-1)
}
func (this *MDtagIter) plusone(l int) bool {
	if l < 0 {
		//log fault
		return false
	}
	if this.status[l] + 1 == this.size[l] {
		this.status[l] = 0
		return this.plusone(l-1)
	} else {
		this.status[l] += 1
		return true
	}
}

func (this *MDtagIter) Reset()  {
	for i := range this.tag {
	this.status[i] = 0
	}
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
		for _, c := range(this.slotag){
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
func (this *SlotSolution) IsTagValid( slotCandidate SlotSoluCandidate) bool {
	if len(this.slotag) == 0 || len(this.Solution) == 0 {
		return false
	}
	solutag := ""
	var count = 0
	for _, cand := range slotCandidate.Candidate {
		if cand.tag == EVENT_EATERY {
			solutag += "E"
			count ++
			} else if cand.tag == EVENT_VISIT {
				solutag += "V"
				count ++
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
func (this *SlotSolution) CreateCandidate( iter MDtagIter, ecluster matching.PlaceCluster, vcluster matching.PlaceCluster) SlotSoluCandidate {
	res := SlotSoluCandidate{}
	res.IsSet = false
	if len(iter.status) != len(this.slotag) {
		//incorrect return
		// return res
		}
	//create a hashtable and iterate through place clusters
	record := make(map[string]bool)
	//check form
	//ASSUME E&V POIs have different placeID
	places := make([]matching.Place, len(iter.status))
	for i, num := range iter.status {
		if this.slotag[i] == 'E' || this.slotag[i] == 'e' {
			_, ok := record[ecluster.Places[num].PlaceId]
			if ok == true {
				return res
			} else {
				record[ecluster.Places[num].PlaceId] = true
				places[i] = ecluster.Places[num]
			}
		} else if this.slotag[i] == 'V' || this.slotag[i] == 'v' {
			_, ok := record[vcluster.Places[num].PlaceId]
			if ok == true {
				return res
			} else {
				record[vcluster.Places[num].PlaceId] = true
				places[i] = vcluster.Places[num]
			}
		} else {
			return res
		}
	}
	//get and set score
	//res.Score = getScore(res.Candidate)
	res.Score = matching.Score(places)
	res.IsSet = true
	fmt.Println(iter.status)
	fmt.Println(res.Score)
	return res
}

func (this *SlotSolution) EnqueueCandidate(candidate SlotSoluCandidate) bool {
	updated := false
	pivot := len(this.Solution)-1
	if len(this.Solution) < CANDIDATE_QUEUE_LENGTH {
		this.Solution = append(this.Solution, candidate)
		updated = true
	} else {
		if this.Solution[pivot].Score < candidate.Score {
			this.Solution[pivot] = candidate
			updated = true
		}
	}
	if updated == false {
		return false
	} else {
		for pivot >= 1 {
			if this.Solution[pivot].Score < this.Solution[pivot -1].Score {
				return true
			} else {
				tempcand := this.Solution[pivot]
				this.Solution[pivot] = this.Solution[pivot -1]
				this.Solution[pivot -1] = tempcand
				pivot--
			}
		}
	return true
	}
}