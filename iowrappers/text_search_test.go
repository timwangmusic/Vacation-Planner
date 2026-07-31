package iowrappers

import (
	"encoding/json"
	"testing"

	"github.com/weihesdlegend/Vacation-planner/POI"
	"googlemaps.github.io/maps"
)

func mustLogger(t *testing.T) {
	t.Helper()
	if err := CreateLogger(); err != nil {
		t.Fatalf("CreateLogger: %v", err)
	}
}

func TestParseTextSearchResponse_SkipsEmptyPlaceID(t *testing.T) {
	mustLogger(t)
	resp := maps.PlacesSearchResponse{
		Results: []maps.PlacesSearchResult{
			{PlaceID: "", Name: "No ID", Geometry: maps.AddressGeometry{Location: maps.LatLng{Lat: 1, Lng: 1}}},
			{PlaceID: "has-id", Name: "Has ID", Geometry: maps.AddressGeometry{Location: maps.LatLng{Lat: 1, Lng: 1}}},
		},
	}

	places := parseTextSearchResponse(resp, 0)
	if len(places) != 1 {
		t.Fatalf("got %d places, want 1", len(places))
	}
	if places[0].ID != "has-id" {
		t.Errorf("got place %q, want has-id", places[0].ID)
	}
}

func TestParseTextSearchResponse_SkipsZeroGeometry(t *testing.T) {
	mustLogger(t)
	resp := maps.PlacesSearchResponse{
		Results: []maps.PlacesSearchResult{
			{PlaceID: "zero-geo", Name: "Null Island", Geometry: maps.AddressGeometry{Location: maps.LatLng{Lat: 0, Lng: 0}}},
			{PlaceID: "real-geo", Name: "Real Place", Geometry: maps.AddressGeometry{Location: maps.LatLng{Lat: 12.5, Lng: -70.1}}},
		},
	}

	places := parseTextSearchResponse(resp, 0)
	if len(places) != 1 {
		t.Fatalf("got %d places, want 1", len(places))
	}
	if places[0].ID != "real-geo" {
		t.Errorf("got place %q, want real-geo", places[0].ID)
	}
}

func TestParseTextSearchResponse_DedupesByPlaceID(t *testing.T) {
	mustLogger(t)
	geo := maps.AddressGeometry{Location: maps.LatLng{Lat: 1, Lng: 1}}
	resp := maps.PlacesSearchResponse{
		Results: []maps.PlacesSearchResult{
			{PlaceID: "dup-1", Name: "First", Geometry: geo},
			{PlaceID: "dup-1", Name: "Second (duplicate)", Geometry: geo},
		},
	}

	places := parseTextSearchResponse(resp, 0)
	if len(places) != 1 {
		t.Fatalf("got %d places, want 1 (dedupe by PlaceID failed)", len(places))
	}
	if places[0].Name != "First" {
		t.Errorf("got name %q, want First (first occurrence should win)", places[0].Name)
	}
}

// TestParseTextSearchResponse_KeepsZeroRatings pins the deliberate divergence from
// parsePlacesSearchResponse (nearby search), which drops UserRatingsTotal == 0. That filter would
// drop exactly the new/obscure places this feature exists to let a user add by name.
func TestParseTextSearchResponse_KeepsZeroRatings(t *testing.T) {
	mustLogger(t)
	resp := maps.PlacesSearchResponse{
		Results: []maps.PlacesSearchResult{
			{
				PlaceID:          "zero-ratings",
				Name:             "Brand New Place",
				Geometry:         maps.AddressGeometry{Location: maps.LatLng{Lat: 1, Lng: 1}},
				UserRatingsTotal: 0,
			},
		},
	}

	places := parseTextSearchResponse(resp, 0)
	if len(places) != 1 {
		t.Fatalf("got %d places, want 1 (zero-rating place must be kept)", len(places))
	}
}

// TestParseTextSearchResponse_BlankStatusBecomesOperational pins that an empty business_status
// (common for text search) is treated as Operational rather than as StatusNotAvailable, which
// would make the place invisible under RedisClient.NearbySearch's Operational-only filter.
func TestParseTextSearchResponse_BlankStatusBecomesOperational(t *testing.T) {
	mustLogger(t)
	resp := maps.PlacesSearchResponse{
		Results: []maps.PlacesSearchResult{
			{
				PlaceID:        "blank-status",
				Name:           "No Status Given",
				Geometry:       maps.AddressGeometry{Location: maps.LatLng{Lat: 1, Lng: 1}},
				BusinessStatus: "",
			},
		},
	}

	places := parseTextSearchResponse(resp, 0)
	if len(places) != 1 {
		t.Fatalf("got %d places, want 1", len(places))
	}
	if places[0].Status != POI.Operational {
		t.Errorf("got status %q, want %q", places[0].Status, POI.Operational)
	}
}

