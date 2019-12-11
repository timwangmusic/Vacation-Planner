package test

import (
	"container/heap"
	"github.com/weihesdlegend/Vacation-planner/graph"
	"testing"
)

func TestMinPriorityQueueVertex(t *testing.T) {
	pq := &graph.MinPriorityQueueVertex{}

	nyc := graph.Vertex{Location: graph.Point{Lat: 40.712776, Lng: -74.005974}, Name: "New York"}
	la := graph.Vertex{Location: graph.Point{Lat: 34.052235, Lng: -118.243683}, Name: "Los Angeles"}
	lv := graph.Vertex{Location: graph.Point{Lat: 36.169941, Lng: -115.139832}, Name: "Las Vegas"}
	pitt := graph.Vertex{Location: graph.Point{Lat: 40.440624, Lng: -79.995888}, Name: "Pittsburgh"}
	boston := graph.Vertex{Location: graph.Point{Lat: 42.360081, Lng: -71.058884}, Name: "Boston"}

	lv.Key = lv.Dist(pitt)
	nyc.Key = nyc.Dist(pitt)
	boston.Key = boston.Dist(pitt)
	la.Key = la.Dist(pitt)

	cities := []graph.Vertex{pitt, lv, nyc, boston, la}

	for _, city := range cities {
		heap.Push(pq, city)
	}

	heap.Init(pq)

	expected := []string{
		pitt.Name,
		nyc.Name,
		boston.Name,
		lv.Name,
		la.Name,
	}

	var idx = 0
	for pq.Len() > 0 {
		cur := heap.Pop(pq).(graph.Vertex).Name
		if cur != expected[idx] {
			t.Errorf("Priority sequence error. At index: %d, expected: %s, got: %s", idx, expected[idx], cur)
		}
		idx++
	}
}
