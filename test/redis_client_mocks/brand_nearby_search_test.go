package redis_client_mocks

import (
	"testing"
	"time"

	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
)

var brandPlaces = []POI.Place{
	{
		ID:               "5005",
		Name:             "Dunkin'",
		LocationType:     POI.LocationTypeCafe,
		Address:          POI.Address{},
		FormattedAddress: "1 Herald Sq, New York, NY 10001",
		Location:         POI.Location{Longitude: -73.9876, Latitude: 40.7496},
		PriceLevel:       POI.PriceLevelOne,
		Rating:           4.1,
		Hours:            [7]string{},
		Status:           POI.Operational,
	},
	{
		ID:               "6006",
		Name:             "Dunkin' Donuts",
		LocationType:     POI.LocationTypeCafe,
		Address:          POI.Address{},
		FormattedAddress: "255 Northern Blvd, Great Neck, NY 11021",
		Location:         POI.Location{Longitude: -73.7271, Latitude: 40.7773},
		PriceLevel:       POI.PriceLevelOne,
		Rating:           4.0,
		Hours:            [7]string{},
		Status:           POI.Operational,
	},
}

func TestBrandNearbySearch_shouldOnlyReturnPlacesFromBrandBucket(t *testing.T) {
	RedisClient.SetPlacesAddGeoLocationsForBrand(RedisContext, "Dunkin'", brandPlaces)

	req := &iowrappers.PlaceSearchRequest{
		Keyword:  "Dunkin'",
		Location: POI.Location{Longitude: -74.0060, Latitude: 40.7128},
		Radius:   uint(8000),
	}

	cachedBrandPlaces, err := RedisClient.NearbySearch(RedisContext, req)
	if err != nil {
		t.Fatalf("RedisClient.NearbySearch error %v", err)
	}

	// only the Herald Square Dunkin' is within the search radius; the category-bucket
	// places seeded by other tests (e.g. Keens Steakhouse nearby) must not appear
	if len(cachedBrandPlaces) != 1 {
		t.Fatalf("expect to have 1 place, but got %d instead", len(cachedBrandPlaces))
	}
	if cachedBrandPlaces[0].ID != brandPlaces[0].ID {
		t.Errorf("expect to get %s, but got %s instead", brandPlaces[0].Name, cachedBrandPlaces[0].Name)
	}
}

func TestBrandMapsLastSearchTime_roundTrip(t *testing.T) {
	location := POI.Location{City: "New York", AdminAreaLevelOne: "NY", Country: "USA"}
	currentTime := time.Now()

	if err := RedisClient.SetBrandMapsLastSearchTime(RedisContext, location, "Dunkin'", currentTime.Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}

	gotLastSearchTime, err := RedisClient.GetBrandMapsLastSearchTime(RedisContext, location, "Dunkin'")
	if err != nil {
		t.Fatal(err)
	}
	if gotLastSearchTime.Format(time.RFC3339) != currentTime.Format(time.RFC3339) {
		t.Errorf("expect last search time %v, got %v", currentTime, gotLastSearchTime)
	}
}

func TestMatchesBrandName(t *testing.T) {
	tests := []struct {
		placeName string
		keyword   string
		want      bool
	}{
		{"Dunkin' Donuts #1234", "Dunkin'", true},
		{"Dunkin'", "Dunkin'", true},
		{"Saks Fifth Avenue", "Saks Fifth Avenue", true},
		{"Saks OFF 5TH", "Saks Fifth Avenue", false},
		{"Dunham's Sports", "Dunkin'", false},
		{"Starbucks", "", false},
	}
	for _, tt := range tests {
		if got := iowrappers.MatchesBrandName(tt.placeName, tt.keyword); got != tt.want {
			t.Errorf("MatchesBrandName(%q, %q) = %v, want %v", tt.placeName, tt.keyword, got, tt.want)
		}
	}
}

func TestNormalizeBrandKey(t *testing.T) {
	tests := []struct {
		keyword string
		want    string
	}{
		{"Dunkin'", "dunkin"},
		{"Dunkin' Donuts", "dunkin-donuts"},
		{"Saks Fifth Avenue", "saks-fifth-avenue"},
		{"  The Home Depot  ", "the-home-depot"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := POI.NormalizeBrandKey(tt.keyword); got != tt.want {
			t.Errorf("NormalizeBrandKey(%q) = %q, want %q", tt.keyword, got, tt.want)
		}
	}
}
