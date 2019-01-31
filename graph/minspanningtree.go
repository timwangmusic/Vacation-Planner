/*
	Define struct and method for implementing Prim's algorithm

	Input: connected undirected graph consisted of a list of Vertexes

	Output: minimum spanning tree
*/

package graph

import (
	"fmt"
	"math"
)

// MinSpanningTree describes data members and methods for minimum spanning tree
type MinSpanningTree struct {
	Root TreeNode
}

// TreeNode ...
type TreeNode struct {
	Self   *Vertex
	parent *Vertex
}

// Init ...
func (tree *MinSpanningTree) Init(rootVertex *TreeNode) {
	tree.Root = *rootVertex
}

// Construct mininum spanning tree from given nodes
// assume that vertex names are distinct
// initialize key of the nodes to positive inf
func (tree *MinSpanningTree) Construct(nodes []*Vertex) map[string]TreeNode {
	N := len(nodes)
	addedNodes := make(map[string]int)   // used as a Set to check if nodes are added
	nodeMap := make(map[string]TreeNode) // maps name to TreeNode

	queue := MinPriorityQueue{Nodes: make([]Vertex, 0), Size: 0}

	for _, node := range nodes {
		name := node.Name
		nodeMap[name] = TreeNode{Self: node}
		node.Key = math.Inf(1) // positive infinity
	}

	// set distance of root as zero
	tree.Root.Self.Key = 0.0
	queue.Insert(*tree.Root.Self)

	for len(addedNodes) < N {
		curVertex := queue.ExtractMin() // get current node
		name := curVertex.Name
		fmt.Println("cur node: ", name)
		addedNodes[name] = 1 // add current node
		for _, w := range curVertex.Neighbors {
			dw := curVertex.Dist(w)
			neighborName := w.Name
			// fmt.Println("neighbor:", neighborName)
			if _, existed := addedNodes[neighborName]; existed {
				continue
			}
			p := nodeMap[neighborName]
			if dw < p.Self.Key {
				p.parent = &curVertex
				p.Self.Key = dw
				queue.Insert(*p.Self)
			}
		}
		fmt.Println("node map: ", addedNodes)
	}

	for _, node := range nodeMap {
		if node.parent != nil {
			fmt.Println(node.Self.Name, node.parent.Name)
		}

	}

	return nodeMap
}
