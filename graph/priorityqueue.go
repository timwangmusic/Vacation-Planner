/*
	Minimum priority queue with non-negative integer as Key and graph.Vertex as Value.
	Interfaces:
		Insert(Vertex) void

		ExtractTop() Vertex

		Size() int

		GetRoot() Vertex
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

func (h *MinPriorityQueue) GetRoot() Vertex {
	if h.size == 0 {
		return Vertex{}
	}
	return h.Nodes[0]
}

func (h *MinPriorityQueue) Size() int {
	return h.size
}

// Insert is a method for inserting a Node to priority queue and maintaining priority
// Node name should not be an empty string
func (h *MinPriorityQueue) Insert(n Vertex) {
	h.Nodes = append(h.Nodes, n)
	h.size++
	h.percolateUp(h.size - 1)
}

// ExtractTop is a method for min-priorityQueue to find element with minimum Key
// Returns node with minimum Key. If queue is empty, returns empty string as fake node name
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

func (h *MinPriorityQueue) findIndexLargestChild(idx int) int {
	leftIdx := idx*2 + 1
	rightIdx := idx*2 + 2
	if leftIdx >= h.size || h.Nodes[leftIdx].Key > h.Nodes[idx].Key {
		leftIdx = -1
	}
	if rightIdx >= h.size || h.Nodes[rightIdx].Key > h.Nodes[idx].Key {
		rightIdx = -1
	}
	if leftIdx >= 0 && rightIdx >= 0 {
		if h.Nodes[leftIdx].Key < h.Nodes[rightIdx].Key {
			return leftIdx
		}
		return rightIdx
	} else if leftIdx >= 0 {
		return leftIdx
	} else if rightIdx >= 0 {
		return rightIdx
	}
	return -1
}

// percolate down performs heapify operation recursively for each sub-tree
func (h *MinPriorityQueue) percolateDown(idx int) {
	childIdx := h.findIndexLargestChild(idx)
	if childIdx == -1 {
		// leaf Node or no need to swap
		return
	}

	// swap with children Node
	swap(h, idx, childIdx)
	// recursion
	h.percolateDown(childIdx)
}

// percolateUp adjusts the inserted Node
func (h *MinPriorityQueue) percolateUp(idx int) {
	parent := findParent(idx)
	if parent == -1 { // root
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
	return (idx - 1) / 2
}

// swap Nodes at index x and y
func swap(h *MinPriorityQueue, x, y int) {
	temp := h.Nodes[x]
	h.Nodes[x] = h.Nodes[y]
	h.Nodes[y] = temp
}
