/*

package POI defines basic point of interest structure and IO with other packages such as graph.
Also this package uses services such as io wrappers in utils.

*/

package POI

type PlaceCategory string

const (
	PlaceCategoryVisit = PlaceCategory("visit")
	PlaceCategoryStay = PlaceCategory("stay")
	PlaceCategoryEatery = PlaceCategory("eatery")
)

type POI interface {
	// unique identity for the POI
	GetID() string
	// name for the POI
	GetName() string
	// POI type
	GetType() string
	// address
	GetAddress() string
	// lat, lng
	GetLocation() [2]float64
	// opening hours of the specified day in a week
	GetHour(Weekday) string
	// price range
	GetPriceLevel() int

	// set POI name
	SetName(string)
	// set POI type
	SetType(string)
	// set POI opening hour for some day in a week
	SetHour(Weekday, string)
	// set price range
	SetPriceLevel(int)
}
