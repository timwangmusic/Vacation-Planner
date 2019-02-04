package main

import (
	"Vacation-planner/constants/city"
	"Vacation-planner/graph"
	"fmt"
)

func main() {
	fmt.Println("welcome to use the Vacation Planner")

	// San Francisco
	l1 := graph.Point{Lat: 37.773972, Long: -122.431297}
	sf := graph.Vertex{Name: "San Francisco", Location: l1}

	// // San Diego
	l2 := graph.Point{Lat: 32.715736, Long: -117.161087}
	sd := graph.Vertex{Name: "San Diego", Location: l2}

	locations := city.GetLocations()

	l3 := locations["Los Angeles"]
	la := graph.Vertex{Location: l3, Name: "Los Angeles"}

	nodes := []*graph.Vertex{&sf, &sd, &la}

	mst := graph.MinSpanningTree{Root: &sf}

	graph.GenerateGraph(nodes, false)

	m := mst.Construct(nodes)

	fmt.Println(mst.PreOrderTraversal(m))

}

func testMinSpanningTree(nodes []*graph.Vertex, limited bool) {
	graph.GenerateGraph(nodes, limited)

	tree := graph.MinSpanningTree{Root: nodes[0]}
	res := tree.Construct(nodes)
	for k, p := range res {
		fmt.Println(k, p.Parent)
	}
}

func testPriorityQueue(nodes []graph.Vertex) {
	q := graph.MinPriorityQueue{Nodes: make([]graph.Vertex, 0), Size: 0}
	for _, node := range nodes {
		q.Insert(node)
	}

	for i := 0; i < len(nodes); i++ {
		cur := q.ExtractMin() // node name
		fmt.Println(cur)
	}
}
