package redis_client_mocks

import (
	"Vacation-planner/POI"
	"Vacation-planner/iowrappers"
	"github.com/alicebob/miniredis"
	"testing"
)

func TestNearbySearch(t *testing.T) {
	// set up
	mockServer, err := miniredis.Run()
	if err != nil {
		panic(err)
	}
	defer mockServer.Close()

	redisClient := iowrappers.RedisClient{}
	redisClient.Init(mockServer.Addr(), "", 0)

	places := make([]POI.Place, 2)
	places[0] = POI.Place{
		ID:               "1001",
		Name:             "Empire state building",
		LocationType:     POI.LocationTypeMuseum,
		Address:          POI.Address{},
		FormattedAddress: "20 W 34th St, New York, NY 10001",
		Location:         POI.Location{Type: "point", Coordinates: [2]float64{-73.9857, 40.7484}},
		PriceLevel:       3,
		Rating:           4.6,
		Hours:            [7]string{},
	}

	places[1] = POI.Place{
		ID:               "2002",
		Name:             "Peter Luger's Steakhouse",
		LocationType:     POI.LocationTypeRestaurant,
		Address:          POI.Address{},
		FormattedAddress: "255 Northern Blvd, Great Neck, NY 11021",
		Location:         POI.Location{Type: "point", Coordinates: [2]float64{-73.7271, 40.7773}},
		PriceLevel:       5,
		Rating:           4.9,
		Hours:            [7]string{},
	}

	redisClient.SetGeocode(iowrappers.GeocodeQuery{
		City:    "New York City",
		Country: "US",
	}, 40.712800, -74.006000)

	err = redisClient.StorePlacesForLocation("40.712800,-74.006000", places)

	if err != nil {
		t.Error(err)
	}

	placeSearchRequest := iowrappers.PlaceSearchRequest{
		Location: "New York City,US",
		PlaceCat: "Visit",
		Radius:   uint(2511),
	}

	cachedPlaces, err := redisClient.NearbySearch(&placeSearchRequest)

	if err != nil {
		t.Error(err)
	}

	expectedNumPlaces := 1
	if len(cachedPlaces) != expectedNumPlaces || cachedPlaces[0].ID != places[0].ID {
		t.Errorf("nearby search should return %d places, got %d", expectedNumPlaces, len(cachedPlaces))
	}
}
