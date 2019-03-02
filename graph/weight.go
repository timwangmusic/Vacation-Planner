package graph
const(
	PRIORITY_TIME=iota
	PRIORITY_BUDGET
)
type SimpleWeight struct{
	w uint32
}
func (this SimpleWeight) SetWeight(weight uint32){
	this.w = weight
}
func (this SimpleWeight) GetWeight(weight uint32) uint32{
	return this.w
}
func (l SimpleWeight) Compare(r SimpleWeight) bool{
	if l.w <= r.w{
		return true
	} else {
		return false
	}
}

type SimpleBaseWeight struct {
	timeInMin uint32
	budget	float64
	cmpflag	uint8
}
func (this SimpleBaseWeight) SetWeight(timeInMin uint32, budget float64){
	this.timeInMin = timeInMin
	this.budget = budget}
func (this SimpleBaseWeight) GetWeight() (uint32, float64) {
	return this.timeInMin, this.budget
}
func (this SimpleBaseWeight) SetCmpFlag( flag uint8) bool {
	switch flag {
	case PRIORITY_BUDGET:
		this.cmpflag = flag
		return true
	case PRIORITY_TIME:
		this.cmpflag = flag
		return true
	default:
		return false
	}
}
func (this SimpleBaseWeight) GetCmpFlag() uint8 {
	return this.cmpflag
}

/*
FIXME: The priority of configuration is based only on the cmpflag
of the object calling the compare function. This configuration must
be used with care.
 */
func (l SimpleBaseWeight) Compare(r SimpleBaseWeight) bool {
	switch l.cmpflag {
	case PRIORITY_TIME:
		if l.timeInMin <= r.timeInMin {
			return true
		} else {
			return false
		}
	case PRIORITY_BUDGET:
		if l.budget <= r.budget {
			return true
		} else {
			return false
		}
	default:
		/*
		Default behavior favor money
		 */
		if l.budget <= r.budget {
			return true
		} else {
			return false
		}
	}
}
