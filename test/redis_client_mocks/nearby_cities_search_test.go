package redis_client_mocks

import (
	log "github.com/sirupsen/logrus"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"reflect"
	"sort"
	"testing"
)

var expectedCities = []iowrappers.City{
	{
		ID:         "5267812",
		GeonameID:  5267812,
		Name:       "Union City",
		Latitude:   37.5934,
		Longitude:  -122.0439,
		AdminArea1: "CA",
		Country:    "United States",
	},
	{
		ID:         "5350734",
		GeonameID:  5350734,
		Name:       "Fremont",
		Latitude:   37.54827,
		Longitude:  -121.9886,
		Population: 101900,
		AdminArea1: "CA",
		Country:    "United States",
	},
	{
		ID:         "5376803",
		GeonameID:  5376803,
		Name:       "Newark",
		Latitude:   37.52966,
		Longitude:  -122.0402,
		Population: 45000,
		AdminArea1: "CA",
		Country:    "United States",
	},
}

func init() {
	if err := RedisClient.AddCities(RedisContext, expectedCities); err != nil {
		log.Error(err)
	}
}

func TestNearbyCitiesSearch_shouldReturnNearbyCities(t *testing.T) {
	var err error

	var resultCities []iowrappers.City
	// simulates a search request from Palo Alto, CA
	if resultCities, err = RedisClient.NearbyCities(RedisContext, 37.4223, -122.1329, 25.0); err != nil {
		t.Fatal(err)
	}

	if len(resultCities) != len(expectedCities) {
		t.Fatalf("expected number of city returned equals %d, got %d", len(expectedCities), len(resultCities))
	}

	sort.Slice(resultCities, func(i, j int) bool {
		return resultCities[i].ID < resultCities[j].ID
	})

	for idx, city := range resultCities {
		if !reflect.DeepEqual(city, expectedCities[idx]) {
			t.Errorf("expected city %s with ID %s, got %s with ID %s", expectedCities[idx].Name, expectedCities[idx].ID, city.Name, city.ID)
		}
	}
}

func TestAddingKnownCities_shouldUpdateRedisRecords(t *testing.T) {
	var err error
	var newCities = []iowrappers.City{
		{
			ID:         "5350734",
			GeonameID:  5350734,
			Name:       "Fremont",
			Latitude:   37.54827,
			Longitude:  -121.9886,
			Population: 151490,
			AdminArea1: "CA",
			Country:    "United States",
		},
	}

	var resultCities []iowrappers.City

	if err = RedisClient.AddCities(RedisContext, newCities); err != nil {
		t.Fatal(err)
	}

	resultCities, err = RedisClient.NearbyCities(RedisContext, 37.4223, -122.1329, 25.0)
	if err != nil {
		t.Fatal(err)
	}

	sort.Slice(resultCities, func(i, j int) bool {
		return resultCities[i].ID < resultCities[j].ID
	})

	fremont := resultCities[1]
	if fremont.Population != newCities[0].Population {
		t.Errorf("expected population of Fremont to be updated to %d, got %d", newCities[0].Population, fremont.Population)
	}
}
