package redis_client_mocks

import (
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
)

var places = []POI.Place{
	{
		ID:               "1001",
		Name:             "Empire state building",
		LocationType:     POI.LocationTypeMuseum,
		Address:          POI.Address{},
		FormattedAddress: "20 W 34th St, New York, NY 10001",
		Location:         POI.Location{Longitude: -73.9857, Latitude: 40.7484},
		PriceLevel:       POI.PriceLevelThree,
		Rating:           4.6,
		Hours:            [7]string{},
		Status:           POI.Operational,
	},
	{
		ID:               "2002",
		Name:             "Peter Luger's Steakhouse",
		LocationType:     POI.LocationTypeRestaurant,
		Address:          POI.Address{},
		FormattedAddress: "255 Northern Blvd, Great Neck, NY 11021",
		Location:         POI.Location{Longitude: -73.7271, Latitude: 40.7773},
		PriceLevel:       POI.PriceLevelFour,
		Rating:           4.9,
		Hours:            [7]string{},
		Status:           POI.Operational,
	},
	{
		ID:               "3003",
		Name:             "Keens Steakhouse",
		LocationType:     POI.LocationTypeRestaurant,
		Address:          POI.Address{},
		FormattedAddress: "72 W 36th St, New York, NY 10018",
		Location:         POI.Location{Longitude: -73.9859, Latitude: 40.7507},
		PriceLevel:       POI.PriceLevelFour,
		Rating:           4.6,
		Hours:            [7]string{},
		Status:           POI.Operational,
	},
	{
		ID:               "4004",
		Name:             "The Morgan Library & Museum",
		LocationType:     POI.LocationTypeMuseum,
		Address:          POI.Address{},
		FormattedAddress: "225 Madison Ave, New York, NY 10016",
		Location:         POI.Location{Longitude: -73.9878, Latitude: 40.7496},
		PriceLevel:       POI.PriceLevelThree,
		Rating:           4.6,
		Hours:            [7]string{},
		Status:           POI.ClosedTemporarily,
	},
}

func init() {
	// cache places
	RedisClient.SetPlacesAddGeoLocations(RedisContext, places)

	// if place are not cached, it is possibly because of GeoAdd failure
	for _, place := range places {
		if !RedisMockSvr.Exists(iowrappers.PlaceDetailsRedisKeyPrefix + place.ID) {
			log.Errorf("place with ID %s does not exist in Redis", place.ID)
		}
	}
}

// The setup of this test case guarantees that the Peter Luger's Steakhouse is located OUTSIDE the search radius coverage,
func TestGetPlaces_shouldExcludePlacesOutsideOfSearchRadius(t *testing.T) {
	placeSearchRequest := iowrappers.PlaceSearchRequest{
		Location:   POI.Location{Longitude: -74.0060, Latitude: 40.7128},
		PlaceCat:   POI.PlaceCategoryEatery,
		Radius:     uint(5000),
		PriceLevel: POI.PriceLevelFour,
	}

	cachedEateryPlaces, err := RedisClient.NearbySearch(RedisContext, &placeSearchRequest)
	if err != nil {
		t.Error(err.Error())
		return
	}
	// "Keens Steakhouse"
	var expectedPlace = places[2]

	if len(cachedEateryPlaces) != 1 {
		t.Errorf("expect to have 1 place, but got %d instead", len(cachedEateryPlaces))
		return
	}
	if cachedEateryPlaces[0].ID != expectedPlace.ID {
		t.Errorf("expect to get %s, but got %s instead", expectedPlace.Name, cachedEateryPlaces[0].Name)
		return
	}
}

func TestGetPlaces_resultShouldBeEmptyAfterPriceMatch(t *testing.T) {
	// expect result should be empty, because mock data has no PriceLevelTwo places.
	placeSearchRequest := iowrappers.PlaceSearchRequest{
		Location:   POI.Location{Longitude: -74.0060, Latitude: 40.7128},
		PlaceCat:   POI.PlaceCategoryEatery,
		Radius:     uint(5000),
		PriceLevel: POI.PriceLevelTwo,
	}

	cachedEateryPlaces, err := RedisClient.NearbySearch(RedisContext, &placeSearchRequest)
	if err != nil {
		t.Error(err)
		return
	}

	if len(cachedEateryPlaces) != 0 {
		t.Errorf("Expect to have 0 place, but got %d instead", len(cachedEateryPlaces))
		return
	}
}

// The setup of this test case guarantees that the Morgan Library & Museum is within the search radius but is excluded due to temporary closure
func TestGetPlaces_shouldExcludePlacesNotOperational(t *testing.T) {
	placeSearchRequest := iowrappers.PlaceSearchRequest{
		Location:       POI.Location{Longitude: -74.0060, Latitude: 40.7128},
		PlaceCat:       POI.PlaceCategoryVisit,
		Radius:         uint(20000),
		BusinessStatus: POI.Operational,
	}

	cachedVisitPlaces, _ := RedisClient.NearbySearch(RedisContext, &placeSearchRequest)

	if len(cachedVisitPlaces) != 1 || cachedVisitPlaces[0].ID != places[0].ID {
		t.Logf("number of nearby visit places obtained from Redis is %d", len(cachedVisitPlaces))
		t.Error("failed to get cached Visit place")
	}
}
