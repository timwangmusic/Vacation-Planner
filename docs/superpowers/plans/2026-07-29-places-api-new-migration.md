# Places API (New) Migration — Search and Photos Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move the category search and place-photo paths off the legacy Places API (`maps.googleapis.com/maps/api/place/*`) onto Places API (New) (`places.googleapis.com/v1`), using `places:searchNearby` with `includedPrimaryTypes` so place types are filtered server-side by primary type, and bucketing results by the `priceLevel` the response already returns.

**Architecture:** A new self-contained `iowrappers/placesv1` package speaks the New API over `net/http` — `googlemaps.github.io/maps` v1.7.0 has no support for it and the v1.7.0 client stays in place for Geocoding (which is not deprecated) and for the brand/keyword path (deferred to a follow-up). A `PLACES_API_VERSION` env flag selects legacy or new at runtime, and the new path writes under versioned Redis keys (`v2:placeIDs:*`, `v2:place_details:*`) so both datasets coexist and cutover is reversible by flipping one Heroku config var.

**Tech Stack:** Go 1.24, `net/http` + `encoding/json` (no new dependencies), Gin, go-redis v9, `googlemaps.github.io/maps` v1.7.0 (retained for Geocoding only).

## Global Constraints

- Go version: `1.24.0` (from `go.mod`) — do not raise it.
- **Add no new module dependencies.** The New API is plain REST; use `net/http`. Do not add a Google client library.
- CI gates are exactly `go build -v .` then `go test -v ./...` (`.github/workflows/go.yml`). Both must pass.
- Target branch is `origin/master`. `origin/main` is stale at `1a1417b` and is not the deploy path.
- **Prerequisite:** the fixes in `2026-07-29-eatery-place-type-fixes.md` must be merged and deployed first. This plan re-adds `fast_food_restaurant` and `food_court` in Task 6, which is only safe once type validation and the `(PlaceCategory, bool)` signature exist.
- Every New API request MUST send an `X-Goog-FieldMask` header. There is no default field list; a missing mask is an error, and an over-broad mask is billed at a higher SKU.
- Do not migrate Geocoding or ReverseGeocode. The Geocoding API (`/maps/api/geocode/json`) is a separate, non-deprecated API and stays on the v1.7.0 SDK.
- Do not migrate the brand/keyword search path in this PR. `searchNearby` has no keyword parameter; that path needs `places:searchText` and is deferred.
- No unit test may make a real network call. Use `httptest` with recorded response bodies.

---

## Why this migration, and what it buys

Verified constraints of each endpoint (Google reference docs, checked 2026-07-29):

| Capability | Legacy nearbysearch | `places:searchNearby` (New) | `places:searchText` (New) |
| --- | --- | --- | --- |
| Types per call | 1 (`type`) | **many** (`includedTypes`, `includedPrimaryTypes`) | 1 (`includedType`) |
| Max results | 20/page, 3 pages | **20, no pagination** | 20/page, 60 total |
| Price filter | `minprice`/`maxprice` | none | `priceLevels` |
| Keyword | `keyword` | none | `textQuery` |
| Rank | `rankby` | `rankPreference: DISTANCE\|POPULARITY` | `rankPreference: DISTANCE\|RELEVANCE` |
| Radius cap | 50000 m | 50000 m | n/a (bias/restriction) |

What the chosen approach fixes or improves:

1. **`fast_food_restaurant` and `food_court` become real.** Both are valid Table A types in the New API. `includedPrimaryTypes` filters by *primary* type server-side — which is exactly what `POI.ReclassifyForCategory` currently approximates client-side after the fact.
2. **One HTTP call replaces up to 35.** Today a cold Eatery search issues 5-7 Nearby Searches per round for up to 5 rounds (`GoogleMapsSearchCallMaxCount = 5`). The new path issues one `searchNearby` with all types in `includedPrimaryTypes`.
3. **The separate Place Details fan-out disappears for search results.** `searchNearby` returns opening hours, `adrFormatAddress`, `googleMapsUri`, `userRatingCount`, `editorialSummary` and `photos` directly via the field mask. `searchPlaceDetails` and its `detailsBudget` (`iowrappers/nearby_search.go:150`) are not needed on the new path.
4. **`rankPreference: DISTANCE` fixes ordering at the source**, complementing the client-side sort added in the prior PR.

**The cost, stated plainly:** a hard cap of 20 results per search versus roughly 100-140 raw results today. Task 5 measures this against production data on real cities and gates the cutover on the result. If coverage is unacceptable, the mitigation is already designed in: `SearchNearby` accepts *groups* of types, so splitting `includedPrimaryTypes` into one group per type restores today's ~140-result ceiling at 7 calls — still far cheaper than today's 35, and still with correct server-side primary-type filtering. Do not skip Task 5.

---

## File Structure

| File | Responsibility |
| --- | --- |
| `iowrappers/placesv1/client.go` (new) | HTTP transport for `places.googleapis.com/v1`: auth header, field mask, timeouts, error decoding. Knows nothing about POI types. |
| `iowrappers/placesv1/types.go` (new) | Request/response structs mirroring the New API JSON exactly (`Place`, `LocalizedText`, `OpeningHours`, `Photo`, enums). |
| `iowrappers/placesv1/search_nearby.go` (new) | `SearchNearby` request building and the multi-group fan-out. |
| `iowrappers/placesv1/photo.go` (new) | Builds the `/v1/{photoName}/media` URL and fetches image bytes. |
| `iowrappers/places_v1_mapper.go` (new) | Maps `placesv1.Place` → `POI.Place`. The only place that knows both vocabularies. |
| `iowrappers/places_v1_search_client.go` (new) | Implements `SearchClient.NearbySearch` against the New API; delegates Geocode/ReverseGeocode to the existing `MapsClient`. |
| `iowrappers/redis_keys.go` (new) | Versioned Redis key building shared by both paths. |
| `POI/categories.go` | Task 6 only: re-add the two types now that they work. |
| `iowrappers/photos_client.go` | Route by reference format: new `places/...` refs to the New media endpoint, legacy refs to the SDK. |
| `iowrappers/poi_searcher.go` | Select the search client from `PLACES_API_VERSION`. |
| `config/config.yml` | New-API field mask; retain the legacy `detailed_search_fields` for the legacy path. |

---

### Task 1: `placesv1` HTTP client and response types

**Files:**
- Create: `iowrappers/placesv1/client.go`, `iowrappers/placesv1/types.go`
- Test: `iowrappers/placesv1/client_test.go`

**Interfaces:**
- Produces:
  - `placesv1.New(apiKey string, opts ...Option) *Client`, `placesv1.WithBaseURL(string) Option`, `placesv1.WithHTTPClient(*http.Client) Option`
  - `(*Client).post(ctx context.Context, path, fieldMask string, body any, out any) error`
  - `placesv1.Place`, `placesv1.LocalizedText`, `placesv1.OpeningHours`, `placesv1.Photo`, `placesv1.LatLng`
  - `placesv1.APIError` with `Code int`, `Status string`, `Message string`
- Tasks 2, 3 and 4 all depend on these exact names.

- [ ] **Step 1: Write the failing test**

Create `iowrappers/placesv1/client_test.go`:

```go
package placesv1

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientSendsAPIKeyAndFieldMask(t *testing.T) {
	var gotKey, gotMask, gotContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-Goog-Api-Key")
		gotMask = r.Header.Get("X-Goog-FieldMask")
		gotContentType = r.Header.Get("Content-Type")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"places":[]}`))
	}))
	defer srv.Close()

	c := New("test-key", WithBaseURL(srv.URL))
	var out SearchNearbyResponse
	if err := c.post(context.Background(), "/v1/places:searchNearby", "places.id", map[string]any{}, &out); err != nil {
		t.Fatalf("post returned %v, want nil", err)
	}
	if gotKey != "test-key" {
		t.Errorf("X-Goog-Api-Key = %q, want %q", gotKey, "test-key")
	}
	if gotMask != "places.id" {
		t.Errorf("X-Goog-FieldMask = %q, want %q", gotMask, "places.id")
	}
	if gotContentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotContentType)
	}
}

func TestClientDecodesAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":400,"message":"Invalid included primary type: nonsense_type","status":"INVALID_ARGUMENT"}}`))
	}))
	defer srv.Close()

	c := New("test-key", WithBaseURL(srv.URL))
	var out SearchNearbyResponse
	err := c.post(context.Background(), "/v1/places:searchNearby", "places.id", map[string]any{}, &out)
	if err == nil {
		t.Fatal("post returned nil error, want APIError")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error %v is not *APIError", err)
	}
	if apiErr.Code != 400 || apiErr.Status != "INVALID_ARGUMENT" {
		t.Errorf("APIError = %+v, want code 400 status INVALID_ARGUMENT", apiErr)
	}
	// Unlike the legacy API, which silently ignored an unknown type, the New API rejects it.
	if apiErr.Message == "" {
		t.Error("APIError.Message is empty, want Google's explanation")
	}
}