func TestParseTextSearchResponse_SkipsClosedPermanently(t *testing.T) {
	mustLogger(t)
	resp := maps.PlacesSearchResponse{
		Results: []maps.PlacesSearchResult{
			{
				PlaceID:        "closed",
				Name:           "Shuttered",
				Geometry:       maps.AddressGeometry{Location: maps.LatLng{Lat: 1, Lng: 1}},
				BusinessStatus: "CLOSED_PERMANENTLY",
			},
			{
				PlaceID:        "open",
				Name:           "Still Open",
				Geometry:       maps.AddressGeometry{Location: maps.LatLng{Lat: 1, Lng: 1}},
				BusinessStatus: "OPERATIONAL",
			},
		},
	}

	places := parseTextSearchResponse(resp, 0)
	if len(places) != 1 {
		t.Fatalf("got %d places, want 1", len(places))
	}
	if places[0].ID != "open" {
		t.Errorf("got place %q, want open", places[0].ID)
	}
}

func TestParseTextSearchResponse_TruncatesToDefaultMax(t *testing.T) {
	mustLogger(t)
	results := make([]maps.PlacesSearchResult, 0, PlaceTextSearchMaxResults+5)
	for i := 0; i < PlaceTextSearchMaxResults+5; i++ {
		results = append(results, maps.PlacesSearchResult{
			PlaceID:  placeIDFor(i),
			Name:     "Place",
			Geometry: maps.AddressGeometry{Location: maps.LatLng{Lat: 1, Lng: 1}},
		})
	}
	resp := maps.PlacesSearchResponse{Results: results}

	places := parseTextSearchResponse(resp, 0) // limit <= 0 -> PlaceTextSearchMaxResults
	if len(places) != PlaceTextSearchMaxResults {
		t.Fatalf("got %d places, want %d (limit<=0 must fall back to PlaceTextSearchMaxResults)", len(places), PlaceTextSearchMaxResults)
	}
}

func TestParseTextSearchResponse_TruncatesToRequestedLimit(t *testing.T) {
	mustLogger(t)
	results := make([]maps.PlacesSearchResult, 0, PlaceTextSearchMaxResults)
	for i := 0; i < PlaceTextSearchMaxResults; i++ {
		results = append(results, maps.PlacesSearchResult{
			PlaceID:  placeIDFor(i),
			Name:     "Place",
			Geometry: maps.AddressGeometry{Location: maps.LatLng{Lat: 1, Lng: 1}},
		})
	}
	resp := maps.PlacesSearchResponse{Results: results}

	places := parseTextSearchResponse(resp, 5)
	if len(places) != 5 {
		t.Fatalf("got %d places, want 5", len(places))
	}
}

func TestParseTextSearchResponse_LimitAboveMaxStillCapsAtMax(t *testing.T) {
	mustLogger(t)
	results := make([]maps.PlacesSearchResult, 0, PlaceTextSearchMaxResults+5)
	for i := 0; i < PlaceTextSearchMaxResults+5; i++ {
		results = append(results, maps.PlacesSearchResult{
			PlaceID:  placeIDFor(i),
			Name:     "Place",
			Geometry: maps.AddressGeometry{Location: maps.LatLng{Lat: 1, Lng: 1}},
		})
	}
	resp := maps.PlacesSearchResponse{Results: results}

	places := parseTextSearchResponse(resp, PlaceTextSearchMaxResults+100)
	if len(places) != PlaceTextSearchMaxResults {
		t.Fatalf("got %d places, want %d (a limit above the max must still cap at the max)", len(places), PlaceTextSearchMaxResults)
	}
}

// TestParseTextSearchResponse_PreservesTypesAndSetsLocationTypeToPrimary pins that Types is kept
// verbatim and LocationType is set to the primary (first non-umbrella) type, not the raw first
// entry and not left blank.
func TestParseTextSearchResponse_PreservesTypesAndSetsLocationTypeToPrimary(t *testing.T) {
	mustLogger(t)
	types := []string{"point_of_interest", "museum", "establishment"}
	resp := maps.PlacesSearchResponse{
		Results: []maps.PlacesSearchResult{
			{
				PlaceID:  "typed-place",
				Name:     "History Museum",
				Geometry: maps.AddressGeometry{Location: maps.LatLng{Lat: 1, Lng: 1}},
				Types:    types,
			},
		},
	}

	places := parseTextSearchResponse(resp, 0)
	if len(places) != 1 {
		t.Fatalf("got %d places, want 1", len(places))
	}
	got := places[0]
	if len(got.Types) != len(types) {
		t.Fatalf("Types not preserved: got %+v, want %+v", got.Types, types)
	}
	for i := range types {
		if got.Types[i] != types[i] {
			t.Fatalf("Types not preserved verbatim: got %+v, want %+v", got.Types, types)
		}
	}
	if got.LocationType != POI.LocationTypeMuseum {
		t.Errorf("LocationType = %q, want the primary type %q", got.LocationType, POI.LocationTypeMuseum)
	}
}

