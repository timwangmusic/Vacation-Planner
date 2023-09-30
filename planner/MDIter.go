package planner

import (
	"errors"

	log "github.com/sirupsen/logrus"
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/matching"
)

type MultiDimIterator struct {
	Categories []POI.PlaceCategory
	Status     []int
	Stop       bool  // indicate iterator is stopped
	Size       []int // number of items in each category
}

func (mdTagIter *MultiDimIterator) Init(categories []POI.PlaceCategory, placeClusters [][]matching.Place) error {
	var err error
	if len(categories) != len(placeClusters) {
		err = errors.New(CategorizedPlaceIterInitFailureErrMsg)
		log.Error("place category list length is different from the number of place clusters")
		return err
	}

	mdTagIter.Categories = categories
	mdTagIter.Status = make([]int, len(mdTagIter.Categories))
	mdTagIter.Size = make([]int, len(mdTagIter.Categories))
	for pos, category := range mdTagIter.Categories {
		// Status is initialized as an all-zero vector
		mdTagIter.Status[pos] = 0
		mdTagIter.Size[pos] = len(placeClusters[pos])

		if mdTagIter.Size[pos] == 0 {
			log.Errorf("the number of places for category %s in slot index %d is 0\n", category, pos)
			return errors.New(CategorizedPlaceIterInitFailureErrMsg)
		}
	}
	return err
}

func (mdTagIter *MultiDimIterator) HasNext() bool {
	var hasNext = !mdTagIter.Stop
	mdTagIter.updateStop()
	return hasNext
}

func (mdTagIter *MultiDimIterator) updateStop() {
	// stop if every dim in status[] has reached its last point
	var stop = true
	for i := range mdTagIter.Categories {
		stop = stop && (mdTagIter.Status[i] == mdTagIter.Size[i]-1)
	}
	mdTagIter.Stop = stop
}

func (mdTagIter *MultiDimIterator) Next() bool {
	l := len(mdTagIter.Categories)
	return mdTagIter.plusOne(l - 1)
}

func (mdTagIter *MultiDimIterator) plusOne(l int) bool {
	if l < 0 {
		//log fault
		return false
	}
	if mdTagIter.Status[l]+1 == mdTagIter.Size[l] {
		mdTagIter.Status[l] = 0
		return mdTagIter.plusOne(l - 1)
	}
	mdTagIter.Status[l]++
	return true
}

func (mdTagIter *MultiDimIterator) ClearStatus() {
	for i := range mdTagIter.Categories {
		mdTagIter.Status[i] = 0
	}
}
