/*
	Define struct and method for implementing Prim's algorithm

	Input: connected undirected graph consisted of a list of Vertexes

	Output: minimum spanning tree
*/

package graph

import (
	"bytes"
	"log"
	"math"
	"strings"
)

// MinSpanningTree describes data members and methods for minimum spanning tree
type MinSpanningTree struct {
	Root *Vertex
}

// A simplified implementation
type treeNode struct {
	name     string
	Children []string
}

// PreOrderTraversal pre-order traversal of the min spanning tree
func (tree *MinSpanningTree) PreOrderTraversal(nodes map[string]*Vertex) string {
	m := findChildren(nodes)

	return "Our suggested path to visit all places is: " +
		strings.Trim(dfs(m[tree.Root.Name], m), "->")
}

func dfs(subtree *treeNode, m map[string]*treeNode) string {
	if len(subtree.Children) == 0 {
		return subtree.name
	}
	var b bytes.Buffer

	b.WriteString(subtree.name + "->")
	for _, child := range subtree.Children {
		b.WriteString(dfs(m[child], m) + "->")
	}

	return b.String()
}

// Interestingly, had I made the key type of the returned map as treeNode instead of pointer
// the logic will not work since map does not allow modify its slice value if it is not pointer
func findChildren(m map[string]*Vertex) map[string]*treeNode {
	res := make(map[string]*treeNode, 0)
	for name, node := range m {
		if name != node.Name {
			log.Fatalln("name error")
		}
		_, exist := res[name]
		if !exist {
			res[name] = &treeNode{name: name, Children: make([]string, 0)}
		}
		parent := node.Parent
		if parent == ""{
			continue
		}
		_, exist = res[parent]
		if !exist {
			res[parent] = &treeNode{name: parent, Children: make([]string, 0)}
		}
		tmp := res[parent].Children
		tmp = append(tmp, name)
		res[parent].Children = tmp
	}
	return res
}

// Construct minimum spanning tree from given nodes
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

	tree.Root.Key = 0.0		// set distance of root as zero
	queue.Insert(*tree.Root)

	for len(addedNodes) < N {
		curVertexName := queue.ExtractMin() // get current node name
		curNode := nodeMap[curVertexName]
		addedNodes[curVertexName] = 1 // add current node
		for _, w := range curNode.Neighbors {
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
	}
	return nodeMap
}
