package utils

import (
	"math"
	"strconv"
	"strings"
)

func HaversineDist(x []float64, y []float64) float64 {
	var xlat, xlng, ylat, ylng = x[0], x[1], y[0], y[1]// latitudes and longtitudes in radius
	lat1 := xlat * math.Pi / 180
	long1 := xlng * math.Pi / 180
	lat2 := ylat * math.Pi / 180
	lng2 := ylng * math.Pi / 180

	// radius of Earth in meters
	rEearth := 6378100.0

	// calculate haversine of central angle of the given two points
	h := hav(lat2-lat1) + math.Cos(lat2)*math.Cos(lat1)*hav(lng2-long1)

	return math.Asin(math.Sqrt(h)) * rEearth * 2
}

func hav(theta float64) float64 {
	return (1 - math.Cos(theta)) / 2
}

// locations are in the format of "lat,lng"
func ParseLocation(location string) ([]float64, error) {
	latlng := strings.Split(location, ",")

	res := make([]float64, 2)

	lat, err := strconv.ParseFloat(latlng[0], 64)
	if err != nil {
		return res, err
	}
	lng, err := strconv.ParseFloat(latlng[1], 64)
	if err != nil {
		return res, err
	}

	res[0] = lat
	res[1] = lng

	return res, nil
}
