package test

import (
	"Vacation-Planner/graph"
	"fmt"
	"testing"
)

// Set a root vertex and start constructing the min spanning tree.
// Verify that the parent of each vertex is correct and the traversal string is correct
func TestMinSpanningTree(t *testing.T){
	nyc := graph.Vertex{Location:graph.Point{Lat:40.712776, Long:-74.005974}, Name: "New York"}
	la := graph.Vertex{Location: graph.Point{Lat: 34.052235, Long: -118.243683}, Name: "Los Angeles"}
	lv := graph.Vertex{Location: graph.Point{Lat: 36.169941, Long: -115.139832}, Name: "Las Vegas"}
	pitt := graph.Vertex{Location: graph.Point{Lat:40.440624, Long: -79.995888}, Name: "Pittsburgh"}

	// pass by pointer
	cities := []*graph.Vertex{&nyc, &la, &lv, &pitt}

	mst := graph.MinSpanningTree{Root: &nyc}

	// connect cities
	graph.GenerateGraph(cities, false)

	res := mst.Construct(cities)

	expected := map[string]string{
		nyc.Name: "",
		la.Name: lv.Name,
		lv.Name: pitt.Name,
		pitt.Name: nyc.Name,
	}

	// Verify construction
	for name, city := range res{
		if city.Parent != expected[name]{
			t.Errorf("Parent city is not set correctly. Expected: %s, got: %s", expected[name], city.Parent)
		}
	}

	// Verify traversal result
	traversal := mst.PreOrderTraversal(res)

	expected_traversal := fmt.Sprintf(
		"Our suggested path to visit all places is: %s->%s->%s->%s",
		nyc.Name,
		pitt.Name,
		lv.Name,
		la.Name)

	if traversal != expected_traversal{
		t.Errorf("City traversal is not correct. Expected: %s, got: %s", expected_traversal, traversal)
	}
}
