package POI

import (
	"googlemaps.github.io/maps"
	"strconv"
	"strings"
)

type OpeningHours struct {
	Hours []string
}

func CreatePlace(name, location, addr, formattedAddr string, locationType LocationType, openingHours *OpeningHours, placeID string, priceLevel int, rating float32, url string, photo *maps.Photo, userRatingsTotal int) (place Place) {
	place.SetType(locationType)
	place.SetName(name)
	place.SetID(placeID)
	var i Weekday
	if openingHours != nil && openingHours.Hours != nil {
		for i = DateMonday; i <= DateSunday; i++ {
			place.SetHour(i, openingHours.Hours[i])
		}
	}
	// set default
	for i = DateMonday; i <= DateSunday; i++ {
		if place.GetHour(i) == "" {
			place.SetHour(i, "8:30 am – 9:30 pm")
		}
	}
	l := strings.Split(location, ",")
	lat, lng := l[0], l[1]
	latFloat, _ := strconv.ParseFloat(lat, 64)
	lngFloat, _ := strconv.ParseFloat(lng, 64)
	place.SetLocation([2]float64{lngFloat, latFloat}) // ensure same lng/lat order as the docs in MongoDB
	place.SetAddress(addr)
	place.SetFormattedAddress(formattedAddr)
	place.SetPriceLevel(priceLevel)
	place.SetRating(rating)
	place.SetURL(url)
	place.SetPhoto(photo)
	place.SetUserRatingsTotal(userRatingsTotal)
	return
}
