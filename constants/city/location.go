package city

import (
	"Vacation-planner/graph"
	"Vacation-planner/utils"
	"fmt"
	"strconv"
)

// GetLocationsFromCsv...
// Current format specifies that each line contain 3 fields: location name, latitude, longitude
func GetLocationsFromCsv(filename string) map[string]graph.Point{
	locations := make(map[string]graph.Point, 0)
	locationsData := utils.ReadCsv(filename)
	for _, location := range locationsData{
		name := location[0]
		lat, _ := strconv.ParseFloat(location[1], 64)
		lng, _ := strconv.ParseFloat(location[2], 64)
		_, exist := locations[name]
		if !exist{	// dedupe
			locations[name] = graph.Point{Lat:lat, Long:lng}
		}
	}
	return locations
}

// WriteLocationsToCsv writes the whole point collection to csv file
func WriteLocationsToCsv(filename string, locations map[string]graph.Point) {
	records := make([][]string, 0)
	for name, location := range locations {
		records = append(records, locationToStringSlice(name, location))
	}
	utils.WriteCsv(filename, records)
}

// convert an object of type location to a slice of string
func locationToStringSlice(name string, point graph.Point) []string {
	res := make([]string, 3)
	res[0] = name
	res[1] = fmt.Sprintf("%f", point.Lat)
	res[2] = fmt.Sprintf("%f", point.Long)
	return res
}
