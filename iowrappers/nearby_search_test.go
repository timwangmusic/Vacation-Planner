package iowrappers

import (
	"testing"

	"github.com/weihesdlegend/Vacation-planner/POI"
	"googlemaps.github.io/maps"
)

func TestSelectPlacesForDetails(t *testing.T) {
	requestLocation := POI.Location{Latitude: 40.7484, Longitude: -73.9857}
	resp := &maps.PlacesSearchResponse{
		Results: []maps.PlacesSearchResult{
			{ // 0: has hours already, never needs details
				Name:         "Dunkin'",
				PlaceID:      "has-hours",
				Geometry:     maps.AddressGeometry{Location: maps.LatLng{Lat: 40.7485, Lng: -73.9858}},
				OpeningHours: &maps.OpeningHours{WeekdayText: []string{"Monday: 6AM-9PM"}},
			},
			{ // 1: name does not match the brand, details would be wasted spend
				Name:     "Dunham's Sports",
				PlaceID:  "wrong-brand",
				Geometry: maps.AddressGeometry{Location: maps.LatLng{Lat: 40.7486, Lng: -73.9859}},
			},
			{ // 2: matching brand, far from the request location
				Name:     "Dunkin' Donuts",
				PlaceID:  "far",
				Geometry: maps.AddressGeometry{Location: maps.LatLng{Lat: 40.8000, Lng: -73.9500}},
			},
			{ // 3: matching brand, nearest candidate
				Name:     "Dunkin'",
				PlaceID:  "near",
				Geometry: maps.AddressGeometry{Location: maps.LatLng{Lat: 40.7490, Lng: -73.9860}},
			},
		},
	}

	request := &PlaceSearchRequest{
		Keyword:         "Dunkin'",
		StrictNameMatch: true,
		Location:        requestLocation,
		DetailsLimit:    1,
	}
	budget := request.DetailsLimit

	placeIdMap := selectPlacesForDetails(request, resp, &budget)

	if len(placeIdMap) != 1 {
		t.Fatalf("expect 1 place selected for details, got %d: %v", len(placeIdMap), placeIdMap)
	}
	if placeIdMap[3] != "near" {
		t.Errorf("expect the nearest matching place (index 3, id 'near') to be selected, got %v", placeIdMap)
	}
	if budget != 0 {
		t.Errorf("expect details budget to be exhausted, got %d", budget)
	}

	// budget exhausted: subsequent pages select nothing
	nextPage := selectPlacesForDetails(request, resp, &budget)
	if len(nextPage) != 0 {
		t.Errorf("expect no selections once the budget is exhausted, got %v", nextPage)
	}

	// no cap: everything missing hours that matches the brand is selected, keyed by
	// (possibly sparse) result index
	uncapped := &PlaceSearchRequest{Keyword: "Dunkin'", StrictNameMatch: true, Location: requestLocation}
	unlimited := 0
	all := selectPlacesForDetails(uncapped, resp, &unlimited)
	if len(all) != 2 || all[2] != "far" || all[3] != "near" {
		t.Errorf("expect indices 2 and 3 selected without a cap, got %v", all)
	}
}
