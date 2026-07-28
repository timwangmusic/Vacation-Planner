package POI

import (
	"fmt"
	"strings"
)

type PlaceCategory string

const (
	PlaceCategoryVisit  = PlaceCategory("Visit")
	PlaceCategoryEatery = PlaceCategory("Eatery")
	// Categories below back the merchant/best-card nearby endpoint. They are not used by
	// trip planning, which only slots Visit/Eatery places.
	PlaceCategoryShopping = PlaceCategory("Shopping")
	PlaceCategoryLodging  = PlaceCategory("Lodging")
	PlaceCategoryWellness = PlaceCategory("Wellness")
)

type PlaceIcon string

const (
	PlaceIconVisit    = PlaceIcon("attractions")
	PlaceIconEatery   = PlaceIcon("restaurant")
	PlaceIconShopping = PlaceIcon("shopping_bag")
	PlaceIconLodging  = PlaceIcon("hotel")
	PlaceIconWellness = PlaceIcon("spa")
	PlaceIconEmpty    = PlaceIcon("")
)

type LocationType string

const (
	// LocationTypeAny leaves the Google Maps place type unset, used by keyword (brand) searches
	LocationTypeAny           = LocationType("")
	LocationTypeCafe          = LocationType("cafe")
	LocationTypeRestaurant    = LocationType("restaurant")
	LocationTypeMuseum        = LocationType("museum")
	LocationTypeGallery       = LocationType("art_gallery")
	LocationTypeAmusementPark = LocationType("amusement_park")
	LocationTypePark          = LocationType("park")
	// Shopping place types
	LocationTypeShoppingMall    = LocationType("shopping_mall")
	LocationTypeDepartmentStore = LocationType("department_store")
	LocationTypeSupermarket     = LocationType("supermarket")
	LocationTypeClothingStore   = LocationType("clothing_store")
	LocationTypeStore           = LocationType("store")
	// Lodging place types
	LocationTypeLodging = LocationType("lodging")
	// Wellness place types
	LocationTypeGym      = LocationType("gym")
	LocationTypeSpa      = LocationType("spa")
	LocationTypePharmacy = LocationType("pharmacy")
)

// GetPlaceCategory maps a Google Maps place type back to its category. It is the inverse of
// GetPlaceTypes and MUST stay consistent with it: the nearby-search cache writes each place
// under EncodeNearbySearchRedisKey(GetPlaceCategory(place.LocationType), ...), so a type that
// resolves to a different category than the one it was searched under would never cache-hit.
func GetPlaceCategory(placeType LocationType) (placeCategory PlaceCategory) {
	switch placeType {
	case LocationTypePark, LocationTypeAmusementPark, LocationTypeGallery, LocationTypeMuseum:
		placeCategory = PlaceCategoryVisit
	case LocationTypeCafe, LocationTypeRestaurant:
		placeCategory = PlaceCategoryEatery
	case LocationTypeShoppingMall, LocationTypeDepartmentStore, LocationTypeSupermarket, LocationTypeClothingStore, LocationTypeStore:
		placeCategory = PlaceCategoryShopping
	case LocationTypeLodging:
		placeCategory = PlaceCategoryLodging
	case LocationTypeGym, LocationTypeSpa, LocationTypePharmacy:
		placeCategory = PlaceCategoryWellness
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
	case PlaceCategoryShopping:
		placeTypes = append(placeTypes,
			[]LocationType{LocationTypeShoppingMall, LocationTypeDepartmentStore, LocationTypeSupermarket, LocationTypeClothingStore, LocationTypeStore}...)
	case PlaceCategoryLodging:
		placeTypes = append(placeTypes,
			[]LocationType{LocationTypeLodging}...)
	case PlaceCategoryWellness:
		placeTypes = append(placeTypes,
			[]LocationType{LocationTypeGym, LocationTypeSpa, LocationTypePharmacy}...)
	}
	return
}

// ParsePlaceCategory converts a category string (e.g. from an API request) into a known
// PlaceCategory, reporting whether it matched. Matching is exact against the canonical
// category names ("Eatery", "Shopping", "Lodging", "Wellness", "Visit").
func ParsePlaceCategory(s string) (PlaceCategory, bool) {
	switch PlaceCategory(s) {
	case PlaceCategoryVisit, PlaceCategoryEatery, PlaceCategoryShopping, PlaceCategoryLodging, PlaceCategoryWellness:
		return PlaceCategory(s), true
	default:
		return PlaceCategory(""), false
	}
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

// NormalizeBrandKey converts a brand keyword into a stable slug used in Redis keys and
// name matching, e.g. "Dunkin' Donuts" -> "dunkin-donuts"
func NormalizeBrandKey(keyword string) string {
	var b strings.Builder
	pendingDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(keyword)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			if pendingDash && b.Len() > 0 {
				b.WriteRune('-')
			}
			b.WriteRune(r)
			pendingDash = false
		} else {
			pendingDash = true
		}
	}
	return b.String()
}

// EncodeBrandNearbySearchRedisKey generates a Redis key for brand-scoped nearby search,
// keeping brand search results in buckets separate from category-based searches
func EncodeBrandNearbySearchRedisKey(keyword string) string {
	return strings.Join([]string{"placeIDs", "brand", NormalizeBrandKey(keyword)}, ":")
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
