package city

import "Vacation-planner/graph"

// GetLocations return location constant for cities
func GetLocations() map[string]graph.Point {
	SanFrancisco := graph.Point{Lat: 37.773972, Long: -122.431297}
	SanDiego := graph.Point{Lat: 32.715736, Long: -117.161087}
	LosAngeles := graph.Point{Lat: 34.052235, Long: -118.243683}
	var locations map[string]graph.Point

	locations = make(map[string]graph.Point)

	locations["San Francisco"] = SanFrancisco
	locations["San Diego"] = SanDiego
	locations["Los Angeles"] = LosAngeles

	return locations
}
