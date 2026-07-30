package iowrappers

import (
	"testing"
	"time"

	"github.com/weihesdlegend/Vacation-planner/POI"
	"googlemaps.github.io/maps"
)

func brandSearchResponse() *maps.PlacesSearchResponse {
	return &maps.PlacesSearchResponse{
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
}

// storedPlace builds a cached record. A non-empty URL is what marks a record as having been
// populated by a Place Details call — see detailsSourcedFields.
func storedPlace(id string, lastUpdated time.Time) POI.Place {
	var p POI.Place
	p.SetID(id)
	p.SetURL("https://maps.google.com/?cid=" + id)
	p.SetLastUpdatedAt(lastUpdated)
	return p
}

func TestSelectPlacesForDetails(t *testing.T) {
	requestLocation := POI.Location{Latitude: 40.7484, Longitude: -73.9857}
	resp := brandSearchResponse()
	now := time.Now()

	request := &PlaceSearchRequest{
		Keyword:         "Dunkin'",
		StrictNameMatch: true,
		Location:        requestLocation,
		DetailsLimit:    1,
	}
	budget := request.DetailsLimit

	placeIdMap := selectPlacesForDetails(request, resp, &budget, nil, now)

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
	nextPage := selectPlacesForDetails(request, resp, &budget, nil, now)
	if len(nextPage) != 0 {
		t.Errorf("expect no selections once the budget is exhausted, got %v", nextPage)
	}

	// no cap: everything missing hours that matches the brand is selected, keyed by
	// (possibly sparse) result index
	uncapped := &PlaceSearchRequest{Keyword: "Dunkin'", StrictNameMatch: true, Location: requestLocation}
	unlimited := 0
	all := selectPlacesForDetails(uncapped, resp, &unlimited, nil, now)
	if len(all) != 2 || all[2] != "far" || all[3] != "near" {
		t.Errorf("expect indices 2 and 3 selected without a cap, got %v", all)
	}
}

// TestSelectPlacesForDetailsSkipsCachedPlaces covers the Place Details saving: Details is the
// dominant cost of a cold search, one call per place, and re-searching ground we already cover
// used to re-buy all of it.
func TestSelectPlacesForDetailsSkipsCachedPlaces(t *testing.T) {
	requestLocation := POI.Location{Latitude: 40.7484, Longitude: -73.9857}
	now := time.Now()
	uncapped := func() (*PlaceSearchRequest, int) {
		return &PlaceSearchRequest{Keyword: "Dunkin'", StrictNameMatch: true, Location: requestLocation}, 0
	}

	t.Run("nothing is selected when every candidate is already stored and current", func(t *testing.T) {
		request, budget := uncapped()
		cached := map[string]POI.Place{
			"far":  storedPlace("far", now.Add(-time.Hour)),
			"near": storedPlace("near", now.Add(-time.Hour)),
		}
		got := selectPlacesForDetails(request, brandSearchResponse(), &budget, cached, now)
		if len(got) != 0 {
			t.Errorf("expect no Place Details calls when everything is cached, got %v", got)
		}
	})

	t.Run("a stored record with no Details data is still selected", func(t *testing.T) {
		request, budget := uncapped()
		var thin POI.Place
		thin.SetID("near")
		thin.SetLastUpdatedAt(now)
		cached := map[string]POI.Place{"near": thin}
		got := selectPlacesForDetails(request, brandSearchResponse(), &budget, cached, now)
		if got[3] != "near" {
			t.Errorf("expect a record with no URL to still need details, got %v", got)
		}
	})

	// Without this, skipping on mere existence would freeze a record permanently: the external
	// search refresh is the only thing that ever rewrites it.
	t.Run("a stale stored record is refreshed", func(t *testing.T) {
		request, budget := uncapped()
		cached := map[string]POI.Place{
			"near": storedPlace("near", now.Add(-PlaceDetailsRefreshDuration-time.Hour)),
		}
		got := selectPlacesForDetails(request, brandSearchResponse(), &budget, cached, now)
		if got[3] != "near" {
			t.Errorf("expect a stale record to be refreshed, got %v", got)
		}
	})

	t.Run("a record with an unparsable timestamp is refreshed", func(t *testing.T) {
		request, budget := uncapped()
		stored := storedPlace("near", now)
		stored.LastUpdatedAt = "not-a-timestamp"
		cached := map[string]POI.Place{"near": stored}
		got := selectPlacesForDetails(request, brandSearchResponse(), &budget, cached, now)
		if got[3] != "near" {
			t.Errorf("expect a record that cannot be aged to be refreshed, got %v", got)
		}
	})

	// The cache filter must run BEFORE the budget cap. "near" is the closest candidate, so under
	// the reverse ordering it would win the single budgeted slot and "far" — the one place we do
	// not have — would be dropped.
	t.Run("a limited budget is spent on places we do not have", func(t *testing.T) {
		request := &PlaceSearchRequest{
			Keyword:         "Dunkin'",
			StrictNameMatch: true,
			Location:        requestLocation,
			DetailsLimit:    1,
		}
		budget := request.DetailsLimit
		cached := map[string]POI.Place{"near": storedPlace("near", now.Add(-time.Hour))}
		got := selectPlacesForDetails(request, brandSearchResponse(), &budget, cached, now)
		if len(got) != 1 || got[2] != "far" {
			t.Errorf("expect the budget spent on the uncached place 'far', got %v", got)
		}
	})
}

// TestRestoreCachedDetails pins the half of the optimisation that protects the data: the write
// path is a blind upsert, so a place whose Details call was skipped must not be written back
// stripped of the fields it was skipped because of.
func TestRestoreCachedDetails(t *testing.T) {
	stored := storedPlace("near", time.Now())
	stored.Summary = "A donut shop."
	stored.FormattedAddress = "1 Main St, New York, NY 10001, USA"
	stored.Hours = [7]string{"Monday: 6AM-9PM", "", "", "", "", "", ""}

	t.Run("a place rebuilt without details recovers its stored fields", func(t *testing.T) {
		// what parsePlacesSearchResponse produces for a skipped place: default hours, no URL
		rebuilt := POI.Place{ID: "near", Hours: [7]string{"8:30 am – 9:30 pm"}}
		places := []POI.Place{rebuilt}

		restoreCachedDetails(places, map[string]POI.Place{"near": stored})

		if places[0].URL != stored.URL {
			t.Errorf("URL = %q, want %q", places[0].URL, stored.URL)
		}
		if places[0].Summary != stored.Summary {
			t.Errorf("Summary = %q, want %q", places[0].Summary, stored.Summary)
		}
		if places[0].FormattedAddress != stored.FormattedAddress {
			t.Errorf("FormattedAddress = %q, want %q", places[0].FormattedAddress, stored.FormattedAddress)
		}
		if places[0].Hours != stored.Hours {
			t.Errorf("Hours = %v, want %v — the default hours would have overwritten real ones", places[0].Hours, stored.Hours)
		}
	})

	t.Run("a freshly detailed place keeps its new data", func(t *testing.T) {
		fresh := POI.Place{
			ID:               "near",
			URL:              "https://maps.google.com/?cid=fresh",
			Summary:          "Newly fetched.",
			FormattedAddress: "2 Second St",
			Hours:            [7]string{"Monday: 5AM-10PM"},
		}
		places := []POI.Place{fresh}

		restoreCachedDetails(places, map[string]POI.Place{"near": stored})

		if places[0].URL != fresh.URL || places[0].Summary != fresh.Summary || places[0].Hours != fresh.Hours {
			t.Errorf("a place with fresh details was overwritten from cache: %+v", places[0])
		}
	})

	t.Run("a place we have never stored is left alone", func(t *testing.T) {
		places := []POI.Place{{ID: "unknown"}}
		restoreCachedDetails(places, map[string]POI.Place{"near": stored})
		if places[0].URL != "" {
			t.Errorf("URL = %q, want empty", places[0].URL)
		}
	})

	// A place whose hours arrived with the Nearby response is skipped for Details and so has no
	// URL. Restoring must not mistake that for "no data" and copy the stored record's placeholder
	// hours over the real ones.
	t.Run("real hours from the nearby response survive a stored placeholder", func(t *testing.T) {
		var placeholderStored POI.Place
		placeholderStored.SetID("near")
		placeholderStored.SetURL("https://maps.google.com/?cid=near")
		for day := POI.DateMonday; day <= POI.DateSunday; day++ {
			placeholderStored.SetHour(day, POI.DefaultOpeningHours)
		}

		fromNearby := POI.Place{ID: "near"}
		for day := POI.DateMonday; day <= POI.DateSunday; day++ {
			fromNearby.SetHour(day, "Monday: 7AM-11PM")
		}
		places := []POI.Place{fromNearby}

		restoreCachedDetails(places, map[string]POI.Place{"near": placeholderStored})

		if places[0].Hours != fromNearby.Hours {
			t.Errorf("Hours = %v, want %v — placeholder hours overwrote real ones", places[0].Hours, fromNearby.Hours)
		}
		// the URL is still a genuine gap and should be filled
		if places[0].URL != placeholderStored.URL {
			t.Errorf("URL = %q, want %q", places[0].URL, placeholderStored.URL)
		}
	})
}
