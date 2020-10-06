package solution

import (
	log "github.com/sirupsen/logrus"
	"strings"
)

type MDtagIter struct {
	Tag    string
	Status []int
	Size   []int
}

func (mdTagIter *MDtagIter) Init(tag string, categorizedPlaces []CategorizedPlaces) bool {
	if tag == "" {
		return false
	}

	if len(tag) != len(categorizedPlaces) {
		log.Printf("tag length is different from categorized places")
		return false
	}
	tag = strings.ToLower(tag)

	mdTagIter.Tag = tag
	mdTagIter.Status = make([]int, len(tag))
	mdTagIter.Size = make([]int, len(tag))
	for pos, char := range tag {
		mdTagIter.Status[pos] = 0
		if char == 'e' {
			mdTagIter.Size[pos] = len(categorizedPlaces[pos].EateryPlaces)
		} else if char == 'v' {
			mdTagIter.Size[pos] = len(categorizedPlaces[pos].VisitPlaces)
		}
		if mdTagIter.Size[pos] == 0 {
			if char == 'e' {
				log.Debugf("number of places for category eatery is 0, tag index is %d \n", pos)
			} else if char == 'v' {
				log.Debugf("number of places for category visit is 0, tag index is %d \n", pos)
			}
			return false
		}
	}
	return true
}

func (mdTagIter *MDtagIter) HasNext() bool {
	for i := range mdTagIter.Tag {
		if mdTagIter.Status[i] < mdTagIter.Size[i]-1 {
			return true
		}
	}
	return false
}

func (mdTagIter *MDtagIter) Next() bool {
	l := len(mdTagIter.Tag)
	return mdTagIter.plusOne(l - 1)
}

func (mdTagIter *MDtagIter) plusOne(l int) bool {
	if l < 0 {
		//log fault
		return false
	}
	if mdTagIter.Status[l]+1 == mdTagIter.Size[l] {
		mdTagIter.Status[l] = 0
		return mdTagIter.plusOne(l - 1)
	} else {
		mdTagIter.Status[l] += 1
		return true
	}
}

func (mdTagIter *MDtagIter) ClearStatus() {
	for i := range mdTagIter.Tag {
		mdTagIter.Status[i] = 0
	}
}
