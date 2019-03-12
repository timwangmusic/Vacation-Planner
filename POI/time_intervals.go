package POI

import (
	"Vacation-planner/utils"
	"errors"
	"regexp"
	"strconv"
	"strings"
)

type Hour uint8

type TimeInterval struct {
	Start Hour
	End   Hour
}

// an interface for handling interval-like data structure
type TimeIntervals interface {
	NumIntervals() int                     // get number of time intervals
	GetAllIntervals() *[]TimeInterval      // get all time intervals as a list of Start and End time
	GetInterval(int) (error, TimeInterval) // get an interval by specifying its index
	InsertTimeInterval(TimeInterval)       // add an interval
}

// given a open hours data from Google, we only use the Thursday data to fill GoogleMapsTimeIntervals struct
type GoogleMapsTimeIntervals struct {
	numIntervals int
	Intervals []TimeInterval
}

func (timeIntervals *GoogleMapsTimeIntervals) GetAllIntervals() *[]TimeInterval {
	return &timeIntervals.Intervals
}

func (timeIntervals *GoogleMapsTimeIntervals) GetInterval(idx int) (error, TimeInterval) {
	if idx < 0 || idx >= timeIntervals.numIntervals{
		return errors.New("index out of bound"), TimeInterval{}
	}
	return nil, timeIntervals.Intervals[idx]
}

// assume input having non-overlapping time intervals
// maintain intervals sorted
func (timeIntervals *GoogleMapsTimeIntervals) InsertTimeInterval(interval TimeInterval) {
	if timeIntervals.numIntervals == 0{
		timeIntervals.Intervals = append(timeIntervals.Intervals, interval)
	} else{
		for idx, itv := range timeIntervals.Intervals{
			if itv.Start >= interval.End {
				timeIntervals.Intervals = append(timeIntervals.Intervals[:idx],
					append([]TimeInterval{interval}, timeIntervals.Intervals[idx:]...)...)
				break
			}
		}
		if interval.Start >= timeIntervals.Intervals[timeIntervals.numIntervals-1].End {
			timeIntervals.Intervals = append(timeIntervals.Intervals, interval)
		}
	}
	timeIntervals.numIntervals++
}

func (timeIntervals *GoogleMapsTimeIntervals) NumIntervals() int{
	return timeIntervals.numIntervals
}

func ParseTimeInterval(openingHour string) (interval TimeInterval){
	closed, _ := regexp.Match(`Closed`, []byte(openingHour))
	if closed{
		interval.Start = 255
		interval.End = 255
		return
	}
	hour_re := regexp.MustCompile(`[\d]{2}:[\d]{2}`)
	hours := hour_re.FindAll([]byte(openingHour), -1)

	am_pm_re := regexp.MustCompile(`[AP]M`)
	am_pm := am_pm_re.FindAll([]byte(openingHour), -1)

	interval.Start = Hour(calculateHour(string(hours[0]), string(am_pm[0])))
	interval.End = Hour(calculateHour(string(hours[1]), string(am_pm[1])))

	return
}

func calculateHour(time string, am_pm string) uint8{
	t := strings.Split(time, ":")
	hour, err := strconv.ParseUint(t[0], 10, 8)
	utils.CheckErr(err)

	if am_pm == "AM"{
		return uint8(hour)
	} else{
		return uint8(hour) + 12
	}
}
