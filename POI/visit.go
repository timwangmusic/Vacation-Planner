package POI

import (
	"strconv"
	"strings"
)

func CreatePlace(name string, location string, addr string, locationType string, placeID string) (place Place) {
	place.SetType(locationType)
	place.SetName(name)
	place.SetID(placeID)
	var i uint8
	for i=DATE_SUNDAY; i<DATE_SATURDAY; i++{
		place.SetHour(i, "10 am - 9 pm")
	}

	l := strings.Split(location, ",")
	lat, lng := l[0], l[1]
	lat_f, _ := strconv.ParseFloat(lat, 64)
	lng_f, _ := strconv.ParseFloat(lng, 64)
	place.SetLocation([2]float64{lat_f, lng_f})
	place.SetAddress(addr)
	return
}