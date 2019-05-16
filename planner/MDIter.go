package planner

type MDtagIter struct {
	Tag    string
	Status []int
	Size   []int
}

func (this *MDtagIter) Init(tag string, cplaces CategorizedPlaces) bool {
	sizeE := len(cplaces.EateryPlaces)
	sizeV := len(cplaces.VisitPlaces)
	if tag == "" {
		return false
	}
	this.Tag = tag
	this.Status = make([]int, len(tag))
	this.Size = make([]int, len(tag))
	for pos, char := range tag {
		this.Status[pos] = 0
		if char =='E' || char == 'e' {
			this.Size[pos] =  sizeE
		} else if char =='V' || char == 'v'{
			this.Size[pos] = sizeV
		}
		if this.Size[pos] == 0 {
			return false
		}
	}
	return true
}
func (iterator *MDtagIter) HasNext() bool {
	for i, _ := range iterator.Tag {
		if iterator.Status[i] < iterator.Size[i] - 1 {
			return true
		}
	}
	return false
}

func (this *MDtagIter) Next() bool{
	l := len(this.Tag)
	return this.plusone(l-1)
}
func (this *MDtagIter) plusone(l int) bool {
	if l < 0 {
		//log fault
		return false
	}
	if this.Status[l] + 1 == this.Size[l] {
		this.Status[l] = 0
		return this.plusone(l-1)
	} else {
		this.Status[l] += 1
		return true
	}
}

func (this *MDtagIter) Reset()  {
	for i := range this.Tag {
		this.Status[i] = 0
	}
}
