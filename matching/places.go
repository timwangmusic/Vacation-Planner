package matching

import (
	"github.com/weihesdlegend/Vacation-planner/POI"
)

const DefaultPlaceSearchRadius = 10000

type Place struct {
	Place    *POI.Place
	Category POI.PlaceCategory `json:"category"`
	Address  string            `json:"address"`
	Price    float64           `json:"price"`
}

type ByScore []Place

func (p ByScore) Len() int { return len(p) }

func (p ByScore) Less(i, j int) bool {
	return Score([]Place{p[i]}, DefaultPlaceSearchRadius) > Score([]Place{p[j]}, DefaultPlaceSearchRadius)
}

func (p ByScore) Swap(i, j int) { p[i], p[j] = p[j], p[i] }

func (place Place) GetHours() [7]string {
	return place.Place.Hours
}

func (place Place) GetPlaceId() string {
	return place.Place.GetID()
}

func (place Place) GetPlaceName() string {
	return place.Place.GetName()
}

func (place Place) GetPlaceType() POI.LocationType {
	return place.Place.GetType()
}

func (place Place) GetPlaceCategory() POI.PlaceCategory {
	return place.Category
}

func (place Place) GetPlaceFormattedAddress() string {
	return place.Address
}

func (place Place) GetPrice() float64 {
	return place.Price
}

func (place Place) GetRating() float32 {
	return place.Place.GetRating()
}

func (place Place) GetLocation() POI.Location {
	return place.Place.Location
}

func (place Place) GetURL() string {
	return place.Place.GetURL()
}

func (place Place) GetUserRatingsCount() int {
	return place.Place.GetUserRatingsTotal()
}

func (place Place) SetURL(url string) {
	place.Place.SetURL(url)
}

func (place Place) IsOpenBetween(interval QueryTimeInterval, stayingDurationInHour uint8) bool {
	//TODO: Query whither this place is open at this period in the future after POI.PLACE contains open hour.
	//Dummy implementation, only checks if the time period is valid

	return interval.StartHour+stayingDurationInHour <= interval.EndHour
}

func CreatePlace(place POI.Place, category POI.PlaceCategory) Place {
	Place_ := Place{}
	Place_.Place = &place
	Place_.Address = place.GetFormattedAddress()
	Place_.Price = AveragePricing(place.GetPriceLevel())
	Place_.Category = category
	return Place_
}
