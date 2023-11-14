package test

import (
	"github.com/weihesdlegend/Vacation-planner/POI"
	"testing"
)

func TestCreatePlace(t *testing.T) {
	name := "The Beat Museum"
	addr := "540 Broadway, San Francisco, USA, 94113"
	microAddr := `<span class="street-address">540 Broadway</span>,
				<span class="locality">San Francisco</span>,
				<span class="region">CA</span> <span class="postal-code">94133-4507</span>,
				<span class="country-name">USA</span>`

	expectedLatitude := 32.715736
	expectedLongitude := -117.161087
	editorialSummary := "The Beat Museum is dedicated to preserving the memory and works of the Beat Generation."
	place := POI.CreatePlace(name, microAddr, addr, "OPERATIONAL", "stay", nil, "landmark_mtv", 3, 4.5, "", nil, 0, expectedLatitude, expectedLongitude, &editorialSummary)
	if place.GetName() != name {
		t.Errorf("Name setting is not correct. \n Expected: %s, got: %s",
			name, place.GetName())
	}
	if place.GetStatus() != POI.Operational {
		t.Errorf("Expected business status: %s, got %s", POI.Operational, place.GetStatus())
	}
	if place.GetLocation().Longitude != expectedLongitude {
		t.Errorf("Location longitude setting is not correct. Expected %f, got %f", expectedLongitude, place.GetLocation().Longitude)
	}

	if place.GetLocation().Latitude != expectedLatitude {
		t.Errorf("Location longitude setting is not correct. Expected %f, got %f", expectedLatitude, place.GetLocation().Latitude)
	}

	if place.GetType() != "stay" {
		t.Errorf("Type setting is not correct.")
	}
	if place.GetFormattedAddress() != addr {
		t.Errorf("Address setting is not correct. \n Expected: %s \n Got: %s",
			addr, place.GetAddress())
	}
	if place.GetPriceLevel() != POI.PriceLevelThree {
		t.Errorf("Price level setting is not correct. \n Expected: %d \n Got: %+v",
			3, place.GetPriceLevel())
	}
	if place.GetRating() != 4.5 {
		t.Errorf("Price rating setting is not correct. \n Expected: %f \n Got: %f	",
			4.5, place.GetRating())
	}
	if place.GetSummary() != editorialSummary {
		t.Errorf("expected place editorialSummary is %s, got: %s", editorialSummary, place.GetSummary())
	}
	retMicroAddr := place.GetAddress()
	if retMicroAddr.StreetAddr != "540 Broadway" {
		t.Errorf("micro address street address parsing error. \n Expected: 540 Broadway \n Got %s", retMicroAddr.StreetAddr)
	}
	if retMicroAddr.Country != "USA" {
		t.Errorf("micro address country parsing error. \n Expected: USA \n Got %s", retMicroAddr.Country)
	}
	if retMicroAddr.Locality != "San Francisco" {
		t.Errorf("micro address locality parsing error. \n Expected: San Francisco \n Got %s", retMicroAddr.Locality)
	}
	if retMicroAddr.PostalCode != "94133-4507" {
		t.Errorf("micro address postal code parsing error. \n Expected: 94133-4507 \n Got %s", retMicroAddr.PostalCode)
	}
}
