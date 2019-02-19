package test

import (
	"Vacation-Planner/graph"
	"testing"
)


// Set an origin and use the distance from origin as key.
// Verify sequence of the output is same as expected.
func TestMinPriorityQueue(t *testing.T){
	pq := graph.MinPriorityQueue{}

	nyc := graph.Vertex{Location:graph.Point{Lat:40.712776, Long:-74.005974}, Name: "New York"}
	la := graph.Vertex{Location: graph.Point{Lat: 34.052235, Long: -118.243683}, Name: "Los Angeles"}
	lv := graph.Vertex{Location: graph.Point{Lat: 36.169941, Long: -115.139832}, Name: "Las Vegas"}
	pitt := graph.Vertex{Location: graph.Point{Lat:40.440624, Long: -79.995888}, Name: "Pittsburgh"}

	origin := nyc

	// pass by value
	cities := []graph.Vertex{nyc, la, lv, pitt}

	for _, city := range cities{
		city.Key = origin.Dist(city)
		pq.Insert(city)
	}

	expected := []string{nyc.Name,
		pitt.Name,
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
