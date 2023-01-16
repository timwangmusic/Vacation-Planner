package planner

import (
	"container/heap"
	"testing"
)

func TestMinPriorityQueue(t *testing.T) {
	pq := &MinPriorityQueue{}

	nyc := Vertex{Location: Point{Lat: 40.712776, Lng: -74.005974}, Name: "New York"}
	la := Vertex{Location: Point{Lat: 34.052235, Lng: -118.243683}, Name: "Los Angeles"}
	lv := Vertex{Location: Point{Lat: 36.169941, Lng: -115.139832}, Name: "Las Vegas"}
	pitt := Vertex{Location: Point{Lat: 40.440624, Lng: -79.995888}, Name: "Pittsburgh"}
	boston := Vertex{Location: Point{Lat: 42.360081, Lng: -71.058884}, Name: "Boston"}

	lv.Key = lv.Dist(pitt)
	nyc.Key = nyc.Dist(pitt)
	boston.Key = boston.Dist(pitt)
	la.Key = la.Dist(pitt)

	cities := []Vertex{pitt, lv, nyc, boston, la}

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
		cur := heap.Pop(pq).(Vertex).Name
		if cur != expected[idx] {
			t.Errorf("Priority sequence error. At index: %d, expected: %s, got: %s", idx, expected[idx], cur)
		}
		idx++
	}
}
