package graph
<<<<<<< Updated upstream
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
	if(l.w <= r.w){
=======
/*
This source provides the polymorphism like compare functions for different kind of weights in the graph
 */
 const(
 	PRIORITY_TIME=iota
 	PRIORITY_BUDGET
 )
type SimpleWeight struct{
	w uint64
}
/*
 type Comparable interface{
 	Compare(r Comparable) bool
 }
 */


func (l SimpleWeight) Compare(r SimpleWeight) bool {
	if l.w <= r.w{
>>>>>>> Stashed changes
		return true
	} else {
		return false
	}
}
<<<<<<< Updated upstream

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
=======
func (v SimpleWeight) SetWeight(weight uint64){
	v.w = weight
}
func (v SimpleWeight) GetWeight(weight uint64) uint64{
	return v.w;
}

type SimpleBaseWeight struct{
	timeInMin uint32
	budget    float64
	/*
	Need to solve cmpflag match problems, make sure cmpflag matches before calling the comparison function
	 */
	cmpflag uint8
}
func (v SimpleBaseWeight) SetWeight(timeInMin uint32, budget float64){
	v.timeInMin = timeInMin
	v.budget = budget
}
func (v SimpleBaseWeight) GetWeight() (uint32, float64){
	return v.timeInMin, v.budget
}
func (v SimpleBaseWeight) Setcmpflag(cmpflag uint8){
	/*
	Need to perform validity check of input values
	will change function signiture to bool after then
	 */
	v.cmpflag = cmpflag
}
func (v SimpleBaseWeight) Getcmpflag() uint8{
	return v.cmpflag
}

func (l SimpleBaseWeight) Compare(r SimpleBaseWeight) bool{
	switch l.cmpflag {
	case PRIORITY_TIME :
		if(l.timeInMin <=r.timeInMin){
>>>>>>> Stashed changes
			return true
		} else {
			return false
		}
<<<<<<< Updated upstream
	case PRIORITY_BUDGET:
		if l.budget <= r.budget {
=======
	case PRIORITY_BUDGET :
		if(l.budget<=r.budget){
>>>>>>> Stashed changes
			return true
		} else {
			return false
		}
	default:
<<<<<<< Updated upstream
		/*
		Default behavior favor money
		 */
		if l.budget <= r.budget {
=======
		if(l.budget<=r.budget){
>>>>>>> Stashed changes
			return true
		} else {
			return false
		}
	}
<<<<<<< Updated upstream
}

=======
	
}
>>>>>>> Stashed changes
