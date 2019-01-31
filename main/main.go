package main

import (
	"Vacation-planner/constants/city"
	"Vacation-planner/graph"
	"fmt"
)

func main() {
	fmt.Println("welcome to use the Vacation Planner")

	// San Franisco
	l1 := graph.Point{Lat: 37.773972, Long: -122.431297}
	sf := graph.Vertex{Name: "San Francisco", Location: l1}

	// San Diego
	l2 := graph.Point{Lat: 32.715736, Long: -117.161087}
	sd := graph.Vertex{Name: "San Diego", Location: l2}

	// fmt.Println("the distance between sd and sf is:", sf.Dist(&sd), "meters")

	locations := city.GetLocations()

	l3 := locations["Los Angeles"]
	la := graph.Vertex{Location: l3, Name: "Los Angeles"}

	// fmt.Println("the distance between la and sd is:", la.Dist(&sd))

	sf.Key = sf.Dist(&sd)
	sd.Key = sd.Dist(&sd)
	la.Key = la.Dist(&sd)

	nodes := []*graph.Vertex{&sf, &sd, &la}
	// nodes := []graph.Vertex{sf, sd}

	// fmt.Println("after processing...")

	graph.GenerateGraph(nodes)

	for _, node := range nodes {
		fmt.Println(node.Neighbors[0].Name)
	}

	mst := graph.MinSpanningTree{Root: &sf}
	m := mst.Construct(nodes)
	for k, v := range m {
		fmt.Println(k, v.Parent)
	}
}

func testPriorityQueue(nodes []graph.Vertex) {
	q := graph.MinPriorityQueue{Nodes: make([]graph.Vertex, 0), Size: 0}
	for _, node := range nodes {
		q.Insert(node)
	}

	for i := 0; i < 3; i++ {
		cur := q.ExtractMin()
		fmt.Println(cur)
	}
}
