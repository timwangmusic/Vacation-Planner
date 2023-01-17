package planner

import (
	"container/heap"
	"testing"
)

func TestMinPriorityQueue(t *testing.T) {
	pq := &MinPriorityQueue{}

	nyc := Vertex{Key: 5.5, Name: "New York"}
	la := Vertex{Key: 4.4, Name: "Los Angeles"}
	lv := Vertex{Key: 3.3, Name: "Las Vegas"}
	pitt := Vertex{Key: 2.2, Name: "Pittsburgh"}
	boston := Vertex{Key: 1.1, Name: "Boston"}

	cities := []Vertex{pitt, lv, nyc, boston, la}

	for _, city := range cities {
		heap.Push(pq, city)
	}

	heap.Init(pq)

	expected := []string{
		boston.Name,
		pitt.Name,
		lv.Name,
		la.Name,
		nyc.Name,
	}

	var idx = 0
	for pq.Len() > 0 {
		cur := heap.Pop(pq).(Vertex).Name
		if cur != expected[idx] {
			t.Errorf("priority sequence error: at index: %d, expected: %s, got: %s", idx, expected[idx], cur)
		}
		idx++
	}
}
