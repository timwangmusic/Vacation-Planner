package POI

import (
	"strconv"
	"strings"
)

func CreateVisitLocation(name string, location string, addr string) (visitLocation Place){
	visitLocation.SetType(VISIT)
	visitLocation.SetName(name)
	for i:=0; i<7; i++{
		visitLocation.SetHour(i, "10 am - 9 pm")
	}

	l := strings.Split(location, ",")
	lat, lng := l[0], l[1]
	lat_f, _ := strconv.ParseFloat(lat, 64)
	lng_f, _ := strconv.ParseFloat(lng, 64)
	visitLocation.SetLocation([2]float64{lat_f, lng_f})
	visitLocation.SetAddress(addr)
	return
}