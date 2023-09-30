package planner

type PriorityQueueItem interface {
	Key() float64
}

type Vertex struct {
	K      float64 // key or priority
	Name   string  // vertex name
	Object interface{}
}

func (v Vertex) Key() float64 {
	return v.K
}
