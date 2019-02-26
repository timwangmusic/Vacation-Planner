package utils

import "math"

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
