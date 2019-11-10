package graph

type MinPriorityQueueVertex []Vertex

func (pq MinPriorityQueueVertex) Len() int {
	return len(pq)
}

func (pq MinPriorityQueueVertex) Less(i, j int) bool {
	return pq[i].Key <= pq[j].Key
}

func (pq MinPriorityQueueVertex) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

func (pq *MinPriorityQueueVertex) Push(item interface{}) {
	*pq = append(*pq, item.(Vertex))
}

func (pq *MinPriorityQueueVertex) Pop() interface{} {
	prev := *pq
	n := len(prev)
	res := prev[n-1]
	*pq = prev[0 : n-1]
	return res
}
