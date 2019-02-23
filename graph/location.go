package graph

import (
	"Vacation-planner/utils"
	"fmt"
	"strconv"
)

// GetLocationsFromCsv...
// Current format specifies that each line contain 3 fields: location name, latitude, longitude
func GetLocationsFromCsv(filename string) map[string]Point{
	locations := make(map[string]Point, 0)
	locationsData := utils.ReadCsv(filename)
	for _, location := range locationsData{
		name := location[0]
		lat, _ := strconv.ParseFloat(location[1], 64)
		lng, _ := strconv.ParseFloat(location[2], 64)
		_, exist := locations[name]
		if !exist{	// dedupe
			locations[name] = Point{Lat:lat, Long:lng}
		}
	}
	return locations
}

// WriteLocationsToCsv writes the whole point collection to csv file
func WriteLocationsToCsv(filename string, locations map[string]Point) {
	records := make([][]string, 0)
	for name, location := range locations {
		records = append(records, locationToStringSlice(name, location))
	}
	utils.WriteCsv(filename, records)
}

// convert an object of type location to a slice of string
func locationToStringSlice(name string, point Point) []string {
	res := make([]string, 3)
	res[0] = name
	res[1] = fmt.Sprintf("%f", point.Lat)
	res[2] = fmt.Sprintf("%f", point.Long)
	return res
}
