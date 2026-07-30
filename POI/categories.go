package POI

import (
	"fmt"
	"math"
	"sort"
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
	LocationTypeAny = LocationType("")
	// Eatery place types
	LocationTypeCafe         = LocationType("cafe")
	LocationTypeRestaurant   = LocationType("restaurant")
	LocationTypeBar          = LocationType("bar")
	LocationTypeBakery       = LocationType("bakery")
	LocationTypeMealTakeaway = LocationType("meal_takeaway")
	LocationTypeMealDelivery = LocationType("meal_delivery")
	LocationTypeNightClub    = LocationType("night_club")
	// Visit place types
	LocationTypeMuseum            = LocationType("museum")
	LocationTypeGallery           = LocationType("art_gallery")
	LocationTypeAmusementPark     = LocationType("amusement_park")
	LocationTypePark              = LocationType("park")
	LocationTypeTouristAttraction = LocationType("tourist_attraction")
	LocationTypeZoo               = LocationType("zoo")
	LocationTypeAquarium          = LocationType("aquarium")
	LocationTypeMovieTheater      = LocationType("movie_theater")
	LocationTypeStadium           = LocationType("stadium")
	LocationTypeBowlingAlley      = LocationType("bowling_alley")
	// Shopping place types
	LocationTypeShoppingMall    = LocationType("shopping_mall")
	LocationTypeDepartmentStore = LocationType("department_store")
	LocationTypeSupermarket     = LocationType("supermarket")
	LocationTypeClothingStore   = LocationType("clothing_store")
	LocationTypeStore           = LocationType("store")
	// LocationTypeGroceryOrSupermarket is a types[]-only value: it appears in a place's
	// Types list but is NOT a legal ?type= value for the legacy Nearby Search
	// (maps.ParsePlaceType rejects it). Never pass it to CreateMapSearchRequest or add it to
	// GetPlaceTypes; it is matched only via PrimaryLocationType/GetPlaceCategory.
	LocationTypeGroceryOrSupermarket = LocationType("grocery_or_supermarket")
	// Any type that is a strict specialization of the already-mapped `store` goes to Shopping.
	LocationTypeConvenienceStore = LocationType("convenience_store")
	LocationTypeHardwareStore    = LocationType("hardware_store")
	LocationTypeHomeGoodsStore   = LocationType("home_goods_store")
	LocationTypeElectronicsStore = LocationType("electronics_store")
	LocationTypeFurnitureStore   = LocationType("furniture_store")
	LocationTypeBookStore        = LocationType("book_store")
	LocationTypeShoeStore        = LocationType("shoe_store")
	LocationTypeJewelryStore     = LocationType("jewelry_store")
	LocationTypePetStore         = LocationType("pet_store")
	LocationTypeBicycleStore     = LocationType("bicycle_store")
	LocationTypeFlorist          = LocationType("florist")
	LocationTypeLiquorStore      = LocationType("liquor_store")
	LocationTypeGasStation       = LocationType("gas_station")
	// Lodging place types
	LocationTypeLodging = LocationType("lodging")
	// Wellness place types
	LocationTypeGym         = LocationType("gym")
	LocationTypeSpa         = LocationType("spa")
	LocationTypePharmacy    = LocationType("pharmacy")
	LocationTypeDrugstore   = LocationType("drugstore")
	LocationTypeBeautySalon = LocationType("beauty_salon")
	LocationTypeHairCare    = LocationType("hair_care")
)

// placeTypeToCategory is the reverse map from a Google place type to its category. It backs
// both GetPlaceCategory (the write path / classification rule) and ReclassifyForCategory (the
// read filter) below, so there is exactly one table that decides "what category does this
// Google type belong to" anywhere in the service.
//
// It is a SUPERSET of GetPlaceTypes' inverse: it covers every Google primary type (see
// PrimaryLocationType) this service knows how to classify, not only the types the nearby-search
// endpoints actively query for (GetPlaceTypes' 18 searched types are all present here too).
// Widening this map only ever makes ReclassifyForCategory keep MORE places, never fewer — see
// TestReclassifyForCategoryKeepsAllFormerlySearchedTypes.
var placeTypeToCategory = map[LocationType]PlaceCategory{
	// Eatery
	LocationTypeCafe:         PlaceCategoryEatery,
	LocationTypeRestaurant:   PlaceCategoryEatery,
	LocationTypeBar:          PlaceCategoryEatery,
	LocationTypeBakery:       PlaceCategoryEatery,
	LocationTypeMealTakeaway: PlaceCategoryEatery,
	LocationTypeMealDelivery: PlaceCategoryEatery,
	LocationTypeNightClub:    PlaceCategoryEatery,

	// Visit
	LocationTypePark:              PlaceCategoryVisit,
	LocationTypeAmusementPark:     PlaceCategoryVisit,
	LocationTypeGallery:           PlaceCategoryVisit,
	LocationTypeMuseum:            PlaceCategoryVisit,
	LocationTypeTouristAttraction: PlaceCategoryVisit,
	LocationTypeZoo:               PlaceCategoryVisit,
	LocationTypeAquarium:          PlaceCategoryVisit,
	LocationTypeMovieTheater:      PlaceCategoryVisit,
	LocationTypeStadium:           PlaceCategoryVisit,
	LocationTypeBowlingAlley:      PlaceCategoryVisit,

	// Shopping. Any type that is a strict specialization of the already-mapped `store` goes
	// to Shopping.
	LocationTypeShoppingMall:         PlaceCategoryShopping,
	LocationTypeDepartmentStore:      PlaceCategoryShopping,
	LocationTypeSupermarket:          PlaceCategoryShopping,
	LocationTypeClothingStore:        PlaceCategoryShopping,
	LocationTypeStore:                PlaceCategoryShopping,
	LocationTypeGroceryOrSupermarket: PlaceCategoryShopping,
	LocationTypeConvenienceStore:     PlaceCategoryShopping,
	LocationTypeHardwareStore:        PlaceCategoryShopping,
	LocationTypeHomeGoodsStore:       PlaceCategoryShopping,
	LocationTypeElectronicsStore:     PlaceCategoryShopping,
	LocationTypeFurnitureStore:       PlaceCategoryShopping,
	LocationTypeBookStore:            PlaceCategoryShopping,
	LocationTypeShoeStore:            PlaceCategoryShopping,
	LocationTypeJewelryStore:         PlaceCategoryShopping,
	LocationTypePetStore:             PlaceCategoryShopping,
	LocationTypeBicycleStore:         PlaceCategoryShopping,
	LocationTypeFlorist:              PlaceCategoryShopping,
	LocationTypeLiquorStore:          PlaceCategoryShopping,
	LocationTypeGasStation:           PlaceCategoryShopping,

	// Lodging
	LocationTypeLodging: PlaceCategoryLodging,

	// Wellness
	LocationTypeGym:         PlaceCategoryWellness,
	LocationTypeSpa:         PlaceCategoryWellness,
	LocationTypePharmacy:    PlaceCategoryWellness,
	LocationTypeDrugstore:   PlaceCategoryWellness,
	LocationTypeBeautySalon: PlaceCategoryWellness,
	LocationTypeHairCare:    PlaceCategoryWellness,

	// LocationTypeAny ("") is deliberately NOT a key: GetPlaceCategory("") must stay
	// ("", false), the same as any other unmapped type.
}

