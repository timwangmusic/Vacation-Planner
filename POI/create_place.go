package POI

import (
	"strconv"
	"strings"
)

type OpeningHours struct {
	Hours []string
}

func CreatePlace(name string, location string, addr string, formattedAddr string, locationType string, openingHours *OpeningHours, placeID string, priceLevel int, rating float32) (place Place) {
	place.SetType(locationType)
	place.SetName(name)
	place.SetID(placeID)
	var i Weekday
	if openingHours != nil && openingHours.Hours != nil {
		for i = DATE_MONDAY; i <= DATE_SUNDAY; i++ {
			place.SetHour(i, openingHours.Hours[i])
		}
	}
	// set default
	for i = DATE_MONDAY; i <= DATE_SUNDAY; i++ {
		if place.GetHour(i) == "" {
			place.SetHour(i, "8:30 am – 9:30 pm")
		}
	}
	l := strings.Split(location, ",")
	lat, lng := l[0], l[1]
	lat_f, _ := strconv.ParseFloat(lat, 64)
	lng_f, _ := strconv.ParseFloat(lng, 64)
	place.SetLocation([2]float64{lng_f, lat_f}) // ensure same lng/lat order as the docs in MongoDB
	place.SetAddress(addr)
	place.SetFormattedAddress(formattedAddr)
	place.SetPriceLevel(priceLevel)
	place.SetRating(rating)
	return
}
