package solution

import (
	"errors"
	"fmt"
	"github.com/weihesdlegend/Vacation-planner/matching"
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
const LimitPerSlot = 4

type TripEvents struct {
	tag        uint8
	starttime  time.Time
	endtime    time.Time
	startplace matching.Place
	endplace   matching.Place
	//For T events, start place and end place are different
	//For E events, start place and end place are same
}

type SlotSolution struct {
	SlotTag                string                  `json:"slot_tag"`
	SlotSolutionCandidates []SlotSolutionCandidate `json:"solution"`
}

type SlotSolutionCandidate struct {
	PlaceNames      []string       `json:"place_names"`
	PlaceIDS        []string       `json:"place_ids"`
	PlaceLocations  [][2]float64   `json:"place_locations"`
	PlaceAddresses  []string       `json:"place_addresses"`
	Candidate       []TripEvents   `json:"candidate"`
	EndPlaceDefault matching.Place `json:"end_place_default"`
	Score           float64        `json:"score"`
	IsSet           bool           `json:"is_set"`
}

func (slotSolution *SlotSolution) SetTag(tag string) (err error) {
	if isSlotTagValid(tag) {
		slotSolution.SlotTag = tag
	} else {
		err = errors.New(fmt.Sprintf("Slot tag %s is invalid.", tag))
	}
	return
}

/*
*This function checks if the slots in the solution fits the
*solution requirement
 */
func isSlotTagValid(tag string) bool {
	if tag == "" {
		return false
	} else {
		var eateryCount uint8 = 0
		var visitCount uint8 = 0
		for _, c := range tag {
			if c == 'e' || c == 'E' {
				eateryCount++
			} else if c == 'v' || c == 'V' {
				visitCount++
			} else {
				return false
			}
			if eateryCount+visitCount > LimitPerSlot {
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
	if len(slotSolution.SlotTag) == 0 || len(slotSolution.SlotSolutionCandidates) == 0 {
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
	if count != len(slotSolution.SlotTag) {
		return false
	}
	if strings.EqualFold(solutag, slotSolution.SlotTag) {
		return false
	}
	return true
}

func (slotSolution *SlotSolution) CreateCandidate(iter MDtagIter, categorizedPlaces []CategorizedPlaces) SlotSolutionCandidate {
	res := SlotSolutionCandidate{}
	res.IsSet = false
	if len(iter.Status) != len(slotSolution.SlotTag) {
		return res
	}
	//create a hashtable for deduplication
	record := make(map[string]bool)
	places := make([]matching.Place, len(iter.Status))
	for i, placeIdx := range iter.Status {
		placesByCategory := categorizedPlaces[i]
		visitPlaces := placesByCategory.VisitPlaces
		eateryPlaces := placesByCategory.EateryPlaces
		if slotSolution.SlotTag[i] == 'E' || slotSolution.SlotTag[i] == 'e' {
			_, ok := record[eateryPlaces[placeIdx].PlaceId]
			if ok == true {
				return res
			} else {
				record[eateryPlaces[placeIdx].PlaceId] = true
				places[i] = eateryPlaces[placeIdx]
				res.PlaceIDS = append(res.PlaceIDS, places[i].PlaceId)
				res.PlaceNames = append(res.PlaceNames, places[i].Name)
				res.PlaceLocations = append(res.PlaceLocations, places[i].Location)
				res.PlaceAddresses = append(res.PlaceAddresses, places[i].Address)
			}
		} else if slotSolution.SlotTag[i] == 'V' || slotSolution.SlotTag[i] == 'v' {
			_, ok := record[visitPlaces[placeIdx].PlaceId]
			if ok == true {
				return res
			} else {
				record[visitPlaces[placeIdx].PlaceId] = true
				places[i] = visitPlaces[placeIdx]
				res.PlaceIDS = append(res.PlaceIDS, places[i].PlaceId)
				res.PlaceNames = append(res.PlaceNames, places[i].Name)
				res.PlaceLocations = append(res.PlaceLocations, places[i].Location)
				res.PlaceAddresses = append(res.PlaceAddresses, places[i].Address)
			}
		} else { // abandon previous results even if part of the tag is valid
			return SlotSolutionCandidate{}
		}
	}
	res.Score = matching.Score(places)
	res.IsSet = true
	return res
}
