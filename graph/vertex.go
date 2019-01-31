package graph

import "math"

// Vertex defines a struct type for basic vertexes
type Vertex struct {
	Key       float64   // vertex key
	Name      string    // vertex name
	Degree    int       // vertex degree
	Neighbors []*Vertex // neighbor list
	Location  Point
	Parent    string
}

// Edge defines a struct type for basic edges in a graph
type edge struct {
	V Vertex
	W Vertex
}

// Point defines a location with latitude and longtitude
type Point struct {
	Lat  float64
	Long float64
}

// Dist calculates distance between two Vertexes
func (v *Vertex) Dist(neighbor *Vertex) float64 {
	return v.Location.dist(neighbor.Location)
}

func hav(theta float64) float64 {
	return (1 - math.Cos(theta)) / 2
}

func (p Point) dist(n Point) float64 {
	// reference: Wikipedia of haversine
	var lat1, long1, lat2, long2 float64 // latitudes and longtitudes in radius
	lat1 = p.Lat * math.Pi / 180
	long1 = p.Long * math.Pi / 180
	lat2 = n.Lat * math.Pi / 180
	long2 = n.Long * math.Pi / 180

	// radius of earch in meters
	rEearth := 6378100.0

	// calculate haversine of central angle of the given two points
	h := hav(lat2-lat1) + math.Cos(lat2)*math.Cos(lat1)*hav(long2-long1)

	return math.Asin(math.Sqrt(h)) * rEearth * 2
}
