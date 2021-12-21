package matching

// ToPlaceView transforms Place to PlaceView
func ToPlaceView(place Place) PlaceView {
	placeView := PlaceView{
		ID:           place.GetPlaceId(),
		Name:         place.GetPlaceName(),
		URL:          place.GetURL(),
		Rating:       place.GetRating(),
		RatingsCount: place.GetUserRatingsCount(),
		AveragePrice: place.GetPrice(),
		Hours:        place.GetHours(),
	}
	return placeView
}
