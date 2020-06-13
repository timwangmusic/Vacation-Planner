package planner

import (
	"errors"
	"fmt"
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/matching"
	"github.com/weihesdlegend/Vacation-planner/solution"
	"strings"
)

func processPlanningPostRequest(req *PlanningPostRequest) (planningRequest solution.PlanningRequest, err error) {
	if req.Weekday > POI.DateSunday || req.Weekday < POI.DateMonday {
		err = errors.New("invalid weekday in the request")
		return
	}

	planningRequest.Weekday = req.Weekday
	planningRequest.SearchRadius = 10000
	// basic POST parameter validations
	if req.StartTime == 0 || req.EndTime == 0 {
		req.StartTime = 9
		req.EndTime = 22
	}

	if req.NumEatery == 0 {
		req.NumEatery = 1
	}

	if req.NumVisit == 0 {
		req.NumVisit = 2
	}

	err = checkPostReqTimePlaceNum(req)
	if err != nil {
		return
	}

	planningRequest.SlotRequests = GenSlotRequests(*req)
	return
}

func GenSlotRequests(req PlanningPostRequest) []solution.SlotRequest {
	// grouping
	numGroups := uint(1)
	numVisit, numEatery := req.NumVisit, req.NumEatery
	if req.NumEatery > req.NumVisit {
		numGroups = req.NumEatery
	} else {
		numGroups = req.NumVisit
	}
	groups := make([][]string, numGroups)
	for idx := range groups {
		groups[idx] = make([]string, 0)
	}
	// construct groups, make sure eatery appear before visit locations
	// depends on the location type ratio, some groups might only has 1 location
	if req.NumVisit > req.NumEatery {
		ratio := int(req.NumVisit / req.NumEatery)
		for idx := range groups {
			groups[idx] = append(groups[idx], "V")
			if idx%ratio == 0 && numEatery > 0 {
				groups[idx] = append([]string{"E"}, groups[idx]...)
				numEatery--
			}
		}
	} else {
		ratio := int(req.NumEatery / req.NumVisit)
		for idx := range groups {
			groups[idx] = append(groups[idx], "E")
			if idx%ratio == 0 && numVisit > 0 {
				groups[idx] = append(groups[idx], "V")
				numVisit--
			}
		}
	}

	// time allocation
	numHours := int(req.EndTime - req.StartTime)
	hours := make([]int, numGroups)

	for idx := range hours {
		hours[idx] = len(groups[idx])
		numHours -= hours[idx]
	}

	groupIdx := 0
	for numHours > 0 {
		hours[groupIdx] += 1
		groupIdx++
		numHours--
		if groupIdx == len(groups) {
			groupIdx = 0
		}
	}

	slotRequests := make([]solution.SlotRequest, numGroups)
	cityCountry := req.City + "," + req.Country

	curTime := req.StartTime

	for groupIdx := range slotRequests {
		slotRequests[groupIdx].Location = cityCountry
		slotRequests[groupIdx].EvOption = strings.Join(groups[groupIdx], "")
		slotRequests[groupIdx].StayTimes = make([]matching.TimeSlot, len(groups[groupIdx]))
		allocatedTime := hours[groupIdx]
		for placeIdx, placeType := range groups[groupIdx] {
			curSlot := matching.TimeSlot{}
			if placeType == "E" {
				curSlot.Slot.Start = curTime
				curSlot.Slot.End = curTime + 1
				allocatedTime -= 1
				curTime += 1
			} else {
				curSlot.Slot.Start = curTime
				curTime += POI.Hour(allocatedTime)
				curSlot.Slot.End = curTime
			}
			slotRequests[groupIdx].StayTimes[placeIdx] = curSlot
		}
	}

	groupIdx = 0
	curGroupIdx := 1
	excludedGroupIndexes := make(map[int]bool)

	// combine groups and limit maximum number of groups
	for numGroups > 3 && curGroupIdx < len(slotRequests) {
		if len(slotRequests[groupIdx].EvOption)+len(slotRequests[curGroupIdx].EvOption) <= MaxPlacesPerSlot {
			slotRequests[groupIdx].EvOption = slotRequests[groupIdx].EvOption + slotRequests[curGroupIdx].EvOption
			slotRequests[groupIdx].StayTimes = append(slotRequests[groupIdx].StayTimes, slotRequests[curGroupIdx].StayTimes...)
			excludedGroupIndexes[curGroupIdx] = true
			curGroupIdx++
			numGroups--
		} else {
			groupIdx = curGroupIdx
			curGroupIdx++
		}
	}

	finalRes := make([]solution.SlotRequest, 0)
	for idx, slotReq := range slotRequests {
		if _, exist := excludedGroupIndexes[idx]; !exist {
			finalRes = append(finalRes, slotReq)
		}
	}
	return finalRes
}

func checkPostReqTimePlaceNum(req *PlanningPostRequest) (err error) {
	if req.StartTime > 24 || req.EndTime > 24 {
		err = errors.New("invalid time, valid times are chosen from 1-24")
		return
	}
	if req.StartTime >= req.EndTime {
		err = errors.New("start time cannot be later than end time")
		return
	}

	if req.NumEatery+req.NumVisit > MaxPlacesPerDay {
		err = fmt.Errorf("total number of places cannot exceed %d", MaxPlacesPerDay)
		return
	}

	if req.NumEatery+req.NumVisit > uint(req.EndTime-req.StartTime) {
		err = errors.New("not enough time for visiting all the places")
	}
	return
}
