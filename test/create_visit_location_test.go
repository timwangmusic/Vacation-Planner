package test

import (
	"github.com/weihesdlegend/Vacation-planner/POI"
	"testing"
)

func TestCreatePlace(t *testing.T) {
	location := "32.715736,-117.161087"
	name := "The Beat Museum"
	addr := "540 Broadway, San Francisco, USA, 94113"
	microAddr := `<span class="street-address">540 Broadway</span>,
				<span class="locality">San Francisco</span>,
				<span class="region">CA</span> <span class="postal-code">94133-4507</span>,
				<span class="country-name">USA</span>`

	place := POI.CreatePlace(name, location, microAddr, addr, "stay", nil, "lincolnpark_mtv", 3, 4.5)
	if place.GetName() != name {
		t.Errorf("Name setting is not correct. \n Expected: %s, got: %s",
			name, place.GetName())
	}
	if place.GetLocation() != [2]float64{-117.161087, 32.715736} {
		t.Errorf("Location setting is not correct.")
	}
	if place.GetType() != "stay" {
		t.Errorf("Type setting is not correct.")
	}
	if place.GetFormattedAddress() != addr {
		t.Errorf("Address setting is not correct. \n Expected: %s \n Got: %s",
			addr, place.GetAddress())
	}
	if place.GetPriceLevel() != 3 {
		t.Errorf("Price level setting is not correct. \n Expected: %d \n Got: %d",
			3, place.GetPriceLevel())
	}
	if place.GetRating() != 4.5 {
		t.Errorf("Price rating setting is not correct. \n Expected: %f \n Got: %f	",
			4.5, place.GetRating())
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
