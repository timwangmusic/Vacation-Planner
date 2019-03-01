package graph

import (
	"Vacation-planner/utils"
)

// Vertex defines a struct type for basic vertexes
type Vertex struct {
	Key       float64   // vertex key
	Name      string    // vertex name
	Degree    int       // vertex degree
	Neighbors []*Vertex // neighbor list
	Location  Point     // geo-location
	Parent    string    // parent name
}

// Point defines a location with latitude and longtitude
type Point struct {
	Lat float64
	Lng float64
}

// Dist calculates distance between two Vertexes
func (v *Vertex) Dist(neighbor Vertex) float64 {
	return v.Location.dist(neighbor.Location)
}

func (p Point) dist(n Point) float64 {
	return utils.HaversineDist([]float64{p.Lat, p.Lng}, []float64{n.Lat, n.Lng})
}
