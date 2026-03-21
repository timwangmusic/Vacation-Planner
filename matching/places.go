package matching

import (
	"github.com/weihesdlegend/Vacation-planner/POI"
)

type Place struct {
	Place    *POI.Place
	Category POI.PlaceCategory `json:"category"`
	Address  string            `json:"address"`
	Price    float64           `json:"price"`
}

func (place *Place) Hours() [7]string {
	return place.Place.Hours
}

func (place *Place) Id() string {
	return place.Place.GetID()
}

func (place *Place) Name() string {
	return place.Place.GetName()
}

func (place *Place) Type() POI.LocationType {
	return place.Place.GetType()
}

func (place *Place) PlaceCategory() POI.PlaceCategory {
	return place.Category
}

func (place *Place) PlaceAddress() string {
	return place.Address
}

func (place *Place) PlacePrice() float64 {
	return place.Price
}

func (place *Place) Rating() float32 {
	return place.Place.GetRating()
}

func (place *Place) Location() POI.Location {
	return place.Place.Location
}

func (place *Place) Url() string {
	return place.Place.GetURL()
}

func (place *Place) UserRatingsCount() int {
	return place.Place.GetUserRatingsTotal()
}

func (place *Place) SetURL(url string) {
	place.Place.SetURL(url)
}

func (place *Place) SetCategory(category POI.PlaceCategory) {
	place.Category = category
}

func (place *Place) IsOpenBetween(interval TimeInterval, stayingDurationInHour uint8) bool {
	if interval.StartHour+stayingDurationInHour > interval.EndHour {
		return false
	}

	hours := place.Hours()
	if int(interval.Day) >= len(hours) {
		return false
	}

	openingHour := hours[interval.Day]
	if openingHour == "" {
		// No hours data available; assume open to avoid false negatives
		return true
	}

	placeInterval, err := POI.ParseTimeInterval(openingHour)
	if err != nil {
		return false
	}

	// Closed marker (ParseTimeInterval returns 255,255 for "Closed")
	if placeInterval.Start == 255 && placeInterval.End == 255 {
		return false
	}

	requestedSlot := POI.TimeInterval{
		Start: POI.Hour(interval.StartHour),
		End:   POI.Hour(interval.StartHour + stayingDurationInHour),
	}
	return placeInterval.Inclusive(&requestedSlot)
}

func CreatePlace(place POI.Place, category POI.PlaceCategory) Place {
	Place_ := Place{}
	Place_.Place = &place
	Place_.Address = place.GetFormattedAddress()
	Place_.Price = AveragePricing(place.GetPriceLevel())
	Place_.Category = category
	return Place_
}
