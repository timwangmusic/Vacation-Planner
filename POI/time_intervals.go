package POI

import (
	"errors"
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
