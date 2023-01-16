package planner

type MinPriorityQueue []Vertex

func (pq MinPriorityQueue) Len() int {
	return len(pq)
}

func (pq MinPriorityQueue) Less(i, j int) bool {
	return pq[i].Key <= pq[j].Key
}

func (pq MinPriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

func (pq *MinPriorityQueue) Push(item interface{}) {
	*pq = append(*pq, item.(Vertex))
}

func (pq *MinPriorityQueue) Pop() interface{} {
	prev := *pq
	n := len(prev)
	res := prev[n-1]
	*pq = prev[0 : n-1]
	return res
}
