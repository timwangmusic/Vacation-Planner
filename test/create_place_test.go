package test

import (
	"testing"

	"github.com/weihesdlegend/Vacation-planner/POI"
	"googlemaps.github.io/maps"
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
	hours := &POI.OpeningHours{Hours: []string{
		"10AM-7PM",
		"Closed",
		"Closed",
		"10AM-7PM",
		"10AM-7PM",
		"10AM-7PM",
		"10AM-7PM",
	}}
	expectedHours := [7]string{
		"10AM-7PM",
		"Closed",
		"Closed",
		"10AM-7PM",
		"10AM-7PM",
		"10AM-7PM",
		"10AM-7PM",
	}
	expectedPhoto := &maps.Photo{
		PhotoReference:   "xyz33521",
		Height:           500,
		Width:            500,
		HTMLAttributions: nil,
	}
	place := POI.CreatePlace(name, microAddr, addr, "OPERATIONAL", "stay", hours, "landmark_mtv", POI.PriceLevelThree, 4.5, "", expectedPhoto, 0, expectedLatitude, expectedLongitude, &editorialSummary)
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
	if place.Hours != expectedHours {
		t.Errorf("expected hours equals %v, got %v", expectedHours, place.Hours)
	}
	storedPhoto := place.GetPhoto()
	if storedPhoto.Width != expectedPhoto.Width {
		t.Errorf("stored photo width does not match expected")
	}
	if storedPhoto.Height != expectedPhoto.Height {
		t.Errorf("stored photo height does not match expected")
	}
	if storedPhoto.Reference != expectedPhoto.PhotoReference {
		t.Errorf("stored photo reference does not match expected")
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
