package matching

// ToPlaceView transforms Place to PlaceView
func ToPlaceView(place Place) PlaceView {
	placeView := PlaceView{
		ID:           place.GetPlaceId(),
		Name:         place.GetPlaceName(),
		URL:          place.GetURL(),
		Rating:       place.GetRating(),
		RatingsCount: place.GetUserRatingsCount(),
		PriceLevel:   place.Place.GetPriceLevel(),
		Hours:        place.GetHours(),
	}
	return placeView
}
