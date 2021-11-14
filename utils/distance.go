package utils

import (
	"math"
	"strconv"
	"strings"
)

func HaversineDist(x []float64, y []float64) float64 {
	var latX, lngX, latY, lngY = x[0], x[1], y[0], y[1] // latitudes and longitudes in radius
	lat1 := latX * math.Pi / 180
	lng1 := lngX * math.Pi / 180
	lat2 := latY * math.Pi / 180
	lng2 := lngY * math.Pi / 180

	// radius of Earth in meters
	rEarth := 6378100.0

	// calculate haversine of central angle of the given two points
	h := hav(lat2-lat1) + math.Cos(lat2)*math.Cos(lat1)*hav(lng2-lng1)

	return math.Asin(math.Sqrt(h)) * rEarth * 2
}

func hav(theta float64) float64 {
	return (1 - math.Cos(theta)) / 2
}

func ParseLocation(location string) ([2]float64, error) {
	latLng := strings.Split(location, ",")

	var res [2]float64

	lat, err := strconv.ParseFloat(latLng[0], 64)
	if err != nil {
		return res, err
	}
	lng, err := strconv.ParseFloat(latLng[1], 64)
	if err != nil {
		return res, err
	}

	res[0] = lat
	res[1] = lng

	return res, nil
}
