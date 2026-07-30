package POI

import (
	"fmt"
	"math"
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
	LocationTypeBar           = LocationType("bar")
	LocationTypeBakery        = LocationType("bakery")
	LocationTypeMealTakeaway  = LocationType("meal_takeaway")
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

// GetPlaceCategory maps a Google Maps place type back to its category, reporting whether
// the type is mapped at all. It is the inverse of GetPlaceTypes and MUST stay consistent
// with it: the nearby-search cache writes each place under
// EncodeNearbySearchRedisKey(GetPlaceCategory(place.LocationType), ...), so a type that
// resolves to a different category than the one it was searched under would never cache-hit.
//
// It deliberately has NO default category. An earlier version defaulted to Eatery, which
// silently absorbed place types the legacy Nearby Search does not understand — two
// Places-API-(New)-only types ("fast_food_restaurant", "food_court") were added to
// GetPlaceTypes(Eatery), Google ignored the unenforceable filter, and prominence-ranked
// hotels were written into the eatery geo buckets. Returning ok=false forces every caller
// to decide what an unmapped type means, and makes TestPlaceCategoryRoundTrip able to fail.
func GetPlaceCategory(placeType LocationType) (PlaceCategory, bool) {
	switch placeType {
	case LocationTypePark, LocationTypeAmusementPark, LocationTypeGallery, LocationTypeMuseum:
		return PlaceCategoryVisit, true
	case LocationTypeCafe, LocationTypeRestaurant, LocationTypeBar, LocationTypeBakery, LocationTypeMealTakeaway:
		return PlaceCategoryEatery, true
	case LocationTypeShoppingMall, LocationTypeDepartmentStore, LocationTypeSupermarket, LocationTypeClothingStore, LocationTypeStore:
		return PlaceCategoryShopping, true
	case LocationTypeLodging:
		return PlaceCategoryLodging, true
	case LocationTypeGym, LocationTypeSpa, LocationTypePharmacy:
		return PlaceCategoryWellness, true
	default:
		return PlaceCategory(""), false
	}
}

// GetPlaceTypes returns a set of types defined in Google Maps API given a location type
func GetPlaceTypes(placeCat PlaceCategory) (placeTypes []LocationType) {
	switch placeCat {
	case PlaceCategoryVisit:
		placeTypes = append(placeTypes,
			[]LocationType{LocationTypePark, LocationTypeAmusementPark, LocationTypeGallery, LocationTypeMuseum}...)
	case PlaceCategoryEatery:
		placeTypes = append(placeTypes,
			[]LocationType{LocationTypeCafe, LocationTypeRestaurant, LocationTypeBar, LocationTypeBakery, LocationTypeMealTakeaway}...)
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

// AllPlaceCategories enumerates every place category. It is the single source of truth for
// "what categories exist": ParsePlaceCategory validates against it, and callers that must touch
// every category's geo bucket (e.g. deleting a place that may be filed under several) iterate
// it rather than hardcoding a subset — which is how Shopping, Lodging, and Wellness came to be
// missed by cleanup paths written before they existed.
var AllPlaceCategories = []PlaceCategory{
	PlaceCategoryVisit,
	PlaceCategoryEatery,
	PlaceCategoryShopping,
	PlaceCategoryLodging,
	PlaceCategoryWellness,
}

// ParsePlaceCategory converts a category string (e.g. from an API request) into a known
// PlaceCategory, reporting whether it matched. Matching is exact against the canonical
// category names ("Eatery", "Shopping", "Lodging", "Wellness", "Visit").
func ParsePlaceCategory(s string) (PlaceCategory, bool) {
	for _, cat := range AllPlaceCategories {
		if PlaceCategory(s) == cat {
			return cat, true
		}
	}
	return PlaceCategory(""), false
}

// umbrellaLocationTypes are Google's generic feature types that describe almost
// every place and say nothing about its primary function. They are skipped when
// picking a place's primary type.
// Note: "store" is intentionally NOT here — it is a meaningful Shopping type.
var umbrellaLocationTypes = map[LocationType]bool{
	LocationType("food"):              true,
	LocationType("point_of_interest"): true,
	LocationType("establishment"):     true,
	LocationType("premise"):           true,
	LocationType("geocode"):           true,
	LocationType("political"):         true,
}

// PrimaryLocationType returns a place's primary Google feature type: the first
// entry in its Types list that isn't a generic umbrella (food, point_of_interest,
// …). Google lists the most specific type first, so this is the place's real
// function (e.g. "supermarket" for a store the restaurant search also matched).
// Returns "" when there is no meaningful type (empty/unknown Types).
func PrimaryLocationType(types []string) LocationType {
	for _, t := range types {
		lt := LocationType(t)
		if !umbrellaLocationTypes[lt] {
			return lt
		}
	}
	return LocationType("")
}

// ReclassifyForCategory decides whether a place belongs in cat based on its
// PRIMARY function, and returns the place re-tagged with that primary type.
//
//   - primary type is one of cat's search types  → keep, LocationType := primary
//     (e.g. a "cafe"-searched result that is really a restaurant is re-tagged).
//   - primary type is known but NOT in cat        → drop (keep=false): its main
//     function is something else (a supermarket the food search returned).
//   - no Types on the place (older cached records) → keep unchanged, so coverage
//     never regresses on data written before Types was captured.
func ReclassifyForCategory(place Place, cat PlaceCategory) (Place, bool) {
	primary := PrimaryLocationType(place.Types)
	if primary == LocationType("") {
		return place, true
	}
	for _, t := range GetPlaceTypes(cat) {
		if t == primary {
			place.LocationType = primary
			return place, true
		}
	}
	return place, false
}

// PriceyEatery returns whether a eatery place is expensive based on its price level
func PriceyEatery(placeCategory PlaceCategory, priceLevel PriceLevel) bool {
	return (placeCategory == PlaceCategoryEatery) && (priceLevel >= PriceLevelThree)
}

// EncodeNearbySearchRedisKey generates the Redis geo-index key for a category's nearby search.
//
// One bucket per category, with no price segment. Eateries used to be split into
// placeIDs:eatery:level0..4 keyed on each place's own price level, which fragmented the index
// for no benefit: Google omits price_level for most places (so they collapsed into level0) and
// only accepts a price filter at level >= 3, so searches for levels 0-2 were identical yet each
// read back a fifth of the data. Callers that care about price already filter after the read
// (matching.filterPlacesOnPriceLevel). Redis GEO is a sorted set scored by 52-bit geohash and
// GEORADIUS probes 9 geohash cells at O(log N + M), so one bucket holds millions of members
// without degrading — which is how placeIDs:visit has always worked.
func EncodeNearbySearchRedisKey(placeCategory PlaceCategory) string {
	return strings.Join([]string{"placeIDs", strings.ToLower(string(placeCategory))}, ":")
}

// searchCellDegrees sizes the freshness grid to iowrappers.ColdStartSearchRadius (~8 km), the
// area one cold external search actually populates. A fixed-degree grid narrows in meters as
// latitude rises, which only shrinks cells — erring toward an extra cold search, never toward
// claiming coverage we do not have.
const searchCellDegrees = 0.072

// EncodeSearchCell quantizes coordinates to the freshness grid. This is a cache key, not a
// spatial index: it is never range-queried, so it needs no neighbor probing or Z-order
// ordering. The only property that matters is that a cell is no larger than the area a cold
// search populates.
func EncodeSearchCell(lat, lng float64) string {
	return fmt.Sprintf("%d_%d",
		int(math.Floor(lat/searchCellDegrees)),
		int(math.Floor(lng/searchCellDegrees)))
}

// EncodeLastSearchTimeField identifies the external search variant that last covered a cell:
//
//	<cell>:<category>          the unfiltered search — every category, and eatery levels 0-2
//	<cell>:eatery:pricey<N>    the price-filtered 4x-radius search, N in {3,4}
//
// Note this is scoped to the SEARCH, not to the bucket. Levels 0-2 share one field because
// Google is issued an identical unfiltered request for all three, so two of every three
// fan-outs were redundant. Levels 3-4 keep their own field because PriceyEatery makes Google
// apply a real price filter at four times the radius: a fresh generic marker must not suppress
// that search, or expensive places beyond the generic search's reach are never fetched.
//
// It is keyed on a location cell rather than country/admin1/city because the buckets it guards
// are geo indexes read from arbitrary coordinates. A city name has no extent, so it cannot
// answer "did we populate the area this query covers?" — a request 20 km from a city centroid
// would read a marker claiming freshness over ground no search had reached.
func EncodeLastSearchTimeField(placeCategory PlaceCategory, level PriceLevel, lat, lng float64) string {
	segments := []string{EncodeSearchCell(lat, lng), strings.ToLower(string(placeCategory))}
	if PriceyEatery(placeCategory, level) {
		segments = append(segments, fmt.Sprintf("pricey%d", level))
	}
	return strings.Join(segments, ":")
}

// EncodeBrandLastSearchTimeField is the brand-search equivalent of EncodeLastSearchTimeField.
// Brand buckets are geo indexes read from precise coordinates too, so they need the same cell
// scoping.
func EncodeBrandLastSearchTimeField(keyword string, lat, lng float64) string {
	return strings.Join([]string{EncodeSearchCell(lat, lng), "brand", NormalizeBrandKey(keyword)}, ":")
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
