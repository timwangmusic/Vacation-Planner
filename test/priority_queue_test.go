package test

import (
	"Vacation-Planner/graph"
	"testing"
)


// Set an origin and use the distance from origin as key.
// Verify sequence of the output is same as expected.
func TestMinPriorityQueue(t *testing.T){
	pq := graph.MinPriorityQueue{}

	nyc := graph.Vertex{Location:graph.Point{Lat:40.712776, Lng:-74.005974}, Name: "New York"}
	la := graph.Vertex{Location: graph.Point{Lat: 34.052235, Lng: -118.243683}, Name: "Los Angeles"}
	lv := graph.Vertex{Location: graph.Point{Lat: 36.169941, Lng: -115.139832}, Name: "Las Vegas"}
	pitt := graph.Vertex{Location: graph.Point{Lat:40.440624, Lng: -79.995888}, Name: "Pittsburgh"}
	boston := graph.Vertex{Location: graph.Point{Lat:42.360081, Lng:-71.058884}, Name: "Boston"}

	lv.Key = lv.Dist(pitt)
	nyc.Key = nyc.Dist(pitt)
	boston.Key = boston.Dist(pitt)
	la.Key = la.Dist(pitt)

	cities := []*graph.Vertex{&pitt, &lv, &nyc, &boston, &la}

	for _, city := range cities{
		pq.Insert(*city)
	}

	expected := []string{
		pitt.Name,
		nyc.Name,
		boston.Name,
		lv.Name,
		la.Name,
	}

	var idx = 0
	for pq.Size() > 0{
		cur := pq.ExtractTop()
		if cur != expected[idx]{
			t.Errorf("Priority sequence error. At index: %d, expected: %s, got: %s", idx, expected[idx], cur)
		}
		idx++
	}

}
