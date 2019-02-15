/*
	Minimum priority queue with Key as non-negative integer and value as Vertex.
	Interfaces:
		Insert(Vertex) void

		ExtractTop() Vertex
*/

package graph

// PriorityQueue defines a general interface for all sorts of priority queue with
// different definitions, sharing the same methods.
type PriorityQueue interface {
	Insert(Vertex)
	ExtractTop() string
	Size() int
	GetRoot() Vertex
}


// MinPriorityQueue defines struct for min-priorityQueue
type MinPriorityQueue struct {
	Nodes []Vertex
	size  int
}

func (h* MinPriorityQueue) GetRoot() Vertex{
	return h.Nodes[0]
}

func (h *MinPriorityQueue) Size() int{
	return h.size
}

// Insert is a method for inserting a Node to priority queue and maintaining priority
func (h *MinPriorityQueue) Insert(n Vertex) {
	if n.Key < 0 {
		return
	}
	h.Nodes = append(h.Nodes, n)
	h.size++
	h.percolateUp(h.size - 1)
}

// ExtractTop is a method for min-priorityQueue to find element with minimum Key
// Returns node with minimum Key. If queue is empty, returns a fake node with Key = -1
func (h *MinPriorityQueue) ExtractTop() string {
	if h.size == 0 {
		return ""
	}
	last := h.size - 1
	res := h.Nodes[0].Name
	h.Nodes[0] = h.Nodes[last]

	// remove last Node
	h.Nodes = h.Nodes[:last]
	h.size--

	h.percolateDown(0)
	return res
}

func (h *MinPriorityQueue) findChildrenIndex(idx int) int {
	leftIdx := idx*2 + 1
	rightIdx := idx*2 + 1
	if leftIdx < h.size && h.Nodes[leftIdx].Key < h.Nodes[idx].Key {
		return leftIdx
	}
	if (rightIdx) < h.size && h.Nodes[rightIdx].Key < h.Nodes[idx].Key {
		return rightIdx
	}
	return -1
}

// percolate down performs heapify operation recursively for each sub-tree
func (h *MinPriorityQueue) percolateDown(idx int) {
	childIdx := h.findChildrenIndex(idx)
	if childIdx == -1 {
		// leaf Node
		return
	}

	// swap with children Node
	swap(h, idx, childIdx)

	// recursion
	h.percolateDown(childIdx)

}

// percolate up the inserted Node
func (h *MinPriorityQueue) percolateUp(idx int) {
	parent := findParent(idx)
	if parent == -1 {	// root
		return
	}
	if h.Nodes[parent].Key > h.Nodes[idx].Key {
		swap(h, parent, idx)
		h.percolateUp(parent)
	}
}

func findParent(idx int) int {
	if idx == 0 {
		return -1
	}
	return idx / 2
}

// swap Nodes at index x and y
func swap(h *MinPriorityQueue, x, y int) {
	temp := h.Nodes[x]
	h.Nodes[x] = h.Nodes[y]
	h.Nodes[y] = temp
}
