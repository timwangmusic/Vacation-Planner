package city

import (
	"Vacation-planner/graph"
	"Vacation-planner/utils"
	"fmt"
)

// GetLocations return location constant for cities
func GetLocations() map[string]graph.Point {
	SanFrancisco := graph.Point{Lat: 37.773972, Long: -122.431297}
	SanDiego := graph.Point{Lat: 32.715736, Long: -117.161087}
	LosAngeles := graph.Point{Lat: 34.052235, Long: -118.243683}
	LasVagas := graph.Point{Lat: 36.169941, Long: -115.139832}
	BuenosAires := graph.Point{Lat: -34.603683, Long: -58.381557}

	var locations map[string]graph.Point

	locations = make(map[string]graph.Point)

	locations["San Francisco"] = SanFrancisco
	locations["San Diego"] = SanDiego
	locations["Los Angeles"] = LosAngeles
	locations["Las Vagas"] = LasVagas
	locations["Brenos Aires"] = BuenosAires

	return locations
}

// convert an object of type location to a slice of string
func locationToStringSlice(name string, point graph.Point) []string {
	res := make([]string, 3)
	res[0] = name
	res[1] = fmt.Sprintf("%f", point.Lat)
	res[2] = fmt.Sprintf("%f", point.Long)
	return res
}

// WriteLocationsCsv writes the whole point collection to csv file
func WriteLocationsCsv(locations map[string]graph.Point) {
	records := make([][]string, 0)
	for name, location := range locations {
		records = append(records, locationToStringSlice(name, location))
	}
	utils.WriteCsv(records)
}
