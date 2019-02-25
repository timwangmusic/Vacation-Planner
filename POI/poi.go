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
	GetHour(int) string

	// re-set name if POI name changed
	SetName(string)
	// re-set type if POI type changed
	SetType(string)
	// re-set time if POI opening hour changed for some day in a week
	SetHour(int, string)
}
