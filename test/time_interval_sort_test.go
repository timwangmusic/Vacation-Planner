package test

import (
	"github.com/weihesdlegend/Vacation-planner/POI"
	"sort"
	"testing"
)

func TestTimeIntervalSort(t *testing.T) {
	interval1 := POI.TimeInterval{
		Start: 15,
		End:   20,
	}
	interval2 := POI.TimeInterval{
		Start: 13,
		End:   16,
	}
	interval3 := POI.TimeInterval{
		Start: 12,
		End:   20,
	}

	intervals := []POI.TimeInterval{interval1, interval2, interval3}

	sort.Sort(POI.ByStartTime(intervals))

	for idx, interval := range intervals {
		if idx < len(intervals)-1 {
			if interval.Start > intervals[idx+1].Start {
				t.Error("intervals are not sort correctly")
			}
		}
	}
}