func TestPlaceJSONDecodesNewAPIShape(t *testing.T) {
	// Trimmed real-shape response body.
	body := `{"places":[{
		"id":"ChIJ_test",
		"types":["cafe","food","point_of_interest","establishment"],
		"primaryType":"cafe",
		"formattedAddress":"367 State St, Los Altos, CA 94022, USA",
		"adrFormatAddress":"<span class=\"street-address\">367 State St</span>",
		"location":{"latitude":37.38025,"longitude":-122.11655},
		"rating":4.3,
		"userRatingCount":412,
		"googleMapsUri":"https://maps.google.com/?cid=1",
		"businessStatus":"OPERATIONAL",
		"priceLevel":"PRICE_LEVEL_INEXPENSIVE",
		"displayName":{"text":"Peet's Coffee","languageCode":"en"},
		"editorialSummary":{"text":"Coffee chain known for house blends.","languageCode":"en"},
		"regularOpeningHours":{"openNow":true,"weekdayDescriptions":[
			"Monday: 5:30 AM – 7:00 PM","Tuesday: 5:30 AM – 7:00 PM","Wednesday: 5:30 AM – 7:00 PM",
			"Thursday: 5:30 AM – 7:00 PM","Friday: 5:30 AM – 7:00 PM","Saturday: 6:00 AM – 7:00 PM",
			"Sunday: 6:00 AM – 7:00 PM"]},
		"photos":[{"name":"places/ChIJ_test/photos/AT_abc","widthPx":4032,"heightPx":3024}]
	}]}`

	var resp SearchNearbyResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if len(resp.Places) != 1 {
		t.Fatalf("got %d places, want 1", len(resp.Places))
	}
	p := resp.Places[0]
	if p.ID != "ChIJ_test" {
		t.Errorf("ID = %q, want ChIJ_test", p.ID)
	}
	if p.DisplayName.Text != "Peet's Coffee" {
		t.Errorf("DisplayName.Text = %q, want Peet's Coffee", p.DisplayName.Text)
	}
	if p.PrimaryType != "cafe" {
		t.Errorf("PrimaryType = %q, want cafe", p.PrimaryType)
	}
	if p.PriceLevel != PriceLevelInexpensive {
		t.Errorf("PriceLevel = %q, want %q", p.PriceLevel, PriceLevelInexpensive)
	}
	if len(p.RegularOpeningHours.WeekdayDescriptions) != 7 {
		t.Errorf("got %d weekday descriptions, want 7", len(p.RegularOpeningHours.WeekdayDescriptions))
	}
	if len(p.Photos) != 1 || p.Photos[0].Name != "places/ChIJ_test/photos/AT_abc" {
		t.Errorf("Photos = %+v, want one photo named places/ChIJ_test/photos/AT_abc", p.Photos)
	}
	if p.EditorialSummary.Text == "" {
		t.Error("EditorialSummary.Text is empty")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./iowrappers/placesv1/ -v`

Expected: FAIL — the package does not exist yet (`no Go files in .../placesv1`).

- [ ] **Step 3: Write `types.go`**

Create `iowrappers/placesv1/types.go`:

```go
// Package placesv1 is a minimal client for Google Places API (New),
// https://places.googleapis.com/v1. The googlemaps.github.io/maps v1.7.0 SDK only
// implements the legacy /maps/api/place/* endpoints, so this speaks REST directly.
//
// Every request must carry an X-Goog-FieldMask; the New API has no default field set.
package placesv1

// PriceLevel is the New API's price enum. Unlike the legacy integer priceLevel, an
// absent value is explicit (PriceLevelUnspecified) rather than indistinguishable from 0.
type PriceLevel string

const (
	PriceLevelUnspecified   PriceLevel = "PRICE_LEVEL_UNSPECIFIED"
	PriceLevelFree          PriceLevel = "PRICE_LEVEL_FREE"
	PriceLevelInexpensive   PriceLevel = "PRICE_LEVEL_INEXPENSIVE"
	PriceLevelModerate      PriceLevel = "PRICE_LEVEL_MODERATE"
	PriceLevelExpensive     PriceLevel = "PRICE_LEVEL_EXPENSIVE"
	PriceLevelVeryExpensive PriceLevel = "PRICE_LEVEL_VERY_EXPENSIVE"
)

// BusinessStatus values match POI.BusinessStatus strings exactly, so no translation
// table is needed: OPERATIONAL, CLOSED_TEMPORARILY, CLOSED_PERMANENTLY.
type BusinessStatus string

// RankPreference selects result ordering for searchNearby.
type RankPreference string

const (
	RankPreferenceDistance   RankPreference = "DISTANCE"
	RankPreferencePopularity RankPreference = "POPULARITY"
)

// LocalizedText backs displayName and editorialSummary.
type LocalizedText struct {
	Text         string `json:"text"`
	LanguageCode string `json:"languageCode"`
}

type LatLng struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type OpeningHours struct {
	OpenNow bool `json:"openNow"`
	// WeekdayDescriptions holds one human-readable string per day. Its starting weekday
	// is verified against the live API in Task 2 Step 0 before being mapped to
	// POI.Weekday — do not assume it matches the legacy WeekdayText ordering.
	WeekdayDescriptions []string `json:"weekdayDescriptions"`
}

// Photo.Name is a full resource name, "places/{placeID}/photos/{photoResource}".
// This is NOT interchangeable with a legacy photo_reference string.
type Photo struct {
	Name     string `json:"name"`
	WidthPx  int    `json:"widthPx"`
	HeightPx int    `json:"heightPx"`
}

type Place struct {
	ID                  string         `json:"id"`
	Types               []string       `json:"types"`
	PrimaryType         string         `json:"primaryType"`
	DisplayName         LocalizedText  `json:"displayName"`
	FormattedAddress    string         `json:"formattedAddress"`
	AdrFormatAddress    string         `json:"adrFormatAddress"`
	Location            LatLng         `json:"location"`
	Rating              float32        `json:"rating"`
	UserRatingCount     int            `json:"userRatingCount"`
	GoogleMapsURI       string         `json:"googleMapsUri"`
	BusinessStatus      BusinessStatus `json:"businessStatus"`
	PriceLevel          PriceLevel     `json:"priceLevel"`
	EditorialSummary    LocalizedText  `json:"editorialSummary"`
	RegularOpeningHours OpeningHours   `json:"regularOpeningHours"`
	Photos              []Photo        `json:"photos"`
}

type SearchNearbyResponse struct {
	Places []Place `json:"places"`
}

// Circle is the only locationRestriction shape searchNearby accepts.
type Circle struct {
	Center LatLng  `json:"center"`
	Radius float64 `json:"radius"` // meters, 0 < radius <= 50000
}

type locationRestriction struct {
	Circle Circle `json:"circle"`
}

type searchNearbyRequest struct {
	IncludedPrimaryTypes []string            `json:"includedPrimaryTypes,omitempty"`
	ExcludedPrimaryTypes []string            `json:"excludedPrimaryTypes,omitempty"`
	LocationRestriction  locationRestriction `json:"locationRestriction"`
	MaxResultCount       int                 `json:"maxResultCount,omitempty"`
	RankPreference       RankPreference      `json:"rankPreference,omitempty"`
	LanguageCode         string              `json:"languageCode,omitempty"`
}
```

- [ ] **Step 4: Write `client.go`**

Create `iowrappers/placesv1/client.go`:

```go
package placesv1

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	defaultBaseURL = "https://places.googleapis.com"
	// MaxRadiusMeters is the searchNearby locationRestriction circle cap.
	MaxRadiusMeters = 50000.0
	// MaxResultCount is the searchNearby hard cap. There is no pagination.
	MaxResultCount = 20
)

// APIError is a structured Places API (New) error. The New API rejects an unknown
// place type with INVALID_ARGUMENT, where the legacy API silently ignored the filter.
type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("places api (new): %d %s: %s", e.Code, e.Status, e.Message)
}

type errorEnvelope struct {
	Error APIError `json:"error"`
}

type Client struct {
	apiKey  string
	baseURL string
	http    *http.Client
}

type Option func(*Client)

func WithBaseURL(u string) Option           { return func(c *Client) { c.baseURL = u } }
func WithHTTPClient(h *http.Client) Option  { return func(c *Client) { c.http = h } }

