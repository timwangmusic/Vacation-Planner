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

func (this *MDtagIter) Init(tag string, categorizedPlaces []CategorizedPlaces) bool {
	if tag == "" {
		return false
	}

	if len(tag) != len(categorizedPlaces) {
		log.Printf("tag length is different from categorized places")
		return false
	}
	tag = strings.ToLower(tag)

	this.Tag = tag
	this.Status = make([]int, len(tag))
	this.Size = make([]int, len(tag))
	for pos, char := range tag {
		this.Status[pos] = 0
		if char == 'e' {
			this.Size[pos] = len(categorizedPlaces[pos].EateryPlaces)
		} else if char == 'v' {
			this.Size[pos] = len(categorizedPlaces[pos].VisitPlaces)
		}
		if this.Size[pos] == 0 {
			if char == 'e' {
				log.Infof("number of places for category eatery is 0, tag index is %d \n", pos)
			} else if char == 'v' {
				log.Infof("number of places for category visit is 0, tag index is %d \n", pos)
			}
			return false
		}
	}
	return true
}

func (iterator *MDtagIter) HasNext() bool {
	for i, _ := range iterator.Tag {
		if iterator.Status[i] < iterator.Size[i]-1 {
			return true
		}
	}
	return false
}

func (this *MDtagIter) Next() bool {
	l := len(this.Tag)
	return this.plusone(l - 1)
}

func (this *MDtagIter) plusone(l int) bool {
	if l < 0 {
		//log fault
		return false
	}
	if this.Status[l]+1 == this.Size[l] {
		this.Status[l] = 0
		return this.plusone(l - 1)
	} else {
		this.Status[l] += 1
		return true
	}
}

func (this *MDtagIter) Reset() {
	for i := range this.Tag {
		this.Status[i] = 0
	}
}
