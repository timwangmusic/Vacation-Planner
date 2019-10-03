package POI

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
		LocationTypeCafe: StayingTimeLocationTypeCafe,
		LocationTypeRestaurant: StayingTimeLocationTypeRestaurant,
		LocationTypeMuseum: StayingTimeLocationTypeMuseum,
		LocationTypeGallery: StayingTimeLocationTypeGallery,
		LocationTypeAmusementPark: StayingTimeLocationTypeAmusementPark,
		LocationTypePark: StayingTimeLocationTypePark,
	}

	return stayingTimeMap[locationType]
}
