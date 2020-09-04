package matching

import "github.com/weihesdlegend/Vacation-planner/POI"

type Place struct {
	Place    *POI.Place
	Category POI.PlaceCategory `json:"category"`
	Address  string            `json:"address"`
	Price    float64           `json:"price"`
	Location [2]float64        `json:"geolocation"`
}

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

func (place Place) GetLocation() [2]float64 {
	return place.Location
}

func (place Place) GetURL() string {
	return place.Place.GetURL()
}

func (place Place) SetURL(url string) {
	place.Place.SetURL(url)
}

func CreatePlace(place POI.Place, category POI.PlaceCategory) Place {
	Place_ := Place{}
	Place_.Place = &place
	Place_.Address = place.GetFormattedAddress()
	Place_.Price = checkPrice(place.GetPriceLevel())
	Place_.Location = place.GetLocation()
	Place_.Category = category
	return Place_
}
