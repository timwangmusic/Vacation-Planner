package POI

import (
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/weihesdlegend/Vacation-planner/utils"
)

var closedRe = regexp.MustCompile(`.*\s+Closed$`)
var hourRegex = regexp.MustCompile(`\d{1,2}:\d{2}`)

type Hour uint8

func (h Hour) ToString() string {
	return strconv.Itoa(int(h))
}

type TimeInterval struct {
	Start Hour `json:"start"`
	End   Hour `json:"end"`
}

type ByStartTime []TimeInterval

func (timeIntervals ByStartTime) Len() int {
	return len(timeIntervals)
}

func (timeIntervals ByStartTime) Swap(i, j int) {
	timeIntervals[i], timeIntervals[j] = timeIntervals[j], timeIntervals[i]
}

func (timeIntervals ByStartTime) Less(i, j int) bool {
	return timeIntervals[i].Start <= timeIntervals[j].Start
}

func (interval *TimeInterval) Serialize() string {
	return strconv.FormatUint(uint64(interval.Start), 10) + "_" + strconv.FormatUint(uint64(interval.End), 10)
}

func (interval *TimeInterval) Intersect(newInterval *TimeInterval) bool {
	if interval.End <= newInterval.Start || interval.Start >= newInterval.End {
		return false
	}
	return true
}

func (interval *TimeInterval) Inclusive(newInterval *TimeInterval) bool {
	return newInterval.Start >= interval.Start && newInterval.End <= interval.End
}

// ParseTimeInterval returns a TimeInterval with start and end hour in [0-24], given a string of form "Monday: 9:30AM - 8:00PM",
func ParseTimeInterval(openingHour string) (interval TimeInterval, err error) {
	closed := closedRe.MatchString(openingHour)
	if closed {
		interval.Start = 255
		interval.End = 255
		return
	}
	hours := hourRegex.FindAll([]byte(openingHour), -1)

	amPmRe := regexp.MustCompile(`[apAP][mM]`)
	amPm := amPmRe.FindAll([]byte(openingHour), -1)

	if len(hours) < 2 || len(amPm) < 2 {
		return TimeInterval{}, errors.New("cannot parse opening hour")
	}

	interval.Start = Hour(calculateHour(string(hours[0]), string(amPm[0])))
	interval.End = Hour(calculateHour(string(hours[1]), string(amPm[1])))

	if interval.Start > interval.End { // set end time the last hour of the day
		interval.End = 24
	}

	return
}

func calculateHour(time string, amPm string) uint8 {
	t := strings.Split(time, ":")
	hour, err := strconv.ParseUint(t[0], 10, 8)
	utils.LogErrorWithLevel(err, utils.LogError)

	if amPm == "AM" || amPm == "am" {
		if hour == 12 {
			return 24
		}
		return uint8(hour)
	} else if amPm == "PM" || amPm == "pm" {
		if hour == 12 {
			return 12
		}
		return uint8(hour) + 12
	}
	return 255 // err
}
