/*
	Mininum priority queue with Key as non-negative integer and value as Vertex.
	Interfaces:
		Insert(Vertex) void

		ExtractMin() Vertex
*/

package graph

// MinPriorityQueue defines struct for min-priorityQueue
type MinPriorityQueue struct {
	Nodes []Vertex
	Size  int
}

// Insert is a method for inserting a Node to priority queue and maintaining priority
func (h *MinPriorityQueue) Insert(n Vertex) {
	if n.Key < 0 {
		return
	}
	h.Nodes = append(h.Nodes, n)
	h.Size++
	h.perculateUp(h.Size - 1)
}

// ExtractMin is a method for min-priorityQueue to find element with minimum Key
// Returns node with minimum Key. If queue is empty, returns a fake node with Key = -1
func (h *MinPriorityQueue) ExtractMin() string {
	if h.Size == 0 {
		return ""
	}
	last := h.Size - 1
	res := h.Nodes[0].Name
	h.Nodes[0] = h.Nodes[last]

	// remove last Node
	h.Nodes = h.Nodes[:last]
	h.Size--

	h.perculateDown(0)
	return res
}

func (h *MinPriorityQueue) findChildrenIndex(idx int) int {
	leftIdx := idx*2 + 1
	rightIdx := idx*2 + 1
	if leftIdx < h.Size && h.Nodes[leftIdx].Key < h.Nodes[idx].Key {
		return leftIdx
	}
	if (rightIdx) < h.Size && h.Nodes[rightIdx].Key < h.Nodes[idx].Key {
		return rightIdx
	}
	return -1
}

// perculate down performs heapify operation recursively for each sub-tree
func (h *MinPriorityQueue) perculateDown(idx int) {
	childIdx := h.findChildrenIndex(idx)
	if childIdx == -1 {
		// leaf Node
		return
	}

	// swap with children Node
	swap(h, idx, childIdx)

	// recursion
	h.perculateDown(childIdx)

}

// perculate up the inserted Node
func (h *MinPriorityQueue) perculateUp(idx int) {
	parent := findParent(idx)
	if parent == -1 {
		// root
		return
	}
	if h.Nodes[parent].Key > h.Nodes[idx].Key {
		swap(h, parent, idx)
		h.perculateUp(parent)
	}
}

func findParent(idx int) int {
	if idx == 0 {
		return -1
	}
	return idx / 2
}

func swap(h *MinPriorityQueue, x, y int) {
	// swap Nodes at index x and y
	temp := h.Nodes[x]
	h.Nodes[x] = h.Nodes[y]
	h.Nodes[y] = temp
}
