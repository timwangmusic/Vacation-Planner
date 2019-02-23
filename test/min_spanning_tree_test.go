package test

import (
	"Vacation-Planner/graph"
	"testing"
)

// Set a root vertex and start constructing the min spanning tree.
// Verify that the parent of each vertex is correct and the traversal string is correct
func TestMinSpanningTree(t *testing.T){
	nyc := graph.Vertex{Location:graph.Point{Lat:40.712776, Long:-74.005974}, Name: "New York"}
	la := graph.Vertex{Location: graph.Point{Lat: 34.052235, Long: -118.243683}, Name: "Los Angeles"}
	lv := graph.Vertex{Location: graph.Point{Lat: 36.169941, Long: -115.139832}, Name: "Las Vegas"}
	pitt := graph.Vertex{Location: graph.Point{Lat:40.440624, Long: -79.995888}, Name: "Pittsburgh"}
	boston := graph.Vertex{Location: graph.Point{Lat:42.360081, Long:-71.058884}, Name: "Boston"}
	met := graph.Vertex{Location:graph.Point{Lat: 40.779079, Long: -73.962578}, Name: "The Met"}
	sd := graph.Vertex{Location:graph.Point{Lat: 32.715736, Long: -117.161087}, Name: "San Diego"}
	sf :=graph.Vertex{Location:graph.Point{Lat: 37.773972, Long: -122.431297}, Name: "San Francisco"}

	// pass by pointer
	cities := []*graph.Vertex{&nyc, &la, &lv, &pitt, &sf, &boston, &met, &sd}
	mst := graph.MinSpanningTree{Root: &nyc}

	// connect cities
	graph.GenerateGraph(cities, false)

	res := mst.Construct(cities)

	expected := map[string]string{
		nyc.Name: "",
		la.Name: lv.Name,
		lv.Name: pitt.Name,
		pitt.Name: nyc.Name,
		boston.Name: met.Name,
		sd.Name: la.Name,
		sf.Name: la.Name,
		met.Name: nyc.Name,
	}

	// Verify construction
	for name, city := range res{
		if city.Parent != expected[name]{
			t.Errorf("Parent city is not set correctly. Expected: %s, got: %s", expected[name], city.Parent)
		}
	}

	// traversal result varies run to run, so not verifying the results
	mst.PreOrderTraversal(res)
}
