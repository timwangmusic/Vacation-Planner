package matching

// ToPlaceView transforms Place to PlaceView
func ToPlaceView(place Place) PlaceView {
	placeView := PlaceView{
		ID:           place.Id(),
		Name:         place.Name(),
		URL:          place.Url(),
		Rating:       place.Rating(),
		RatingsCount: place.UserRatingsCount(),
		AveragePrice: place.PlacePrice(),
		Hours:        place.Hours(),
	}
	return placeView
}
