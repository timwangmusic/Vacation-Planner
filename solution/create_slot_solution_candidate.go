package solution

import (
	"fmt"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"github.com/weihesdlegend/Vacation-planner/matching"
	"strings"
)

const (
	EventEatery = iota + 10 // avoid default 0s
	EventVisit
)

const (
	LimitPerSlot = 4
)

type Properties struct {
	tag uint8
}

type SlotSolution struct {
	SlotTag                string                  `json:"slot_tag"`
	SlotSolutionCandidates []SlotSolutionCandidate `json:"solution"`
}

type SlotSolutionCandidate struct {
	PlaceNames     []string     `json:"place_names"`
	PlaceIDS       []string     `json:"place_ids"`
	PlaceLocations [][2]float64 `json:"place_locations"`
	PlaceAddresses []string     `json:"place_addresses"`
	PlaceURLs      []string     `json:"place_urls"`
	Properties     []Properties `json:"properties"`
	Score          float64      `json:"score"`
	IsSet          bool         `json:"is_set"`
}

func (slotSolution *SlotSolution) SetTag(tag string) (err error) {
	if isSlotTagValid(tag) {
		slotSolution.SlotTag = tag
	} else {
		err = fmt.Errorf("slot tag %s is invalid", tag)
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
	}

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

/*
* This function matches the slot tag and those of its solutions
 */
func (slotSolution *SlotSolution) IsCandidateTagValid(slotCandidate SlotSolutionCandidate) bool {
	if len(slotSolution.SlotTag) == 0 || len(slotSolution.SlotSolutionCandidates) == 0 {
		return false
	}
	slotTag := ""
	var count = 0
	for _, c := range slotCandidate.Properties {
		if c.tag == EventEatery {
			slotTag += "E"
			count++
		} else if c.tag == EventVisit {
			slotTag += "V"
			count++
		}
	}
	if count != len(slotSolution.SlotTag) {
		return false
	}
	if strings.EqualFold(slotTag, slotSolution.SlotTag) {
		return false
	}
	return true
}

func (slotSolution *SlotSolution) CreateCandidate(iter MDtagIter, categorizedPlaces []CategorizedPlaces) (res SlotSolutionCandidate) {
	if len(iter.Status) != len(slotSolution.SlotTag) {
		return
	}
	// deduplication of repeating places in the result
	record := make(map[string]bool)
	places := make([]matching.Place, len(iter.Status))
	for i, placeIdx := range iter.Status {
		placesByCategory := categorizedPlaces[i]
		visitPlaces := placesByCategory.VisitPlaces
		eateryPlaces := placesByCategory.EateryPlaces
		// a single-letter place category identifier
		categoryIdentifier := strings.ToLower(string(slotSolution.SlotTag[i]))
		var place matching.Place
		if categoryIdentifier == "e" {
			place = eateryPlaces[placeIdx]
		} else if categoryIdentifier == "v" {
			place = visitPlaces[placeIdx]
		}

		// if the same place appears in two indexes, return incomplete result
		if _, exist := record[place.GetPlaceId()]; exist {
			return
		}

		record[place.GetPlaceId()] = true
		places[i] = place
		res.PlaceIDS = append(res.PlaceIDS, place.GetPlaceId())
		res.PlaceNames = append(res.PlaceNames, place.GetPlaceName())
		res.PlaceLocations = append(res.PlaceLocations, place.GetLocation())
		res.PlaceAddresses = append(res.PlaceAddresses, place.GetPlaceFormattedAddress())
		if len(strings.TrimSpace(place.GetURL())) == 0 {
			place.SetURL(iowrappers.GoogleSearchHomePageURL)
		}
		res.PlaceURLs = append(res.PlaceURLs, place.GetURL())
	}
	res.Score = matching.Score(places)
	res.IsSet = true
	return
}
