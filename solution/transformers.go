package solution

import "github.com/weihesdlegend/Vacation-planner/matching"

// ToPlaceView transforms matching.Place to solution.PlaceView
func ToPlaceView(place matching.Place) PlaceView {
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