// TestParseTextSearchResponse_NoRealOpeningHours pins that a text-search-derived place never has
// real opening hours (no weekday data exists in the Text Search response), which is precisely why
// confirming a candidate buys a Place Details call in AddSearchedPlaceToCache.
func TestParseTextSearchResponse_NoRealOpeningHours(t *testing.T) {
	mustLogger(t)
	resp := maps.PlacesSearchResponse{
		Results: []maps.PlacesSearchResult{
			{
				PlaceID:  "no-hours",
				Name:     "Mystery Hours",
				Geometry: maps.AddressGeometry{Location: maps.LatLng{Lat: 1, Lng: 1}},
			},
		},
	}

	places := parseTextSearchResponse(resp, 0)
	if len(places) != 1 {
		t.Fatalf("got %d places, want 1", len(places))
	}
	if places[0].HasRealOpeningHours() {
		t.Errorf("HasRealOpeningHours() = true, want false (text search carries no weekday hours)")
	}
}

func placeIDFor(i int) string {
	return "place-" + string(rune('a'+i%26)) + string(rune('0'+i/26))
}

// TestPlaceSearchCandidate_MarshalsExpectedTopLevelKeys and
// TestAddSearchedPlaceResult_MarshalsExpectedTopLevelKeys pin the wire contract the external
// Convex client depends on: PlaceSearchCandidate/AddSearchedPlaceResult must serialize with
// exactly the documented camelCase top-level keys, and the nested Place object must keep
// POI.Place's own (capitalized, untagged) field names, since POI.Place deliberately carries no
// json tags of its own — tagging it would change every endpoint's wire shape at once, not just
// these two. Purely in-memory json.Marshal/Unmarshal: no HTTP, no network, no Redis.
func TestPlaceSearchCandidate_MarshalsExpectedTopLevelKeys(t *testing.T) {
	candidate := PlaceSearchCandidate{
		Place:      POI.Place{ID: "p1", Name: "History Museum", LocationType: POI.LocationTypeMuseum, Types: []string{"museum", "point_of_interest"}},
		Category:   POI.PlaceCategoryVisit,
		Insertable: true,
	}

	data, err := json.Marshal(candidate)
	if err != nil {
		t.Fatalf("json.Marshal(PlaceSearchCandidate): %v", err)
	}

	assertTopLevelKeys(t, data, []string{"place", "category", "insertable"})
	assertPlaceKeysPresent(t, data)
}

func TestAddSearchedPlaceResult_MarshalsExpectedTopLevelKeys(t *testing.T) {
	result := AddSearchedPlaceResult{
		Place:         POI.Place{ID: "p1", Name: "History Museum", LocationType: POI.LocationTypeMuseum, Types: []string{"museum", "point_of_interest"}},
		Category:      POI.PlaceCategoryVisit,
		AlreadyCached: false,
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal(AddSearchedPlaceResult): %v", err)
	}

	assertTopLevelKeys(t, data, []string{"place", "category", "alreadyCached"})
	assertPlaceKeysPresent(t, data)
}

// assertTopLevelKeys asserts that data's top-level JSON object has exactly the given key set —
// not a subset, not a superset, so an accidental new/dropped field on either struct fails here.
func assertTopLevelKeys(t *testing.T, data []byte, want []string) {
	t.Helper()
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("json.Unmarshal top level: %v", err)
	}
	if len(raw) != len(want) {
		t.Fatalf("got %d top-level keys %v, want %d %v", len(raw), keysOf(raw), len(want), want)
	}
	for _, k := range want {
		if _, ok := raw[k]; !ok {
			t.Errorf("missing top-level key %q, got keys %v", k, keysOf(raw))
		}
	}
}

// assertPlaceKeysPresent asserts the nested "place" object carries POI.Place's untagged
// (capitalized) field names, pinning that POI.Place itself stays free of json tags.
func assertPlaceKeysPresent(t *testing.T, data []byte) {
	t.Helper()
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("json.Unmarshal top level: %v", err)
	}
	placeRaw, ok := raw["place"]
	if !ok {
		t.Fatal(`missing "place" top-level key`)
	}
	var placeFields map[string]json.RawMessage
	if err := json.Unmarshal(placeRaw, &placeFields); err != nil {
		t.Fatalf("json.Unmarshal nested place object: %v", err)
	}
	for _, key := range []string{"ID", "Name", "LocationType", "Types"} {
		if _, ok := placeFields[key]; !ok {
			t.Errorf("place object missing capitalized key %q (external Convex client wire contract); got keys %v", key, keysOf(placeFields))
		}
	}
}

func keysOf(m map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
