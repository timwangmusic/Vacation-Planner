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
	Root *Vertex
}

// Construct mininum spanning tree from given nodes
// assume that vertex names are distinct
// initialize key of the nodes to positive inf
func (tree *MinSpanningTree) Construct(nodes []*Vertex) map[string]*Vertex {
	N := len(nodes)
	addedNodes := make(map[string]int)  // used as a Set to check if nodes are added
	nodeMap := make(map[string]*Vertex) // maps name to Vertex pointer

	queue := MinPriorityQueue{Nodes: make([]Vertex, 0), Size: 0}

	for _, node := range nodes {
		name := node.Name
		nodeMap[name] = node
		node.Key = math.Inf(1) // positive infinity
	}

	// set distance of root as zero
	tree.Root.Key = 0.0
	queue.Insert(*tree.Root)

	for len(addedNodes) < N {
		curVertexName := queue.ExtractMin() // get current node name
		curNode := nodeMap[curVertexName]
		fmt.Println("cur node: ", curVertexName)
		addedNodes[curVertexName] = 1 // add current node
		for _, w := range curNode.Neighbors {
			fmt.Println("neighbor:", w.Name)
			dw := curNode.Dist(w)
			neighborName := w.Name
			if _, existed := addedNodes[neighborName]; existed {
				continue
			}
			p := nodeMap[neighborName]
			if dw < p.Key {
				p.Parent = curVertexName
				p.Key = dw
				queue.Insert(*p)
			}
		}
		fmt.Println("node map: ", addedNodes)
	}

	// for _, node := range nodeMap {
	// 	if node.parent != nil {
	// 		fmt.Println(node.Self.Name, node.parent.Name)
	// 	}

	// }

	return nodeMap
}
