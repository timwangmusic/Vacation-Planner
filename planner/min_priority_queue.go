package planner

type MinPriorityQueue[T PriorityQueueItem] struct {
	items []T
}

func (pq *MinPriorityQueue[T]) Len() int {
	return len(pq.items)
}

func (pq *MinPriorityQueue[T]) Less(i, j int) bool {
	return pq.items[i].Key() <= pq.items[j].Key()
}

func (pq *MinPriorityQueue[T]) Swap(i, j int) {
	pq.items[i], pq.items[j] = pq.items[j], pq.items[i]
}

func (pq *MinPriorityQueue[T]) Push(item interface{}) {
	pq.items = append(pq.items, item.(T))
}

func (pq *MinPriorityQueue[T]) Pop() interface{} {
	prev := pq.items
	n := len(prev)
	res := prev[n-1]
	pq.items = prev[0 : n-1]
	return res
}