func New(apiKey string, opts ...Option) *Client {
	c := &Client{
		apiKey:  apiKey,
		baseURL: defaultBaseURL,
		http:    &http.Client{Timeout: 15 * time.Second},
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// post sends a JSON POST with the API key and field mask headers the New API requires,
// and decodes either the success body into out or the error body into *APIError.
func (c *Client) post(ctx context.Context, path, fieldMask string, body any, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshaling request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Goog-Api-Key", c.apiKey)
	req.Header.Set("X-Goog-FieldMask", fieldMask)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("calling %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response from %s: %w", path, err)
	}
	if resp.StatusCode != http.StatusOK {
		var env errorEnvelope
		if jsonErr := json.Unmarshal(raw, &env); jsonErr == nil && env.Error.Code != 0 {
			return &env.Error
		}
		return &APIError{Code: resp.StatusCode, Status: resp.Status, Message: string(raw)}
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("decoding response from %s: %w", path, err)
	}
	return nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./iowrappers/placesv1/ -v`

Expected: PASS on all three tests.

- [ ] **Step 6: Commit**

```bash
git add iowrappers/placesv1/
git commit -m "feat: add minimal Places API (New) HTTP client

googlemaps.github.io/maps v1.7.0 only implements the legacy
/maps/api/place/* endpoints, so speak places.googleapis.com/v1 REST
directly. No new module dependencies."
```

---

### Task 2: `SearchNearby` request building and POI mapping

**Files:**
- Create: `iowrappers/placesv1/search_nearby.go`, `iowrappers/places_v1_mapper.go`
- Test: `iowrappers/placesv1/search_nearby_test.go`, `iowrappers/places_v1_mapper_test.go`

**Interfaces:**
- Consumes: everything from Task 1.
- Produces:
  - `(*Client).SearchNearby(ctx context.Context, req SearchNearbyRequest) ([]Place, error)` where `SearchNearbyRequest` is the exported struct defined below
  - `placesv1.SearchNearbyFieldMask` — the exact mask string
  - `iowrappers.MapPlace(p placesv1.Place) POI.Place`
  - `iowrappers.MapPriceLevel(pl placesv1.PriceLevel) POI.PriceLevel`

- [ ] **Step 0: Verify weekday ordering against the live API before writing the mapper**

`POI.CreatePlace` indexes hours by `POI.Weekday` from `DateMonday` to `DateSunday`. The legacy `WeekdayText` is Monday-first. Confirm the New API's `weekdayDescriptions` ordering rather than assuming it, because a silent off-by-one here shifts every place's opening hours by a day:

```bash
curl -s -X POST 'https://places.googleapis.com/v1/places:searchNearby' \
  -H "X-Goog-Api-Key: $GOOGLE_MAPS_API_KEY" \
  -H 'X-Goog-FieldMask: places.displayName,places.regularOpeningHours.weekdayDescriptions' \
  -H 'Content-Type: application/json' \
  -d '{"includedPrimaryTypes":["cafe"],"maxResultCount":1,
       "locationRestriction":{"circle":{"center":{"latitude":37.38006,"longitude":-122.11612},"radius":2000}}}' | jq
```

Record the first element's day name in a code comment in `places_v1_mapper.go`. If it is not Monday, the mapper must rotate the slice before handing it to `POI.OpeningHours.Hours`.

- [ ] **Step 1: Write the failing test**

Create `iowrappers/placesv1/search_nearby_test.go`:

```go
package placesv1

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearchNearbyBuildsRequest(t *testing.T) {
	var got searchNearbyRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &got)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"places":[]}`))
	}))
	defer srv.Close()

	c := New("k", WithBaseURL(srv.URL))
	_, err := c.SearchNearby(context.Background(), SearchNearbyRequest{
		IncludedPrimaryTypes: []string{"cafe", "restaurant", "bar", "bakery", "meal_takeaway"},
		Latitude:             37.38006,
		Longitude:            -122.11612,
		RadiusMeters:         8000,
		MaxResultCount:       20,
		RankPreference:       RankPreferenceDistance,
	})
	if err != nil {
		t.Fatalf("SearchNearby returned %v", err)
	}
	if len(got.IncludedPrimaryTypes) != 5 {
		t.Errorf("IncludedPrimaryTypes = %v, want 5 entries", got.IncludedPrimaryTypes)
	}
	if got.LocationRestriction.Circle.Radius != 8000 {
		t.Errorf("radius = %v, want 8000", got.LocationRestriction.Circle.Radius)
	}
	if got.LocationRestriction.Circle.Center.Latitude != 37.38006 {
		t.Errorf("center.latitude = %v, want 37.38006", got.LocationRestriction.Circle.Center.Latitude)
	}
	if got.RankPreference != RankPreferenceDistance {
		t.Errorf("rankPreference = %q, want DISTANCE", got.RankPreference)
	}
}

func TestSearchNearbyClampsRadiusAndCount(t *testing.T) {
	var got searchNearbyRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &got)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"places":[]}`))
	}))
	defer srv.Close()

	c := New("k", WithBaseURL(srv.URL))
	_, err := c.SearchNearby(context.Background(), SearchNearbyRequest{
		IncludedPrimaryTypes: []string{"cafe"},
		Latitude:             37.38006,
		Longitude:            -122.11612,
		RadiusMeters:         120000, // over the 50km cap
		MaxResultCount:       500,    // over the 20 cap
	})
	if err != nil {
		t.Fatalf("SearchNearby returned %v", err)
	}
	if got.LocationRestriction.Circle.Radius != MaxRadiusMeters {
		t.Errorf("radius = %v, want clamped to %v", got.LocationRestriction.Circle.Radius, MaxRadiusMeters)
	}
	if got.MaxResultCount != MaxResultCount {
		t.Errorf("maxResultCount = %d, want clamped to %d", got.MaxResultCount, MaxResultCount)
	}
}

func TestSearchNearbyRejectsEmptyTypes(t *testing.T) {
	c := New("k", WithBaseURL("http://unused"))
	if _, err := c.SearchNearby(context.Background(), SearchNearbyRequest{
		Latitude: 1, Longitude: 1, RadiusMeters: 100,
	}); err == nil {
		t.Error("SearchNearby with no types returned nil error, want validation failure")
	}
}
```

Create `iowrappers/places_v1_mapper_test.go`:

```go
package iowrappers

import (
	"testing"

	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/iowrappers/placesv1"
)

func TestMapPriceLevel(t *testing.T) {
	cases := map[placesv1.PriceLevel]POI.PriceLevel{
		// Unspecified maps to 0, matching the legacy behavior where an absent
		// priceLevel arrived as integer 0 and was bucketed into level0.
		placesv1.PriceLevelUnspecified:   POI.PriceLevelZero,
		placesv1.PriceLevelFree:          POI.PriceLevelZero,
		placesv1.PriceLevelInexpensive:   POI.PriceLevelOne,
		placesv1.PriceLevelModerate:      POI.PriceLevelTwo,
		placesv1.PriceLevelExpensive:     POI.PriceLevelThree,
		placesv1.PriceLevelVeryExpensive: POI.PriceLevelFour,
	}
	for in, want := range cases {
		if got := MapPriceLevel(in); got != want {
			t.Errorf("MapPriceLevel(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestMapPlace(t *testing.T) {
	in := placesv1.Place{
		ID:               "ChIJ_test",
		Types:            []string{"cafe", "food", "point_of_interest", "establishment"},
		PrimaryType:      "cafe",
		DisplayName:      placesv1.LocalizedText{Text: "Peet's Coffee"},
		FormattedAddress: "367 State St, Los Altos, CA 94022, USA",
		AdrFormatAddress: `<span class="street-address">367 State St</span>`,
		Location:         placesv1.LatLng{Latitude: 37.38025, Longitude: -122.11655},
		Rating:           4.3,
		UserRatingCount:  412,
		GoogleMapsURI:    "https://maps.google.com/?cid=1",
		BusinessStatus:   placesv1.BusinessStatus("OPERATIONAL"),
		PriceLevel:       placesv1.PriceLevelInexpensive,
		EditorialSummary: placesv1.LocalizedText{Text: "Coffee chain known for house blends."},
		RegularOpeningHours: placesv1.OpeningHours{WeekdayDescriptions: []string{
			"Monday: 5:30 AM – 7:00 PM", "Tuesday: 5:30 AM – 7:00 PM", "Wednesday: 5:30 AM – 7:00 PM",
			"Thursday: 5:30 AM – 7:00 PM", "Friday: 5:30 AM – 7:00 PM", "Saturday: 6:00 AM – 7:00 PM",
			"Sunday: 6:00 AM – 7:00 PM"}},
		Photos: []placesv1.Photo{{Name: "places/ChIJ_test/photos/AT_abc", WidthPx: 4032, HeightPx: 3024}},
	}

	got := MapPlace(in)

	if got.GetID() != "ChIJ_test" {
		t.Errorf("ID = %q, want ChIJ_test", got.GetID())
	}
	if got.GetName() != "Peet's Coffee" {
		t.Errorf("Name = %q, want Peet's Coffee", got.GetName())
	}
	// LocationType comes from primaryType, so the record is correctly typed at write
	// time. The legacy path stamped the SEARCHED type here, which is how hotels ended
	// up labeled fast_food_restaurant.
	if got.LocationType != POI.LocationTypeCafe {
		t.Errorf("LocationType = %q, want cafe", got.LocationType)
	}
	if len(got.Types) != 4 || got.Types[0] != "cafe" {
		t.Errorf("Types = %v, want Google's full list primary-first", got.Types)
	}
	if got.Status != POI.Operational {
		t.Errorf("Status = %q, want OPERATIONAL", got.Status)
	}
	if got.PriceLevel != POI.PriceLevelOne {
		t.Errorf("PriceLevel = %d, want 1", got.PriceLevel)
	}
	if got.UserRatingsTotal != 412 {
		t.Errorf("UserRatingsTotal = %d, want 412", got.UserRatingsTotal)
	}
	if got.URL != "https://maps.google.com/?cid=1" {
		t.Errorf("URL = %q, want the googleMapsUri", got.URL)
	}
	if got.Summary != "Coffee chain known for house blends." {
		t.Errorf("Summary = %q, want the editorial summary text", got.Summary)
	}
	// The photo reference is a full resource name now, not a legacy photo_reference.
	if got.Photo.Reference != "places/ChIJ_test/photos/AT_abc" {
		t.Errorf("Photo.Reference = %q, want the full resource name", got.Photo.Reference)
	}
	if got.GetHour(POI.DateMonday) != "Monday: 5:30 AM – 7:00 PM" {
		t.Errorf("Monday hours = %q, want the Monday description", got.GetHour(POI.DateMonday))
	}
}

// TestMapPlaceEmptyOptionalFields pins that a sparse response does not panic and
// leaves POI defaults intact.
func TestMapPlaceEmptyOptionalFields(t *testing.T) {
	got := MapPlace(placesv1.Place{
		ID:          "ChIJ_sparse",
		PrimaryType: "restaurant",
		DisplayName: placesv1.LocalizedText{Text: "Sparse Diner"},
		Location:    placesv1.LatLng{Latitude: 1, Longitude: 2},
	})
	if got.GetID() != "ChIJ_sparse" {
		t.Errorf("ID = %q, want ChIJ_sparse", got.GetID())
	}
	if got.Photo.Reference != "" {
		t.Errorf("Photo.Reference = %q, want empty", got.Photo.Reference)
	}
	if got.PriceLevel != POI.PriceLevelZero {
		t.Errorf("PriceLevel = %d, want 0", got.PriceLevel)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./iowrappers/placesv1/ -run TestSearchNearby -v && go test ./iowrappers/ -run 'TestMapPlace|TestMapPriceLevel' -v`

Expected: FAIL with `c.SearchNearby undefined` and `undefined: MapPlace`.

- [ ] **Step 3: Write `search_nearby.go`**

Create `iowrappers/placesv1/search_nearby.go`:

```go
package placesv1

import (
	"context"
	"errors"
	"fmt"
)

// SearchNearbyFieldMask is the exact field set the planner needs. Keep it minimal:
// the New API bills by SKU tier based on which fields are requested, and there is no
// default mask. Every field here replaces something the legacy path needed a separate
// Place Details call to get.
const SearchNearbyFieldMask = "places.id," +
	"places.types," +
	"places.primaryType," +
	"places.displayName," +
	"places.formattedAddress," +
	"places.adrFormatAddress," +
	"places.location," +
	"places.rating," +
	"places.userRatingCount," +
	"places.googleMapsUri," +
	"places.businessStatus," +
	"places.priceLevel," +
	"places.editorialSummary," +
	"places.regularOpeningHours.weekdayDescriptions," +
	"places.photos"

// SearchNearbyRequest is the caller-facing shape. RadiusMeters and MaxResultCount are
// clamped to the API's limits rather than rejected, so callers can pass through the
// planner's own wider radius constants unchanged.
type SearchNearbyRequest struct {
	IncludedPrimaryTypes []string
	ExcludedPrimaryTypes []string
	Latitude             float64
	Longitude            float64
	RadiusMeters         float64
	MaxResultCount       int
	RankPreference       RankPreference
	LanguageCode         string
}

// SearchNearby calls places:searchNearby.
//
// Unlike the legacy endpoint this filters by PRIMARY type server-side, so results do
// not need client-side reclassification, and an unknown type is rejected with
// INVALID_ARGUMENT instead of silently disabling the filter.
//
// There is no pagination: at most MaxResultCount (cap 20) places come back.
func (c *Client) SearchNearby(ctx context.Context, req SearchNearbyRequest) ([]Place, error) {
	if len(req.IncludedPrimaryTypes) == 0 {
		return nil, errors.New("placesv1: SearchNearby requires at least one included primary type")
	}
	radius := req.RadiusMeters
	if radius > MaxRadiusMeters {
		radius = MaxRadiusMeters
	}
	if radius <= 0 {
		return nil, fmt.Errorf("placesv1: radius must be > 0, got %v", req.RadiusMeters)
	}
	count := req.MaxResultCount
	if count > MaxResultCount || count <= 0 {
		count = MaxResultCount
	}
	rank := req.RankPreference
	if rank == "" {
		rank = RankPreferenceDistance
	}

	body := searchNearbyRequest{
		IncludedPrimaryTypes: req.IncludedPrimaryTypes,
		ExcludedPrimaryTypes: req.ExcludedPrimaryTypes,
		LocationRestriction: locationRestriction{
			Circle: Circle{
				Center: LatLng{Latitude: req.Latitude, Longitude: req.Longitude},
				Radius: radius,
			},
		},
		MaxResultCount: count,
		RankPreference: rank,
		LanguageCode:   req.LanguageCode,
	}

	var resp SearchNearbyResponse
	if err := c.post(ctx, "/v1/places:searchNearby", SearchNearbyFieldMask, body, &resp); err != nil {
		return nil, err
	}
	return resp.Places, nil
}
```

- [ ] **Step 4: Write `places_v1_mapper.go`**

Create `iowrappers/places_v1_mapper.go`. Adjust the weekday rotation only if Step 0 showed a non-Monday first element:

```go
package iowrappers

import (
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/iowrappers/placesv1"
)

// MapPriceLevel converts the New API's price enum to POI.PriceLevel.
//
// PRICE_LEVEL_UNSPECIFIED maps to 0 deliberately: the legacy path received an absent
// price as integer 0 and bucketed it into placeIDs:eatery:level0, so this preserves
// which bucket an unpriced place lands in.
func MapPriceLevel(pl placesv1.PriceLevel) POI.PriceLevel {
	switch pl {
	case placesv1.PriceLevelInexpensive:
		return POI.PriceLevelOne
	case placesv1.PriceLevelModerate:
		return POI.PriceLevelTwo
	case placesv1.PriceLevelExpensive:
		return POI.PriceLevelThree
	case placesv1.PriceLevelVeryExpensive:
		return POI.PriceLevelFour
	case placesv1.PriceLevelFree, placesv1.PriceLevelUnspecified:
		return POI.PriceLevelZero
	default:
		return POI.PriceLevelZero
	}
}

// MapPlace converts a Places API (New) place into the internal POI.Place.
//
// LocationType is set from primaryType — Google's own answer for what the place mainly
// is. The legacy path stamped the SEARCHED type here instead, which is how a hotel
// returned by an unenforceable fast_food_restaurant filter became a labeled eatery.
//
// weekdayDescriptions is Monday-first (verified against the live API on 2026-07-29),
// matching POI.Weekday's DateMonday..DateSunday order.
func MapPlace(p placesv1.Place) POI.Place {
	var hours *POI.OpeningHours
	if len(p.RegularOpeningHours.WeekdayDescriptions) > 0 {
		hours = &POI.OpeningHours{Hours: append([]string(nil), p.RegularOpeningHours.WeekdayDescriptions...)}
	}

	var summary *string
	if p.EditorialSummary.Text != "" {
		text := p.EditorialSummary.Text
		summary = &text
	}

	place := POI.CreatePlace(
		p.DisplayName.Text,
		p.AdrFormatAddress,
		p.FormattedAddress,
		string(p.BusinessStatus),
		POI.LocationType(p.PrimaryType),
		hours,
		p.ID,
		int(MapPriceLevel(p.PriceLevel)),
		p.Rating,
		p.GoogleMapsURI,
		nil, // legacy *maps.Photo is not used on this path; set below
		p.UserRatingCount,
		p.Location.Latitude,
		p.Location.Longitude,
		summary,
	)

	// Photo.Reference holds the New API resource name ("places/{id}/photos/{ref}"),
	// which is NOT a legacy photo_reference. photos_client.go routes on this prefix.
	if len(p.Photos) > 0 {
		place.Photo = POI.PlacePhoto{
			Reference: p.Photos[0].Name,
			Width:     p.Photos[0].WidthPx,
			Height:    p.Photos[0].HeightPx,
		}
	}

	// Preserve Google's full feature-type list so ReclassifyForCategory and
	// PrimaryLocationType keep working on records written by this path.
	place.Types = append([]string(nil), p.Types...)
	return place
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go build -v . && go test ./iowrappers/placesv1/ -v && go test ./iowrappers/ -run 'TestMapPlace|TestMapPriceLevel' -v`

Expected: PASS on all tests.

- [ ] **Step 6: Commit**

```bash
git add iowrappers/placesv1/search_nearby.go iowrappers/placesv1/search_nearby_test.go iowrappers/places_v1_mapper.go iowrappers/places_v1_mapper_test.go
git commit -m "feat: add searchNearby call and Places API (New) to POI mapping

LocationType now comes from Google's primaryType rather than the searched
type, so records are correctly labeled at write time. The field mask pulls
opening hours, adr address, maps URI, rating count, editorial summary and
photos in the search response, removing the need for a separate Place
Details call per result."
```

---

### Task 3: Versioned Redis keys and a New-API `SearchClient` behind a flag

**Files:**
- Create: `iowrappers/redis_keys.go`, `iowrappers/places_v1_search_client.go`
- Modify: `iowrappers/redis_client.go` (`nearbySearchRedisKeys`, `SetPlacesAddGeoLocations`, `getPlace`/`setPlace` key building), `iowrappers/poi_searcher.go` (client selection)
- Test: `iowrappers/redis_keys_test.go`, `test/redis_client_mocks/v2_keys_test.go`

**Interfaces:**
- Consumes: `placesv1.Client.SearchNearby`, `iowrappers.MapPlace` (Task 2); `POI.GetPlaceCategory(...) (PlaceCategory, bool)` (prior PR).
- Produces:
  - `iowrappers.KeyVersion` type with `KeyVersionLegacy KeyVersion = ""` and `KeyVersionV2 KeyVersion = "v2"`
  - `iowrappers.NearbySearchKey(cat POI.PlaceCategory, level POI.PriceLevel, v KeyVersion) string`
  - `iowrappers.PlaceDetailsKey(placeID string, v KeyVersion) string`
  - `iowrappers.NewPlacesV1SearchClient(apiKey string, mapsClient *MapsClient) *PlacesV1SearchClient` implementing `SearchClient`
  - `iowrappers.ActiveKeyVersion() KeyVersion` — reads `PLACES_API_VERSION`

- [ ] **Step 1: Write the failing test**

Create `iowrappers/redis_keys_test.go`:

```go
package iowrappers

import (
	"strings"
	"testing"

	"github.com/weihesdlegend/Vacation-planner/POI"
)

// PlaceIDsKeyPrefix and PlaceDetailsKeyPrefix come from redis_data_inspections.go,
// PlaceDetailsRedisKeyPrefix from redis_client.go — all the same package, no import needed.

// TestNearbySearchKeyLegacyUnchanged pins that the legacy key format is byte-identical
// to what production already holds. Any drift orphans the existing cache.
func TestNearbySearchKeyLegacyUnchanged(t *testing.T) {
	cases := map[string]string{
		NearbySearchKey(POI.PlaceCategoryEatery, POI.PriceLevelZero, KeyVersionLegacy):  "placeIDs:eatery:level0",
		NearbySearchKey(POI.PlaceCategoryEatery, POI.PriceLevelThree, KeyVersionLegacy): "placeIDs:eatery:level3",
		NearbySearchKey(POI.PlaceCategoryVisit, POI.PriceLevelTwo, KeyVersionLegacy):    "placeIDs:visit",
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
}

// TestNearbySearchKeyV2Namespaced pins that v2 data never collides with legacy data,
// so cutover and rollback are both non-destructive.
//
// The version is the FIRST segment on purpose. Existing code scans by legacy prefix —
// redis_data_inspections.go:22 scans "place_details*" and PlaceIDsKeyPrefix is
// "placeIDs" — so a suffixed name like "place_details_v2:" or an infixed one like
// "placeIDs:v2:" would be swept up by those scans and double-count or corrupt stats
// and migrations. Leading with "v2:" keeps v2 keys invisible to every legacy scan.
func TestNearbySearchKeyV2Namespaced(t *testing.T) {
	cases := map[string]string{
		NearbySearchKey(POI.PlaceCategoryEatery, POI.PriceLevelZero, KeyVersionV2): "v2:placeIDs:eatery:level0",
		NearbySearchKey(POI.PlaceCategoryVisit, POI.PriceLevelTwo, KeyVersionV2):   "v2:placeIDs:visit",
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
}

// TestV2KeysInvisibleToLegacyScans is the regression guard for the collision above.
func TestV2KeysInvisibleToLegacyScans(t *testing.T) {
	v2Keys := []string{
		NearbySearchKey(POI.PlaceCategoryEatery, POI.PriceLevelZero, KeyVersionV2),
		PlaceDetailsKey("ChIJ_x", KeyVersionV2),
	}
	legacyScanPrefixes := []string{PlaceDetailsKeyPrefix, PlaceIDsKeyPrefix, PlaceDetailsRedisKeyPrefix}
	for _, key := range v2Keys {
		for _, prefix := range legacyScanPrefixes {
			if strings.HasPrefix(key, prefix) {
				t.Errorf("v2 key %q is matched by legacy scan pattern %q*", key, prefix)
			}
		}
	}
}

func TestPlaceDetailsKey(t *testing.T) {
	if got, want := PlaceDetailsKey("ChIJ_x", KeyVersionLegacy), PlaceDetailsRedisKeyPrefix+"ChIJ_x"; got != want {
		t.Errorf("legacy details key = %q, want %q", got, want)
	}
	if got, want := PlaceDetailsKey("ChIJ_x", KeyVersionV2), "v2:place_details:place_ID:ChIJ_x"; got != want {
		t.Errorf("v2 details key = %q, want %q", got, want)
	}
}

func TestActiveKeyVersionDefaultsToLegacy(t *testing.T) {
	t.Setenv("PLACES_API_VERSION", "")
	if got := ActiveKeyVersion(); got != KeyVersionLegacy {
		t.Errorf("ActiveKeyVersion() = %q with no env set, want legacy", got)
	}
	t.Setenv("PLACES_API_VERSION", "new")
	if got := ActiveKeyVersion(); got != KeyVersionV2 {
		t.Errorf("ActiveKeyVersion() = %q with PLACES_API_VERSION=new, want v2", got)
	}
	t.Setenv("PLACES_API_VERSION", "legacy")
	if got := ActiveKeyVersion(); got != KeyVersionLegacy {
		t.Errorf("ActiveKeyVersion() = %q with PLACES_API_VERSION=legacy, want legacy", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./iowrappers/ -run 'TestNearbySearchKey|TestPlaceDetailsKey|TestActiveKeyVersion' -v`

Expected: FAIL with `undefined: NearbySearchKey`.

- [ ] **Step 3: Write `redis_keys.go`**

```go
package iowrappers

import (
	"fmt"
	"os"
	"strings"

	"github.com/weihesdlegend/Vacation-planner/POI"
)

// KeyVersion namespaces cached place data by the API that produced it.
//
// Places API (New) records are NOT interchangeable with legacy ones: photo references
// change from an opaque photo_reference to a "places/{id}/photos/{ref}" resource name,
// and LocationType comes from primaryType rather than the searched type. Writing both
// under one key would mix formats with no way to tell them apart, so the new path gets
// its own namespace. Cutover and rollback are then a single env-var flip with no deletes.
type KeyVersion string

const (
	KeyVersionLegacy KeyVersion = ""
	KeyVersionV2     KeyVersion = "v2"
)

// PlaceDetailsV2RedisKeyPrefix mirrors PlaceDetailsRedisKeyPrefix for v2 records.
//
// The version leads the key. Existing code scans by legacy prefix —
// redis_data_inspections.go:22 scans PlaceDetailsKeyPrefix+"*" ("place_details*") and
// PlaceIDsKeyPrefix is "placeIDs" — so a suffixed "place_details_v2:" would be caught
// by those scans and make GetPlaceCountInRedis and the RemovePlaces migration operate
// on v2 records they know nothing about. "v2:" first keeps them cleanly separated.
const PlaceDetailsV2RedisKeyPrefix = "v2:place_details:place_ID:"

// ActiveKeyVersion reads PLACES_API_VERSION. Anything other than "new" means legacy,
// so an unset or misspelled value fails safe onto the working path.
func ActiveKeyVersion() KeyVersion {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("PLACES_API_VERSION")), "new") {
		return KeyVersionV2
	}
	return KeyVersionLegacy
}

// NearbySearchKey builds a geo bucket key. The legacy form is byte-identical to
// POI.EncodeNearbySearchRedisKey so existing production data stays addressable, and the
// v2 form leads with the version so legacy "placeIDs*"/"place_details*" scans skip it.
func NearbySearchKey(cat POI.PlaceCategory, level POI.PriceLevel, v KeyVersion) string {
	segments := make([]string, 0, 4)
	if v != KeyVersionLegacy {
		segments = append(segments, string(v))
	}
	segments = append(segments, PlaceIDsKeyPrefix, strings.ToLower(string(cat)))
	if cat == POI.PlaceCategoryEatery {
		segments = append(segments, fmt.Sprintf("level%d", level))
	}
	return strings.Join(segments, ":")
}

// PlaceDetailsKey builds the per-place record key for a version.
func PlaceDetailsKey(placeID string, v KeyVersion) string {
	if v == KeyVersionV2 {
		return PlaceDetailsV2RedisKeyPrefix + placeID
	}
	return PlaceDetailsRedisKeyPrefix + placeID
}
```

- [ ] **Step 4: Route the Redis read/write paths through the version**

In `iowrappers/redis_client.go`, add a `keyVersion KeyVersion` field to `RedisClient`, defaulted from `ActiveKeyVersion()` wherever the client is constructed. Then:

- `nearbySearchRedisKeys` (`:452`): replace each `POI.EncodeNearbySearchRedisKey(cat, lvl)` with `NearbySearchKey(cat, lvl, r.keyVersion)`. This requires making it a method on `*RedisClient`; update its two callers and `iowrappers/nearby_search_keys_test.go` accordingly.
- `SetPlacesAddGeoLocations` (`:222`): use `NearbySearchKey(placeCategory, place.PriceLevel, r.keyVersion)` and `PlaceDetailsKey(place.ID, r.keyVersion)`.
- `getPlace` / `setPlace`: take the version from `r.keyVersion`.

**Read-compatibility requirement.** Saved trip plans store bare Google place IDs and read them back through `place_details:place_ID:` (`planner/planner.go:797`). Google place IDs are identical across both APIs, but a plan saved before cutover has records only under the legacy key. So the single-record read must fall back:

```go
// getPlaceAnyVersion reads a place record, preferring the active key version and falling
// back to the other. Saved trip plans reference bare place IDs, and a plan saved before
// the Places API (New) cutover has a record only under the legacy key — so a
// version-strict read would break every existing saved plan.
func (r *RedisClient) getPlaceAnyVersion(ctx context.Context, placeID string) (POI.Place, error) {
	place, err := r.getPlaceAtKey(ctx, PlaceDetailsKey(placeID, r.keyVersion))
	if err == nil {
		return place, nil
	}
	other := KeyVersionLegacy
	if r.keyVersion == KeyVersionLegacy {
		other = KeyVersionV2
	}
	return r.getPlaceAtKey(ctx, PlaceDetailsKey(placeID, other))
}
```

Use `getPlaceAnyVersion` for saved-plan reads (`planner/planner.go:797-806`) and the version-strict `getPlace` for geo-bucket reads, where members always come from the matching namespace.

- [ ] **Step 5: Write the New-API `SearchClient`**

Create `iowrappers/places_v1_search_client.go`:

```go
package iowrappers

import (
	"context"
	"fmt"

	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/iowrappers/placesv1"
)

// PlacesV1SearchClient serves category searches from Places API (New).
//
// Geocode and ReverseGeocode delegate to the legacy MapsClient on purpose: the
// Geocoding API (/maps/api/geocode/json) is a separate, non-deprecated API and is not
// part of this migration.
//
// Brand/keyword searches also stay on the legacy client: searchNearby has no keyword
// parameter, and moving them needs places:searchText. Until that lands, a request with
// a Keyword is delegated wholesale.
type PlacesV1SearchClient struct {
	places     *placesv1.Client
	mapsClient *MapsClient
	// TypeGroups controls fan-out. One group containing every type = one HTTP call,
	// capped at 20 results. Splitting into one group per type restores the legacy
	// per-type ceiling at the cost of one call each. Set from Task 5's measurements.
	TypeGroups func(POI.PlaceCategory) [][]POI.LocationType
}

func NewPlacesV1SearchClient(apiKey string, mapsClient *MapsClient) *PlacesV1SearchClient {
	return &PlacesV1SearchClient{
		places:     placesv1.New(apiKey),
		mapsClient: mapsClient,
		TypeGroups: SingleGroupTypes,
	}
}

// SingleGroupTypes puts every type of a category into one searchNearby call.
func SingleGroupTypes(cat POI.PlaceCategory) [][]POI.LocationType {
	return [][]POI.LocationType{POI.GetPlaceTypes(cat)}
}

// PerTypeGroups issues one searchNearby call per place type, restoring the legacy
// per-type result ceiling. Costs len(GetPlaceTypes(cat)) calls instead of one.
func PerTypeGroups(cat POI.PlaceCategory) [][]POI.LocationType {
	types := POI.GetPlaceTypes(cat)
	groups := make([][]POI.LocationType, 0, len(types))
	for _, t := range types {
		groups = append(groups, []POI.LocationType{t})
	}
	return groups
}

func (c *PlacesV1SearchClient) Geocode(ctx context.Context, q *GeocodeQuery) (float64, float64, error) {
	return c.mapsClient.Geocode(ctx, q)
}

func (c *PlacesV1SearchClient) ReverseGeocode(ctx context.Context, lat, lng float64) (*GeocodeQuery, error) {
	return c.mapsClient.ReverseGeocode(ctx, lat, lng)
}

func (c *PlacesV1SearchClient) NearbySearch(ctx context.Context, req *PlaceSearchRequest) ([]POI.Place, error) {
	if req.Keyword != "" {
		// searchNearby has no keyword parameter; brand search still needs searchText.
		return c.mapsClient.NearbySearch(ctx, req)
	}

	groups := c.TypeGroups(req.PlaceCat)
	if len(groups) == 0 {
		return nil, fmt.Errorf("no place types for category %q", req.PlaceCat)
	}

	seen := make(map[string]bool)
	places := make([]POI.Place, 0, len(groups)*placesv1.MaxResultCount)
	for _, group := range groups {
		types := make([]string, 0, len(group))
		for _, t := range group {
			if t != POI.LocationTypeAny {
				types = append(types, string(t))
			}
		}
		if len(types) == 0 {
			continue
		}
		found, err := c.places.SearchNearby(ctx, placesv1.SearchNearbyRequest{
			IncludedPrimaryTypes: types,
			Latitude:             req.Location.Latitude,
			Longitude:            req.Location.Longitude,
			RadiusMeters:         float64(req.Radius),
			MaxResultCount:       placesv1.MaxResultCount,
			RankPreference:       placesv1.RankPreferenceDistance,
		})
		if err != nil {
			// Unlike the legacy API, an unknown type is a hard INVALID_ARGUMENT here.
			// Log and continue so one bad type cannot zero out a whole category.
			Logger.Error(fmt.Errorf("searchNearby failed for types %v: %w", types, err))
			continue
		}
		for _, p := range found {
			if seen[p.ID] {
				continue
			}
			seen[p.ID] = true
			place := MapPlace(p)
			// Match the legacy path's filter: places with no ratings are not useful.
			if place.UserRatingsTotal == 0 {
				continue
			}
			places = append(places, place)
		}
	}
	return places, nil
}
```

- [ ] **Step 6: Select the client from the flag**

In `iowrappers/poi_searcher.go`, wherever `PoiSearcher` is constructed with its `SearchClient`, choose based on the flag:

```go
	if ActiveKeyVersion() == KeyVersionV2 {
		Logger.Info("PLACES_API_VERSION=new: serving category searches from Places API (New)")
		searcher.searchClient = NewPlacesV1SearchClient(apiKey, mapsClient)
	} else {
		searcher.searchClient = mapsClient
	}
```

- [ ] **Step 7: Run the full suite**

Run: `go build -v . && go test ./... 2>&1 | tail -25`

Expected: PASS. With `PLACES_API_VERSION` unset, every existing test exercises the unchanged legacy path.

- [ ] **Step 8: Commit**

```bash
git add iowrappers/redis_keys.go iowrappers/redis_keys_test.go iowrappers/places_v1_search_client.go iowrappers/redis_client.go iowrappers/poi_searcher.go planner/planner.go
git commit -m "feat: add PLACES_API_VERSION flag and v2-namespaced cache keys

New API records are not interchangeable with legacy ones (photo resource
names, primaryType-derived LocationType), so they get their own key
namespace. Cutover and rollback are one env-var flip with no deletes.
Saved-plan reads fall back across versions since place IDs are shared."
```

---

### Task 4: Photos via the New media endpoint

**Files:**
- Create: `iowrappers/placesv1/photo.go`
- Modify: `iowrappers/photos_client.go`
- Test: `iowrappers/placesv1/photo_test.go`

**Interfaces:**
- Consumes: `placesv1.Client` (Task 1).
- Produces: `(*placesv1.Client).PhotoMediaURL(photoName string, maxWidthPx int) (string, error)` and `(*placesv1.Client).FetchPhoto(ctx context.Context, photoName string, maxWidthPx int) ([]byte, string, error)` returning bytes and content type.

- [ ] **Step 1: Write the failing test**

Create `iowrappers/placesv1/photo_test.go`:

```go
package placesv1

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPhotoMediaURL(t *testing.T) {
	c := New("test-key")
	got, err := c.PhotoMediaURL("places/ChIJ_x/photos/AT_abc", 400)
	if err != nil {
		t.Fatalf("PhotoMediaURL error: %v", err)
	}
	for _, want := range []string{
		"https://places.googleapis.com/v1/places/ChIJ_x/photos/AT_abc/media",
		"maxWidthPx=400",
		"key=test-key",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("URL %q missing %q", got, want)
		}
	}
}

// TestPhotoMediaURLRejectsLegacyReference pins that an opaque legacy photo_reference
// cannot be passed to the New media endpoint. Cached legacy references are not
// convertible, which is why photos_client.go routes on the "places/" prefix.
func TestPhotoMediaURLRejectsLegacyReference(t *testing.T) {
	c := New("test-key")
	if _, err := c.PhotoMediaURL("ATtYBwLQ_legacy_opaque_ref", 400); err == nil {
		t.Error("PhotoMediaURL accepted a legacy photo_reference, want error")
	}
}

func TestFetchPhotoFollowsRedirectAndReturnsBytes(t *testing.T) {
	image := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte{0xFF, 0xD8, 0xFF, 0xE0})
	}))
	defer image.Close()

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, image.URL, http.StatusFound)
	}))
	defer api.Close()

	c := New("test-key", WithBaseURL(api.URL))
	data, contentType, err := c.FetchPhoto(context.Background(), "places/ChIJ_x/photos/AT_abc", 400)
	if err != nil {
		t.Fatalf("FetchPhoto error: %v", err)
	}
	if contentType != "image/jpeg" {
		t.Errorf("contentType = %q, want image/jpeg", contentType)
	}
	if len(data) != 4 {
		t.Errorf("got %d bytes, want 4", len(data))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./iowrappers/placesv1/ -run 'TestPhoto|TestFetchPhoto' -v`

Expected: FAIL with `c.PhotoMediaURL undefined`.

- [ ] **Step 3: Write `photo.go`**

```go
package placesv1

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// PhotoNamePrefix marks a New API photo resource name. Legacy photo_reference strings
// are opaque and have no prefix, which makes this a reliable discriminator for cached
// records written by either API.
const PhotoNamePrefix = "places/"

// PhotoMediaURL builds the media URL for a photo resource name obtained from a search
// or details response, e.g. "places/{placeID}/photos/{photoResource}".
//
// A legacy photo_reference is NOT convertible to this form — the only way to get a
// usable reference for a place cached under the legacy API is to re-fetch the place.
func (c *Client) PhotoMediaURL(photoName string, maxWidthPx int) (string, error) {
	if !strings.HasPrefix(photoName, PhotoNamePrefix) {
		return "", fmt.Errorf("placesv1: %q is not a photo resource name (want %s...); legacy photo_reference values are not convertible", photoName, PhotoNamePrefix)
	}
	if maxWidthPx < 1 || maxWidthPx > 4800 {
		return "", fmt.Errorf("placesv1: maxWidthPx must be 1..4800, got %d", maxWidthPx)
	}
	q := url.Values{}
	q.Set("maxWidthPx", fmt.Sprint(maxWidthPx))
	q.Set("key", c.apiKey)
	return fmt.Sprintf("%s/v1/%s/media?%s", c.baseURL, photoName, q.Encode()), nil
}

// FetchPhoto downloads the image bytes. The endpoint answers with an HTTP redirect to
// the image by default, which http.Client follows.
func (c *Client) FetchPhoto(ctx context.Context, photoName string, maxWidthPx int) ([]byte, string, error) {
	mediaURL, err := c.PhotoMediaURL(photoName, maxWidthPx)
	if err != nil {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, mediaURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("building photo request: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("fetching photo %s: %w", photoName, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return nil, "", &APIError{Code: resp.StatusCode, Status: resp.Status, Message: string(raw)}
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("reading photo %s: %w", photoName, err)
	}
	return data, resp.Header.Get("Content-Type"), nil
}
```

- [ ] **Step 4: Route by reference format in `photos_client.go`**

`MapsPhotoClient.placeImage` (`iowrappers/photos_client.go:183`) currently always calls the legacy `client.PlacePhoto`. Route on the prefix instead, because the details keyspace holds both formats during and after cutover — brand/keyword places stay on the legacy path in this PR:

```go
func (c *MapsPhotoClient) placeImage(ctx context.Context, ref string) (image.Image, error) {
	// Acquire semaphore for API rate limiting
	c.mapsClient.apiSemaphore <- struct{}{}
	defer func() { <-c.mapsClient.apiSemaphore }()

	// A "places/..." reference is a Places API (New) resource name and must go to the
	// New media endpoint. Legacy opaque photo_reference values stay on the SDK. Both
	// formats coexist: brand/keyword searches still write legacy references.
	if strings.HasPrefix(ref, placesv1.PhotoNamePrefix) {
		data, contentType, err := c.placesV1.FetchPhoto(ctx, ref, 400)
		if err != nil {
			return nil, err
		}
		Logger.Debugf("photo response content type is: %s", contentType)
		switch contentType {
		case "image/png":
			return png.Decode(bytes.NewReader(data))
		case "image/jpeg":
			return jpeg.Decode(bytes.NewReader(data))
		default:
			return nil, fmt.Errorf(UnknownImageFormat+": %s", contentType)
		}
	}

	resp, err := c.mapsClient.client.PlacePhoto(ctx, &maps.PlacePhotoRequest{PhotoReference: ref, MaxWidth: 400})
	if err != nil {
		return nil, err
	}
	Logger.Debugf("photo response content type is: %s", resp.ContentType)
	switch resp.ContentType {
	case "image/png":
		return png.Decode(resp.Data)
	case "image/jpeg":
		return resp.Image()
	default:
		return nil, fmt.Errorf(UnknownImageFormat+": %s", resp.ContentType)
	}
}
```

Add a `placesV1 *placesv1.Client` field to `MapsPhotoClient` and initialize it in `CreatePhotoClient` (`iowrappers/photos_client.go:55`) from the same API key.

Also update the stale-reference recovery branch in `GetPhotoURL` (`:131-160`): when the active version is v2, re-fetching a place's photo must come from a New API lookup rather than `PlaceDetailedSearch`. The simplest correct behavior for this PR is to skip recovery on v2 records and let the next cache refresh repopulate:

```go
		if strings.HasPrefix(err.Error(), UnknownImageFormat) {
			if strings.HasPrefix(photoRef, placesv1.PhotoNamePrefix) {
				// v2 records carry a resource name that cannot be repaired by a legacy
				// Place Details call. Let the 14-day cache refresh replace it.
				return "", fmt.Errorf("stale Places API (New) photo reference for place %s: %w", placeId, err)
			}
			// ... existing legacy recovery path unchanged ...
		}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go build -v . && go test ./iowrappers/... -v 2>&1 | tail -25`

Expected: PASS, including the existing `photos_client` tests on the legacy branch.

- [ ] **Step 6: Commit**

```bash
git add iowrappers/placesv1/photo.go iowrappers/placesv1/photo_test.go iowrappers/photos_client.go
git commit -m "feat: fetch photos from the Places API (New) media endpoint

Route on the reference format: 'places/{id}/photos/{ref}' resource names
go to /v1/{name}/media, opaque legacy photo_reference values stay on the
SDK. Both formats coexist because brand/keyword search is still legacy."
```

---

### Task 5: Measure coverage, then cut over

The one hard tradeoff in this design is the 20-result cap. Measure it on production data before flipping the flag. Do not skip this task, and do not tune `TypeGroups` by guesswork.

**Files:**
- Create: `iowrappers/coverage_compare_test.go` (build-tagged, opt-in)

- [ ] **Step 1: Write the comparison harness**

Create `iowrappers/coverage_compare_test.go`. The build tag keeps it out of CI, since it makes real billed API calls:

```go
//go:build coverage_compare

package iowrappers

import (
	"context"
	"os"
	"testing"

	"github.com/weihesdlegend/Vacation-planner/POI"
)

// TestCompareLegacyVsNewCoverage reports how many distinct places each API returns for
// the same request, so the 20-result searchNearby cap can be judged on real data rather
// than assumed acceptable.
//
// Run with:
//   GOOGLE_MAPS_API_KEY=... go test -tags coverage_compare ./iowrappers/ \
//     -run TestCompareLegacyVsNewCoverage -v
func TestCompareLegacyVsNewCoverage(t *testing.T) {
	apiKey := os.Getenv("GOOGLE_MAPS_API_KEY")
	if apiKey == "" {
		t.Skip("GOOGLE_MAPS_API_KEY not set")
	}
	if err := CreateLogger(); err != nil {
		t.Fatalf("CreateLogger: %v", err)
	}
	mapsClient := CreateMapsClient(apiKey)
	newClient := NewPlacesV1SearchClient(apiKey, mapsClient)

	locations := map[string]POI.Location{
		"los altos (dense suburb)": {Latitude: 37.38006, Longitude: -122.11612, City: "Los Altos", AdminAreaLevelOne: "CA", Country: "United States"},
		"manhattan (very dense)":   {Latitude: 40.7580, Longitude: -73.9855, City: "New York", AdminAreaLevelOne: "NY", Country: "United States"},
		"bozeman (sparse)":         {Latitude: 45.6796, Longitude: -111.0471, City: "Bozeman", AdminAreaLevelOne: "MT", Country: "United States"},
	}
	categories := []POI.PlaceCategory{POI.PlaceCategoryEatery, POI.PlaceCategoryVisit, POI.PlaceCategoryShopping}

	for name, loc := range locations {
		for _, cat := range categories {
			legacyReq := &PlaceSearchRequest{
				Location: loc, PlaceCat: cat, Radius: ColdStartSearchRadius,
				MinNumResults: 40, PriceLevel: POI.PriceLevelDefault,
				BusinessStatus: POI.Operational, AllPriceLevels: cat == POI.PlaceCategoryEatery,
			}
			legacy, err := mapsClient.NearbySearch(context.Background(), legacyReq)
			if err != nil {
				t.Errorf("%s/%s legacy: %v", name, cat, err)
				continue
			}

			singleReq := *legacyReq
			newClient.TypeGroups = SingleGroupTypes
			single, err := newClient.NearbySearch(context.Background(), &singleReq)
			if err != nil {
				t.Errorf("%s/%s new(single): %v", name, cat, err)
				continue
			}

			perTypeReq := *legacyReq
			newClient.TypeGroups = PerTypeGroups
			perType, err := newClient.NearbySearch(context.Background(), &perTypeReq)
			if err != nil {
				t.Errorf("%s/%s new(per-type): %v", name, cat, err)
				continue
			}

			// After ReclassifyForCategory, which is what actually reaches the response.
			t.Logf("%-26s %-9s legacy=%3d (kept %3d)  new-single=%3d  new-per-type=%3d",
				name, cat, len(legacy), countKept(legacy, cat), len(single), len(perType))
		}
	}
}

func countKept(places []POI.Place, cat POI.PlaceCategory) int {
	kept := 0
	for _, p := range places {
		if _, keep := POI.ReclassifyForCategory(p, cat); keep {
			kept++
		}
	}
	return kept
}
```

- [ ] **Step 2: Run the comparison and record the numbers**

Run:

```bash
GOOGLE_MAPS_API_KEY=$GOOGLE_MAPS_API_KEY go test -tags coverage_compare ./iowrappers/ \
  -run TestCompareLegacyVsNewCoverage -v 2>&1 | tee /tmp/coverage-compare.txt
```

Compare `legacy (kept N)` — the count that actually survives to the response today — against `new-single`. The kept count is the honest baseline, because the legacy raw count includes results `ReclassifyForCategory` discards.

- [ ] **Step 3: Choose the fan-out and record why**

Decision rule, to be written into the PR description with the measured numbers:

- If `new-single` >= the legacy kept count for every location and category, keep `SingleGroupTypes`.
- If any sparse or dense case regresses materially, set the default to `PerTypeGroups` for the affected categories. Encode it explicitly rather than leaving the default implicit:

```go
// TypeGroupsForCategory splits Eatery across per-type calls because a single
// 20-result searchNearby underperformed the legacy kept count in dense areas
// (see docs/superpowers/plans/ measurements, 2026-07-29). Other categories fit in one call.
func TypeGroupsForCategory(cat POI.PlaceCategory) [][]POI.LocationType {
	if cat == POI.PlaceCategoryEatery {
		return PerTypeGroups(cat)
	}
	return SingleGroupTypes(cat)
}
```

- [ ] **Step 4: Commit the harness and the decision**

```bash
git add iowrappers/coverage_compare_test.go iowrappers/places_v1_search_client.go
git commit -m "test: add legacy vs new coverage comparison harness

searchNearby caps at 20 results with no pagination, so the fan-out choice
has to be measured against production data rather than assumed. Build-tagged
out of CI because it makes real billed API calls."
```

- [ ] **Step 5: Cut over in staging, then production**

```bash
# 1. Enable on a staging/review app first.
heroku config:set PLACES_API_VERSION=new -a <staging-app>

# 2. Exercise a cold search per category and confirm v2 keys appear.
redis-cli --scan --pattern 'v2:placeIDs:*' | head

# 3. Confirm correct typing — the whole point of the migration.
#    No record in a v2 eatery bucket should have a lodging primary type.
curl -s -H "Authorization: Bearer $ADMIN_JWT" \
  "https://<staging-app>/v1/migrate/reclassify-buckets?category=Eatery" | jq '.report.misclassified'
# Expected: 0

# 4. Production.
heroku config:set PLACES_API_VERSION=new -a best-vacation-planner

# Rollback at any point, no deletes, legacy cache still warm:
heroku config:set PLACES_API_VERSION=legacy -a best-vacation-planner
```

---

### Task 6: Re-add `fast_food_restaurant` and `food_court`

Only after Task 5's cutover is stable in production. This is the original intent of commit `8644199`, now actually achievable.

**Files:**
- Modify: `POI/categories.go`
- Test: `test/place_category_test.go`

- [ ] **Step 1: Write the failing test**

In `test/place_category_test.go`, extend the Eatery entry of `TestGetPlaceTypesByCategory` and add:

```go
// TestFastFoodTypesRoundTrip pins that the Places API (New) Table A eatery types are
// mapped in BOTH directions. Commit 8644199 added them to GetPlaceTypes only; the
// GetPlaceCategory default silently absorbed them into Eatery and the round-trip guard
// could not fail. Both directions must be explicit now.
func TestFastFoodTypesRoundTrip(t *testing.T) {
	for _, placeType := range []POI.LocationType{POI.LocationTypeFastFood, POI.LocationTypeFoodCourt} {
		got, ok := POI.GetPlaceCategory(placeType)
		if !ok {
			t.Errorf("GetPlaceCategory(%q) returned ok=false, want Eatery", placeType)
			continue
		}
		if got != POI.PlaceCategoryEatery {
			t.Errorf("GetPlaceCategory(%q) = %q, want Eatery", placeType, got)
		}
	}
	types := POI.GetPlaceTypes(POI.PlaceCategoryEatery)
	for _, want := range []POI.LocationType{POI.LocationTypeFastFood, POI.LocationTypeFoodCourt} {
		found := false
		for _, t2 := range types {
			if t2 == want {
				found = true
			}
		}
		if !found {
			t.Errorf("GetPlaceTypes(Eatery) = %v, missing %q", types, want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./test/ -run TestFastFoodTypesRoundTrip -v`

Expected: FAIL with `undefined: POI.LocationTypeFastFood`.

- [ ] **Step 3: Add the constants and both mappings**

In `POI/categories.go`, restore the constants:

```go
	// LocationTypeFastFood and LocationTypeFoodCourt are Places API (New) Table A types.
	// They only work when PLACES_API_VERSION=new: the legacy Nearby Search does not
	// define them, ignores the ?type= filter rather than erroring, and its response
	// types[] never contains them, so nothing can classify a place as either one.
	LocationTypeFastFood  = LocationType("fast_food_restaurant")
	LocationTypeFoodCourt = LocationType("food_court")
```

Add them to **both** functions — this is the invariant that was violated the first time:

```go
	// in GetPlaceCategory
	case LocationTypeCafe, LocationTypeRestaurant, LocationTypeBar, LocationTypeBakery,
		LocationTypeMealTakeaway, LocationTypeFastFood, LocationTypeFoodCourt:
		return PlaceCategoryEatery, true

	// in GetPlaceTypes
	case PlaceCategoryEatery:
		placeTypes = append(placeTypes,
			[]LocationType{LocationTypeCafe, LocationTypeRestaurant, LocationTypeBar,
				LocationTypeBakery, LocationTypeMealTakeaway, LocationTypeFastFood,
				LocationTypeFoodCourt}...)
```

- [ ] **Step 4: Guard the legacy path against them**

The legacy `CreateMapSearchRequest` validation added in the prior PR now rejects these two types, which is correct — but it would log an error on every legacy search. Make the legacy client skip them quietly instead, in `extensiveNearbySearch` where `placeTypes` is built:

```go
	placeTypes := POI.GetPlaceTypes(request.PlaceCat) // get place types in a category
	// Drop types the legacy Places API does not define. They are searched only when
	// PLACES_API_VERSION=new; sending them here would spend a call whose type filter
	// Google silently ignores.
	placeTypes = Filter(placeTypes, func(t POI.LocationType) bool {
		if t == POI.LocationTypeAny {
			return true
		}
		_, err := maps.ParsePlaceType(string(t))
		return err == nil
	})
```

- [ ] **Step 5: Run the full suite**

Run: `go build -v . && go test ./... 2>&1 | tail -20`

Expected: PASS, including `TestPlaceCategoryRoundTrip` and the legacy `CreateMapSearchRequest` tests.

- [ ] **Step 6: Verify against the live New API**

```bash
curl -s -X POST 'https://places.googleapis.com/v1/places:searchNearby' \
  -H "X-Goog-Api-Key: $GOOGLE_MAPS_API_KEY" \
  -H 'X-Goog-FieldMask: places.displayName,places.primaryType,places.types' \
  -H 'Content-Type: application/json' \
  -d '{"includedPrimaryTypes":["fast_food_restaurant","food_court"],"maxResultCount":20,
       "rankPreference":"DISTANCE",
       "locationRestriction":{"circle":{"center":{"latitude":37.38006,"longitude":-122.11612},"radius":8000}}}' \
  | jq '.places[] | {name: .displayName.text, primaryType}'
```

Expected: every `primaryType` is `fast_food_restaurant` or `food_court`, and no hotels appear. That is the concrete difference from the legacy behavior that started this work.

- [ ] **Step 7: Commit**

```bash
git add POI/categories.go iowrappers/nearby_search.go test/place_category_test.go
git commit -m "feat: search fast_food_restaurant and food_court on the New API

These Table A types are filtered server-side by includedPrimaryTypes, so
they now return correctly typed places instead of prominence-ranked
establishments. Mapped in both GetPlaceTypes and GetPlaceCategory, and
filtered out of the legacy path where they are undefined."
```

---

## Verification checklist

- [ ] `go build -v .` and `go test -v ./...` pass with `PLACES_API_VERSION` unset (legacy path untouched).
- [ ] `go test -v ./...` passes with `PLACES_API_VERSION=new`.
- [ ] `grep -rn 'places.googleapis.com' --include='*.go' iowrappers/ | grep -v _test` shows requests only from `iowrappers/placesv1`.
- [ ] Coverage comparison numbers recorded in the PR description, with the `TypeGroups` choice justified by them.
- [ ] After staging cutover, `reclassify-buckets?category=Eatery` reports `misclassified: 0` against v2 buckets.
- [ ] A saved trip plan created before cutover still renders (exercises `getPlaceAnyVersion`).
- [ ] Photos load for both a v2 place and a legacy brand-search place.
- [ ] Rollback tested: set `PLACES_API_VERSION=legacy`, confirm legacy results still serve from the warm legacy cache.

## Deliberately out of scope

- **Brand/keyword search.** `searchNearby` has no keyword parameter; this needs `places:searchText` with `locationBias`, plus `MatchesBrandName`/`StrictNameMatch` re-tested against relevance-ranked results. `PlacesV1SearchClient.NearbySearch` delegates keyword requests to the legacy client until then.
- **Geocoding and ReverseGeocode.** `/maps/api/geocode/json` is the Geocoding API, not Places, and is not deprecated. It stays on `googlemaps.github.io/maps` v1.7.0.
- **Place Details.** Once search returns the full field set, the only remaining legacy Place Details caller is the stale-photo recovery path and the `data_migrations.go` backfills. Retire those separately.
- **Removing `googlemaps.github.io/maps`.** Cannot happen while Geocoding and brand search remain on it.

## Risk notes

- **Billing.** The New API bills per SKU tier by field mask. `SearchNearbyFieldMask` requests Enterprise-tier fields (`regularOpeningHours`, `editorialSummary`). Per-search cost may rise even as call count falls sharply — check the first days of billing after cutover rather than assuming the call-count reduction dominates.
- **The 20-result cap is the one-way door in this design.** Task 5 exists specifically to size it. If coverage proves unacceptable even with `PerTypeGroups`, the fallback is `places:searchText` per type, which paginates to 60 — a larger change that would supersede Task 3's client.
- **Legacy is not dead, but it is frozen.** Legacy Places became unavailable to Cloud projects created after 2025-03-01 and receives no fixes; Google has announced no turn-down date and promises 12 months' notice. The concrete risk is that recreating or swapping the GCP project behind `GOOGLE_MAPS_API_KEY` would break the legacy path outright — which is also why the brand-search path should not stay on it indefinitely.