package solution

import (
	"errors"
	log "github.com/sirupsen/logrus"
	"github.com/weihesdlegend/Vacation-planner/POI"
)

type MultiDimIterator struct {
	Categories []POI.PlaceCategory
	Status     []int
	Size       []int // number of items in each category
}

func (mdTagIter *MultiDimIterator) Init(categories []POI.PlaceCategory, categorizedPlaces []CategorizedPlaces) error {
	var err error
	if len(categories) != len(categorizedPlaces) {
		err = errors.New(CategorizedPlaceIterInitFailureErrMsg)
		log.Error("place category list length is different from categorized places")
		return err
	}

	mdTagIter.Categories = categories
	mdTagIter.Status = make([]int, len(mdTagIter.Categories))
	mdTagIter.Size = make([]int, len(mdTagIter.Categories))
	for pos, category := range mdTagIter.Categories {
		mdTagIter.Status[pos] = 0
		if category == POI.PlaceCategoryEatery {
			mdTagIter.Size[pos] = len(categorizedPlaces[pos].EateryPlaces)
		} else if category == POI.PlaceCategoryVisit {
			mdTagIter.Size[pos] = len(categorizedPlaces[pos].VisitPlaces)
		}
		if mdTagIter.Size[pos] == 0 {
			if category == POI.PlaceCategoryEatery {
				log.Errorf("number of places for category eatery is 0, tag index is %d \n", pos)
				err = errors.New(CategorizedPlaceIterInitFailureErrMsg)
			} else if category == POI.PlaceCategoryVisit {
				log.Errorf("number of places for category visit is 0, tag index is %d \n", pos)
				err = errors.New(CategorizedPlaceIterInitFailureErrMsg)
			}
		}
	}
	return err
}

func (mdTagIter *MultiDimIterator) HasNext() bool {
	for i := range mdTagIter.Categories {
		if mdTagIter.Status[i] < mdTagIter.Size[i]-1 {
			return true
		}
	}
	return false
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
