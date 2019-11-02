package redis_client_mocks

import (
	"Vacation-planner/POI"
	"Vacation-planner/iowrappers"
	"github.com/alicebob/miniredis/v2"
	"testing"
)

func TestGetPlaces(t *testing.T) {
	// set up mock server
	mockServer, err := miniredis.Run()
	if err != nil {
		panic(err)
	}
	defer mockServer.Close()

	// set up data
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
		ID:               "3003",
		Name:             "Keens Steakhouse",
		LocationType:     POI.LocationTypeRestaurant,
		Address:          POI.Address{},
		FormattedAddress: "72 W 36th St, New York, NY 10018",
		Location:         POI.Location{Type: "point", Coordinates: [2]float64{-73.98597, 40.750706}},
		PriceLevel:       5,
		Rating:           4.6,
		Hours:            [7]string{},
	}

	redisClient := iowrappers.RedisClient{}
	redisClient.Init(mockServer.Addr(), "", 0)

	// cache places
	redisClient.SetPlacesOnCategory(places)

	// if place are not cached, it is possibly because of GeoAdd failure
	for _, place := range places {
		if !mockServer.Exists(place.ID) {
			t.Errorf("place %s does not exist in Redis", place.ID)
		}
	}

	nycLatLng := "40.712800,-74.006000"
	placeSearchRequest := iowrappers.PlaceSearchRequest{
		Location: nycLatLng,
		PlaceCat: "Visit",
		Radius:   uint(5000),
		MinNumResults: 1,
	}

	cachedVisitPlaces := redisClient.GetPlaces(&placeSearchRequest)

	if len(cachedVisitPlaces) != 1 || cachedVisitPlaces[0].ID != places[0].ID {
		t.Logf("number of nearby visit places obtained from Redis is %d", len(cachedVisitPlaces))
		t.Error("failed to get cached Visit place")
	}

	placeSearchRequest = iowrappers.PlaceSearchRequest{
		Location: nycLatLng,
		PlaceCat: "Eatery",
		Radius:   uint(5000),
		MinNumResults: 1,
	}

	cachedEateryPlaces := redisClient.GetPlaces(&placeSearchRequest)

	if len(cachedEateryPlaces) != 1 || cachedEateryPlaces[0].ID != places[1].ID {
		t.Logf("number of nearby eatery places obtained from Redis is %d", len(cachedEateryPlaces))
		t.Error("failed to get cached Eatery place")
	}
}
