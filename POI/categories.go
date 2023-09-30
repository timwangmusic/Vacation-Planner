package POI

import (
	"fmt"
	"strings"
)

type PlaceCategory string

const (
	PlaceCategoryVisit  = PlaceCategory("Visit")
	PlaceCategoryEatery = PlaceCategory("Eatery")
)

type PlaceIcon string

const (
	PlaceIconVisit  = PlaceIcon("attractions")
	PlaceIconEatery = PlaceIcon("restaurant")
	PlaceIconEmpty  = PlaceIcon("")
)

type LocationType string

const (
	LocationTypeCafe          = LocationType("cafe")
	LocationTypeRestaurant    = LocationType("restaurant")
	LocationTypeMuseum        = LocationType("museum")
	LocationTypeGallery       = LocationType("art_gallery")
	LocationTypeAmusementPark = LocationType("amusement_park")
	LocationTypePark          = LocationType("park")
)

func GetPlaceCategory(placeType LocationType) (placeCategory PlaceCategory) {
	switch placeType {
	case LocationTypePark, LocationTypeAmusementPark, LocationTypeGallery, LocationTypeMuseum:
		placeCategory = PlaceCategoryVisit
	case LocationTypeCafe, LocationTypeRestaurant:
		placeCategory = PlaceCategoryEatery
	default:
		placeCategory = PlaceCategoryEatery
	}
	return
}

// GetPlaceTypes returns a set of types defined in Google Maps API given a location type
func GetPlaceTypes(placeCat PlaceCategory) (placeTypes []LocationType) {
	switch placeCat {
	case PlaceCategoryVisit:
		placeTypes = append(placeTypes,
			[]LocationType{LocationTypePark, LocationTypeAmusementPark, LocationTypeGallery, LocationTypeMuseum}...)
	case PlaceCategoryEatery:
		placeTypes = append(placeTypes,
			[]LocationType{LocationTypeCafe, LocationTypeRestaurant}...)
	}
	return
}

// PriceyEatery returns whether a eatery place is expensive based on its price level
func PriceyEatery(placeCategory PlaceCategory, priceLevel PriceLevel) bool {
	return (placeCategory == PlaceCategoryEatery) && (priceLevel >= PriceLevelThree)
}

// EncodeNearbySearchRedisKey generates a Redis Key for Redis nearby search with place category and price info
// The key includes the price level info for eatery and no price info for visit
func EncodeNearbySearchRedisKey(placeCategory PlaceCategory, level PriceLevel) string {
	keys := []string{"placeIDs", strings.ToLower(string(placeCategory))}
	// add price levels for eatery category
	if placeCategory == PlaceCategoryEatery {
		keys = append(keys, fmt.Sprintf("level%d", level))
	}
	return strings.Join(keys, ":")
}

type StayingTime uint8

const (
	StayingTimeLocationTypeCafe          = StayingTime(1)
	StayingTimeLocationTypeRestaurant    = StayingTime(1)
	StayingTimeLocationTypeMuseum        = StayingTime(3)
	StayingTimeLocationTypeGallery       = StayingTime(2)
	StayingTimeLocationTypeAmusementPark = StayingTime(3)
	StayingTimeLocationTypePark          = StayingTime(2)
)

func GetStayingTimeForLocationType(locationType LocationType) StayingTime {
	var stayingTimeMap = map[LocationType]StayingTime{
		LocationTypeCafe:          StayingTimeLocationTypeCafe,
		LocationTypeRestaurant:    StayingTimeLocationTypeRestaurant,
		LocationTypeMuseum:        StayingTimeLocationTypeMuseum,
		LocationTypeGallery:       StayingTimeLocationTypeGallery,
		LocationTypeAmusementPark: StayingTimeLocationTypeAmusementPark,
		LocationTypePark:          StayingTimeLocationTypePark,
	}

	return stayingTimeMap[locationType]
}
