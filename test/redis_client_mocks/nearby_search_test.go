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
		t.Errorf("RedisClient.NearbySearch error %v", err)
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

// TestGetPlaces_readsEveryPriceLevel pins that the geo read is price-agnostic.
//
// This replaces a test that asserted a PriceLevelTwo eatery search returned nothing because no
// fixture place carries price level 2. That only held while eateries were split across
// placeIDs:eatery:level0..4, and the split was the defect: Google omits price_level for most
// places (so they all landed in level0) and only accepts a price filter at level >= 3, so
// searches for levels 0-2 issued identical requests yet each read back a fifth of the data.
//
// Price selection is the caller's job — matching.MatcherForPriceRange applies
// filterPlacesOnPriceLevel to these results (planner/solver.go). The cache's contract is
// "everything of this category near here", nothing narrower.
func TestGetPlaces_readsEveryPriceLevel(t *testing.T) {
	requestAt := func(level POI.PriceLevel) []POI.Place {
		t.Helper()
		req := iowrappers.PlaceSearchRequest{
			Location:   POI.Location{Longitude: -74.0060, Latitude: 40.7128},
			PlaceCat:   POI.PlaceCategoryEatery,
			Radius:     uint(5000),
			PriceLevel: level,
		}
		got, err := RedisClient.NearbySearch(RedisContext, &req)
		if err != nil {
			t.Fatalf("RedisClient.NearbySearch at price level %d: %v", level, err)
		}
		return got
	}

	// Keens Steakhouse (price level 4) is the one eatery inside the radius.
	atLevelFour := requestAt(POI.PriceLevelFour)
	if len(atLevelFour) != 1 || atLevelFour[0].ID != places[2].ID {
		t.Fatalf("expected only %s in radius, got %+v", places[2].Name, atLevelFour)
	}

	// A level-2 search must see it too: no fixture place has price level 2, yet the read is not
	// scoped by price.
	for _, level := range POI.AllPriceLevels {
		got := requestAt(level)
		if len(got) != len(atLevelFour) {
			t.Errorf("price level %d returned %d places, want %d — the read must not be price-scoped",
				level, len(got), len(atLevelFour))
			continue
		}
		if got[0].ID != atLevelFour[0].ID {
			t.Errorf("price level %d returned place %s, want %s", level, got[0].ID, atLevelFour[0].ID)
		}
	}
}

// The setup of this test case guarantees that the Morgan Library & Museum is within the search radius but is excluded due to temporary closure
func TestGetPlaces_shouldExcludePlacesNotOperational(t *testing.T) {
	placeSearchRequest := iowrappers.PlaceSearchRequest{
		Location:       POI.Location{Longitude: -74.0060, Latitude: 40.7128},
		PlaceCat:       POI.PlaceCategoryVisit,
		Radius:         uint(20000),
	}

	cachedVisitPlaces, _ := RedisClient.NearbySearch(RedisContext, &placeSearchRequest)

	if len(cachedVisitPlaces) != 1 || cachedVisitPlaces[0].ID != places[0].ID {
		t.Logf("number of nearby visit places obtained from Redis is %d", len(cachedVisitPlaces))
		t.Error("failed to get cached Visit place")
	}
}
