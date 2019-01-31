package graph

import (
	"Vacation-Planner/utils"
)

// connect two vertexes
func connect(v *Vertex, w *Vertex) {
	v.Neighbors = append(v.Neighbors, w)
	w.Neighbors = append(w.Neighbors, v)
}

// Generate edges with sparsity that each node is connected with at most
// half of all nodes in the graph
func generateEdges(nodes []*Vertex) {
	N := len(nodes)
	var maxDegree = N / 2
	for i := 0; i < N; i++ {
		for j := i + 1; j < utils.MinInt(N, i+maxDegree+1); j++ {
			connect(nodes[i], nodes[j])
		}
	}
}

// GenerateGraph is entrance of graph generation mechanism
func GenerateGraph(nodes []*Vertex) {
	generateEdges(nodes)
}