// GetPlaceCategory maps a Google Maps place type back to its category, reporting whether
// the type is mapped at all. placeTypeToCategory (above) is now a SUPERSET of GetPlaceTypes'
// inverse, not its exact inverse — it classifies every primary type this service recognizes,
// while GetPlaceTypes still only lists the subset each category's Nearby Search issues as
// ?type=. The write path relies on the shared subset: the nearby-search cache writes each place
// under EncodeNearbySearchRedisKey(GetPlaceCategory(place.LocationType), ...), so a type that
// resolves to a different category than the one it was searched under would never cache-hit.
//
// It deliberately has NO default category. An earlier version defaulted to Eatery, which
// silently absorbed place types the legacy Nearby Search does not understand — two
// Places-API-(New)-only types ("fast_food_restaurant", "food_court") were added to
// GetPlaceTypes(Eatery), Google ignored the unenforceable filter, and prominence-ranked
// hotels were written into the eatery geo buckets. Returning ok=false forces every caller
// to decide what an unmapped type means, and makes TestPlaceCategoryRoundTrip able to fail.
//
// DO NOT ADD: fast_food_restaurant, food_court. Both are Places API (New)-only values that
// never appear in a legacy Nearby Search result's types[], so adding them re-opens the exact
// guard class TestGetPlaceCategoryRejectsUnknownTypes exists for.
func GetPlaceCategory(placeType LocationType) (PlaceCategory, bool) {
	cat, ok := placeTypeToCategory[placeType]
	return cat, ok
}

// MappedLocationTypes returns every LocationType classified by GetPlaceCategory, sorted for
// deterministic iteration. Exported for tests (e.g. TestGetPlaceCategoryKeysAreGoogleTypes),
// which need to walk the full map without depending on Go's randomized map iteration order.
func MappedLocationTypes() []LocationType {
	keys := make([]LocationType, 0, len(placeTypeToCategory))
	for t := range placeTypeToCategory {
		keys = append(keys, t)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}

// GetPlaceTypes returns a set of types defined in Google Maps API given a location type. This
// is the SEARCHED subset: the exact place types each category's Nearby Search issues as ?type=.
// It is intentionally unchanged by the classification-map expansion above — widening it changes
// the outbound Google query for every existing search, not just how a result gets classified,
// and is how the fast_food_restaurant incident happened (a type the legacy API doesn't
// understand, silently ignored by Google, poisoning the eatery cache). Add new types to
// placeTypeToCategory / GetPlaceCategory instead of here.
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

// ReclassifyForCategory decides whether a place belongs in cat based on its PRIMARY function,
// and returns the place re-tagged with that primary type. It now keys on the same
// placeTypeToCategory map as GetPlaceCategory — the same map the nearby-search write path
// (SetPlacesAddGeoLocations) and the bucket-cleanup migration
// (RemoveMisclassifiedPlacesFromCategoryBuckets) key on — so there is one rule everywhere for
// "does this place belong in this category":
//
//   - primary type maps to cat                     → keep, LocationType := primary
//     (e.g. a "cafe"-searched result that is really a restaurant is re-tagged).
//   - primary type maps to a DIFFERENT category      → drop (keep=false): its main
//     function is something else (a supermarket the food search returned).
//   - primary type is unmapped, or there is no Types → keep unchanged (older cached records,
//     or a legal-but-uninteresting type), so coverage never regresses on data written before
//     Types was captured.
//
// Because placeTypeToCategory is a strict superset of GetPlaceTypes' searched types (see
// GetPlaceCategory's docstring), this keeps every place the old primary-in-GetPlaceTypes(cat)
// rule kept, plus more — never fewer. TestReclassifyForCategoryKeepsAllFormerlySearchedTypes
// pins that monotonicity.
func ReclassifyForCategory(place Place, cat PlaceCategory) (Place, bool) {
	primary := PrimaryLocationType(place.Types)
	if primary == LocationType("") {
		return place, true // records with no Types stay kept (older cache entries)
	}
	if c, ok := GetPlaceCategory(primary); ok && c == cat {
		place.LocationType = primary // re-tag with the true type
		return place, true
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
