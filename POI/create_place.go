package POI

import (
	"googlemaps.github.io/maps"
)

type OpeningHours struct {
	Hours []string
}

func CreatePlace(name, addr, formattedAddr, businessStatus string, locationType LocationType, openingHours *OpeningHours, placeID string, priceLevel int, rating float32, url string, photo *maps.Photo, userRatingsTotal int, latitude, longitude float64) (place Place) {
	place.SetType(locationType)
	place.SetName(name)
	place.SetID(placeID)
	var weekday Weekday
	if openingHours != nil && openingHours.Hours != nil {
		for weekday = DateMonday; weekday <= DateSunday; weekday++ {
			place.SetHour(weekday, openingHours.Hours[weekday])
		}
	}
	// set default
	for weekday = DateMonday; weekday <= DateSunday; weekday++ {
		if place.GetHour(weekday) == "" {
			place.SetHour(weekday, "8:30 am – 9:30 pm")
		}
	}

	place.SetStatus(businessStatus)
	place.SetLocationCoordinates([2]float64{latitude, longitude})
	place.SetAddress(addr)
	place.SetFormattedAddress(formattedAddr)
	place.SetPriceLevel(priceLevel)
	place.SetRating(rating)
	place.SetURL(url)
	place.SetPhoto(photo)
	place.SetUserRatingsTotal(userRatingsTotal)
	return
}
