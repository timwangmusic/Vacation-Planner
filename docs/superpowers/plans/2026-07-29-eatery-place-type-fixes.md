# Eatery Place-Type Misclassification Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stop the legacy Places API from writing mislabeled places into the category geo buckets, make the existing round-trip guard actually capable of failing, fix result truncation so it drops the farthest places rather than the last-searched place types, and purge the records already written to prod.

**Architecture:** Four independent forward fixes on `origin/master`. The root cause is that `POI.GetPlaceCategory` has a `default:` branch returning `PlaceCategoryEatery`, which silently absorbs any place type the legacy Nearby Search does not understand. We convert that function to the `(value, ok)` shape already used by `ParsePlaceCategory`, so the compiler forces all three call sites to decide what an unknown type means; then we remove the two New-API-only types, reject unknown types at request-build time, sort by distance before truncating, and ship an admin migration to clean the buckets.

**Tech Stack:** Go 1.24, Gin, go-redis v9, `googlemaps.github.io/maps` v1.7.0 (legacy Places API), testify.

## Global Constraints

- Go version: `1.24.0` (from `go.mod`) — do not raise it.
- Add no new module dependencies in this PR.
- CI gates are exactly `go build -v .` then `go test -v ./...` (`.github/workflows/go.yml`). Both must pass.
- Target branch is `origin/master` (the repo default; `origin/main` is stale at `1a1417b` and is not the deploy path). Commit `8644199` is already on `master` and live on Heroku app `best-vacation-planner`.
- Do not migrate to Places API (New) in this PR. That is the follow-up plan, `2026-07-29-places-api-new-migration.md`.
- Never run destructive Redis commands by hand against prod. Cleanup ships as a dry-run-by-default admin endpoint (Task 4).

---

## Background: what is actually broken

Verified against the code at `8644199`:

1. `POI/categories.go:39-40` added `LocationTypeFastFood = "fast_food_restaurant"` and `LocationTypeFoodCourt = "food_court"`. Neither string exists anywhere in `googlemaps.github.io/maps@v1.7.0` — they are Places API (New) Table A types.
2. `iowrappers/nearby_search.go:87` builds the request with a direct cast, `Type: maps.PlaceType(placeType)`, bypassing the SDK's own `maps.ParsePlaceType` validator (`types.go:301`). The SDK then does `q.Set("type", string(r.Type))` (`places.go:125`) unchecked, so the unknown string reaches Google verbatim.
3. Legacy Nearby Search accepts the parameter, ignores the filter, and returns prominence-ranked establishments. `parsePlacesSearchResponse(searchResp, placeType, ...)` (`nearby_search.go:224`) then stamps the **queried** type onto every result via `POI.CreatePlace(..., locationType, ...)` (`:419`), so hotels get `LocationType: fast_food_restaurant`. `:421` (`place.Types = res.Types`) keeps Google's truthful types, which is why the damage is visible.
4. `iowrappers/redis_client.go:222` writes each place under `EncodeNearbySearchRedisKey(GetPlaceCategory(place.LocationType), place.PriceLevel)`. Neither new type appears in `GetPlaceCategory`'s Eatery case, so they reach Eatery through `default:` (`categories.go:77-78`) — meaning the invariant documented at `categories.go:61-64` was broken by the commit and the `default` hid it.
5. `TestPlaceCategoryRoundTrip` (`test/place_category_test.go`) passed anyway, because that `default` makes the test un-failable for *any* unknown type. The guard provides no protection today.
6. `POI.ReclassifyForCategory` (`categories.go:154-166`) drops these from API responses (primary type `lodging` is not in `GetPlaceTypes(Eatery)`), which is why the endpoint looks clean while the cache is dirty.

Confirmed non-issues, so nobody wastes time on them:

- These records do **not** consume result slots in the response. `planner/planner.go` reclassifies at `:1406` *before* truncating at `:1414`, and the Redis read is unbounded (`GeoRadius` with no `Count`, `redis_client.go:484-489`).
- They did **not** waste Place Details spend. `detailsBudget` (`nearby_search.go:150`) is shared and consumed in `placeTypes` order, so `cafe`/`restaurant` exhaust it before the junk types are processed.
- Zero `food_court`-tagged places in prod is expected, not a contradiction. `placeMap` dedups by place ID across all types in one search, and `fast_food_restaurant` is processed first, so an identical ignored-filter response for `food_court` is entirely deduped away.

The real cache-side cost is that these records inflate `len(cachedQualifiedPlaces)`, which is the radius-doubling break condition at `redis_client.go:520` — junk can satisfy `MinNumResults` and stop the radius from growing, so sparse areas return fewer genuine eateries.

---

## File Structure

| File | Responsibility in this PR |
| --- | --- |
| `POI/categories.go` | Remove the two New-API-only types from `GetPlaceTypes(Eatery)`; change `GetPlaceCategory` to `(PlaceCategory, bool)`; delete the two unused constants. |
| `iowrappers/redis_client.go` | Skip + log places whose type has no category, instead of writing them to Eatery (`:182`, `:222`). |
| `planner/planner.go` | Handle the new `ok` return at `:810`; sort by distance before truncating at `:1414`. |
| `iowrappers/nearby_search.go` | Validate place types in `CreateMapSearchRequest`; fix the dead `maxRetries` cap. |
| `iowrappers/data_migrations.go` | Add `RemoveMisclassifiedPlacesFromCategoryBuckets` for prod cleanup. |
| `iowrappers/maps_client.go` | Extend the `SearchClient`/migration interface with the new cleanup method. |
| `test/place_category_test.go` | Update for the new signature; the round-trip guard becomes meaningful. |
| `iowrappers/nearby_search_validation_test.go` (new) | Unit tests for place-type validation. |
| `iowrappers/place_distance_sort_test.go` (new) | Unit tests for distance sorting. |

---

### Task 1: Make unknown place types un-mappable, and remove the two New-API-only types

This is the root-cause fix. It must land as one commit — changing `GetPlaceCategory`'s signature without removing the two types would leave the build red on the round-trip test, which is exactly the point of the guard.

**Files:**
- Modify: `POI/categories.go:37-79` (constants, `GetPlaceCategory`, `GetPlaceTypes`)
- Modify: `iowrappers/redis_client.go:182`, `iowrappers/redis_client.go:222`
- Modify: `planner/planner.go:810`
- Test: `test/place_category_test.go`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: `POI.GetPlaceCategory(placeType LocationType) (PlaceCategory, bool)` — returns `("", false)` when the type maps to no category. Tasks 2 and 4 rely on this exact signature.

- [ ] **Step 1: Write the failing test**

Add to `test/place_category_test.go`:

```go
// TestGetPlaceCategoryRejectsUnknownTypes pins the fix for the fast_food_restaurant
// incident: GetPlaceCategory must NOT silently absorb unmapped types into Eatery.
// A default-to-Eatery branch made TestPlaceCategoryRoundTrip un-failable, so two
// Places-API-(New)-only types were added to GetPlaceTypes(Eatery) and hotels were
// written into the eatery geo buckets.
func TestGetPlaceCategoryRejectsUnknownTypes(t *testing.T) {
	unknown := []POI.LocationType{
		POI.LocationType("fast_food_restaurant"),
		POI.LocationType("food_court"),
		POI.LocationType("lodging_but_not_really"),
		POI.LocationType(""),
	}
	for _, placeType := range unknown {
		if got, ok := POI.GetPlaceCategory(placeType); ok {
			t.Errorf("GetPlaceCategory(%q) = (%q, true), want ok=false", placeType, got)
		}
	}
}

// TestGetPlaceCategoryKnownTypes pins that every mapped type still resolves.
func TestGetPlaceCategoryKnownTypes(t *testing.T) {
	cases := map[POI.LocationType]POI.PlaceCategory{
		POI.LocationTypeCafe:         POI.PlaceCategoryEatery,
		POI.LocationTypeRestaurant:   POI.PlaceCategoryEatery,
		POI.LocationTypeBar:          POI.PlaceCategoryEatery,
		POI.LocationTypeBakery:       POI.PlaceCategoryEatery,
		POI.LocationTypeMealTakeaway: POI.PlaceCategoryEatery,
		POI.LocationTypePark:         POI.PlaceCategoryVisit,
		POI.LocationTypeMuseum:       POI.PlaceCategoryVisit,
		POI.LocationTypeStore:        POI.PlaceCategoryShopping,
		POI.LocationTypeLodging:      POI.PlaceCategoryLodging,
		POI.LocationTypeGym:          POI.PlaceCategoryWellness,
	}
	for placeType, want := range cases {
		got, ok := POI.GetPlaceCategory(placeType)
		if !ok {
			t.Errorf("GetPlaceCategory(%q) returned ok=false, want %q", placeType, want)
			continue
		}
		if got != want {
			t.Errorf("GetPlaceCategory(%q) = %q, want %q", placeType, got, want)
		}
	}
}
```

Update the two existing tests in the same file for the new signature and the reverted type list:

```go
// in TestGetPlaceTypesByCategory, the Eatery entry becomes:
		POI.PlaceCategoryEatery: {
			POI.LocationTypeCafe, POI.LocationTypeRestaurant,
			POI.LocationTypeBar, POI.LocationTypeBakery, POI.LocationTypeMealTakeaway,
		},

// in TestPlaceCategoryRoundTrip, the assertion becomes:
			got, ok := POI.GetPlaceCategory(placeType)
			if !ok {
				t.Errorf("round-trip broken: GetPlaceCategory(%q) has no category, want %q", placeType, category)
				continue
			}
			if got != category {
				t.Errorf("round-trip broken: GetPlaceCategory(%q) = %q, want %q", placeType, got, category)
			}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./test/ -run 'TestGetPlaceCategory|TestPlaceCategoryRoundTrip|TestGetPlaceTypesByCategory' -v`

Expected: FAIL to compile with `assignment mismatch: 2 variables but POI.GetPlaceCategory returns 1 value`. That compile failure is the expected red state.

- [ ] **Step 3: Change `GetPlaceCategory` and remove the two types**

In `POI/categories.go`, delete these two constant lines (they have no remaining caller):

```go
	LocationTypeFastFood      = LocationType("fast_food_restaurant")
	LocationTypeFoodCourt     = LocationType("food_court")
```

Replace `GetPlaceCategory` (currently `categories.go:61-79`) with:

```go
// GetPlaceCategory maps a Google Maps place type back to its category, reporting whether
// the type is mapped at all. It is the inverse of GetPlaceTypes and MUST stay consistent
// with it: the nearby-search cache writes each place under
// EncodeNearbySearchRedisKey(GetPlaceCategory(place.LocationType), ...), so a type that
// resolves to a different category than the one it was searched under would never cache-hit.
//
// It deliberately has NO default category. An earlier version defaulted to Eatery, which
// silently absorbed place types the legacy Nearby Search does not understand — two
// Places-API-(New)-only types ("fast_food_restaurant", "food_court") were added to
// GetPlaceTypes(Eatery), Google ignored the unenforceable filter, and prominence-ranked
// hotels were written into the eatery geo buckets. Returning ok=false forces every caller
// to decide what an unmapped type means, and makes TestPlaceCategoryRoundTrip able to fail.
func GetPlaceCategory(placeType LocationType) (PlaceCategory, bool) {
	switch placeType {
	case LocationTypePark, LocationTypeAmusementPark, LocationTypeGallery, LocationTypeMuseum:
		return PlaceCategoryVisit, true
	case LocationTypeCafe, LocationTypeRestaurant, LocationTypeBar, LocationTypeBakery, LocationTypeMealTakeaway:
		return PlaceCategoryEatery, true
	case LocationTypeShoppingMall, LocationTypeDepartmentStore, LocationTypeSupermarket, LocationTypeClothingStore, LocationTypeStore:
		return PlaceCategoryShopping, true
	case LocationTypeLodging:
		return PlaceCategoryLodging, true
	case LocationTypeGym, LocationTypeSpa, LocationTypePharmacy:
		return PlaceCategoryWellness, true
	default:
		return PlaceCategory(""), false
	}
}
```

Revert the Eatery line in `GetPlaceTypes` back to five types:

```go
	case PlaceCategoryEatery:
		placeTypes = append(placeTypes,
			[]LocationType{LocationTypeCafe, LocationTypeRestaurant, LocationTypeBar, LocationTypeBakery, LocationTypeMealTakeaway}...)
```

- [ ] **Step 4: Update the three call sites**

`iowrappers/redis_client.go:222` — inside the `SetPlacesAddGeoLocations` pipeline. This is the write path that caused the incident, so it must refuse to guess:

```go
				for _, place := range placeBatch {
					placeCategory, ok := POI.GetPlaceCategory(place.LocationType)
					if !ok {
						// Refuse to guess a bucket. A place whose type maps to no category
						// came from a search whose type filter Google did not enforce, so
						// writing it would poison whichever bucket we picked.
						Logger.Errorf("SetPlacesAddGeoLocations: place %s has unmapped location type %q, skipping geo bucket write",
							place.ID, place.LocationType)
						continue
					}
					geoLocation := &redis.GeoLocation{
						Name:      place.ID,
						Latitude:  place.GetLocation().Latitude,
						Longitude: place.GetLocation().Longitude,
					}

					redisKey := POI.EncodeNearbySearchRedisKey(placeCategory, place.PriceLevel)
					pipe.GeoAdd(c, redisKey, geoLocation)
```

`iowrappers/redis_client.go:182` — inside the deprecated `StorePlacesForLocation`. Same rule, minimal change:

```go
	for _, place := range places {
		placeCategory, ok := POI.GetPlaceCategory(place.LocationType)
		if !ok {
			Logger.Errorf("StorePlacesForLocation: place %s has unmapped location type %q, skipping",
				place.ID, place.LocationType)
			continue
		}
		sortedSetKey := strings.Join([]string{geocodeInString, string(placeCategory)}, "_")
```

`planner/planner.go:810` — this reads *saved plan* records out of `place_details:place_ID:`, which can include older cached places with legacy or empty types (brand searches write `LocationType: ""`). Preserve today's observable response here rather than changing an unrelated endpoint's output:

```go
			// Saved plans can contain older cached records, including brand-search places
			// written with an empty LocationType. Preserve the historical Eatery default for
			// display only — the write path (redis_client.go) is where guessing is unsafe.
			if placeCategory, ok := POI.GetPlaceCategory(place.LocationType); ok {
				resp.PlaceCategories[i] = placeCategory
			} else {
				resp.PlaceCategories[i] = POI.PlaceCategoryEatery
			}
```

- [ ] **Step 5: Run the full suite to verify it passes**

Run: `go build -v . && go test ./... 2>&1 | tail -30`

Expected: PASS. In particular `TestGetPlaceCategoryRejectsUnknownTypes`, `TestGetPlaceCategoryKnownTypes`, `TestPlaceCategoryRoundTrip` and `TestGetPlaceTypesByCategory` all pass, and no package fails to compile.

- [ ] **Step 6: Commit**

```bash
git add POI/categories.go iowrappers/redis_client.go planner/planner.go test/place_category_test.go
git commit -m "fix: stop mapping unknown place types to Eatery

GetPlaceCategory had a default branch returning Eatery, which silently
absorbed any place type the legacy Nearby Search does not understand. That
made TestPlaceCategoryRoundTrip un-failable, so fast_food_restaurant and
food_court (Places API (New) Table A types, absent from the v1.7.0 SDK)
were added to GetPlaceTypes(Eatery). Google ignored the unenforceable type
filter and returned prominence-ranked establishments, which were stamped
with the queried type and written into placeIDs:eatery:level* as hotels.

Return (PlaceCategory, bool) so the compiler forces every caller to handle
an unmapped type, refuse the geo-bucket write instead of guessing, and
remove the two types."
```

---

### Task 2: Reject unknown place types when building the Maps request

Defense in depth: Task 1 stops bad data reaching Redis, this stops the useless API call being made at all, and makes the failure loud.

**Files:**
- Modify: `iowrappers/nearby_search.go:76-100` (`CreateMapSearchRequest`), `:141` (`maxRetries`), `:170-205` (Phase A/B)
- Test: `iowrappers/nearby_search_validation_test.go` (create)

**Interfaces:**
- Consumes: nothing from Task 1 (independent).
- Produces: `CreateMapSearchRequest(reqIn *PlaceSearchRequest, placeType POI.LocationType, token string) (maps.NearbySearchRequest, error)` — returns a non-nil error when `placeType` is neither `POI.LocationTypeAny` nor a type `maps.ParsePlaceType` accepts.

- [ ] **Step 1: Write the failing test**

Create `iowrappers/nearby_search_validation_test.go`:

```go
package iowrappers

import (
	"strings"
	"testing"

	"github.com/weihesdlegend/Vacation-planner/POI"
)

// TestCreateMapSearchRequestRejectsUnknownPlaceType guards the fast_food_restaurant
// incident at the request boundary. The SDK casts POI.LocationType straight to
// maps.PlaceType and forwards it as ?type=, so an unknown value silently disables
// the filter server-side instead of erroring. Validate before spending the call.
func TestCreateMapSearchRequestRejectsUnknownPlaceType(t *testing.T) {
	req := &PlaceSearchRequest{
		Location:   POI.Location{Latitude: 37.38006, Longitude: -122.11612},
		PlaceCat:   POI.PlaceCategoryEatery,
		Radius:     8000,
		PriceLevel: POI.PriceLevelTwo,
	}
	for _, placeType := range []POI.LocationType{
		POI.LocationType("fast_food_restaurant"),
		POI.LocationType("food_court"),
		POI.LocationType("not_a_google_type"),
	} {
		if _, err := CreateMapSearchRequest(req, placeType, ""); err == nil {
			t.Errorf("CreateMapSearchRequest(%q) returned nil error, want validation failure", placeType)
		} else if !strings.Contains(err.Error(), string(placeType)) {
			t.Errorf("CreateMapSearchRequest(%q) error %q should name the offending type", placeType, err)
		}
	}
}

// TestCreateMapSearchRequestAcceptsKnownPlaceTypes pins that every type the
// categories actually search for still builds a request.
func TestCreateMapSearchRequestAcceptsKnownPlaceTypes(t *testing.T) {
	req := &PlaceSearchRequest{
		Location:   POI.Location{Latitude: 37.38006, Longitude: -122.11612},
		PlaceCat:   POI.PlaceCategoryEatery,
		Radius:     8000,
		PriceLevel: POI.PriceLevelTwo,
	}
	categories := []POI.PlaceCategory{
		POI.PlaceCategoryVisit, POI.PlaceCategoryEatery,
		POI.PlaceCategoryShopping, POI.PlaceCategoryLodging, POI.PlaceCategoryWellness,
	}
	for _, category := range categories {
		for _, placeType := range POI.GetPlaceTypes(category) {
			got, err := CreateMapSearchRequest(req, placeType, "")
			if err != nil {
				t.Errorf("CreateMapSearchRequest(%q) in category %q: unexpected error %v", placeType, category, err)
				continue
			}
			if string(got.Type) != string(placeType) {
				t.Errorf("CreateMapSearchRequest(%q) set Type=%q, want %q", placeType, got.Type, placeType)
			}
		}
	}
}

// TestCreateMapSearchRequestAcceptsAnyType pins that keyword (brand) searches,
// which intentionally leave the type unset, are not rejected.
func TestCreateMapSearchRequestAcceptsAnyType(t *testing.T) {
	req := &PlaceSearchRequest{
		Location: POI.Location{Latitude: 37.38006, Longitude: -122.11612},
		PlaceCat: POI.PlaceCategoryEatery,
		Radius:   8000,
		Keyword:  "Dunkin'",
	}
	got, err := CreateMapSearchRequest(req, POI.LocationTypeAny, "")
	if err != nil {
		t.Fatalf("CreateMapSearchRequest(LocationTypeAny) returned error %v, want nil", err)
	}
	if got.Type != "" {
		t.Errorf("CreateMapSearchRequest(LocationTypeAny) set Type=%q, want empty", got.Type)
	}
	if got.Keyword != "Dunkin'" {
		t.Errorf("CreateMapSearchRequest kept Keyword=%q, want %q", got.Keyword, "Dunkin'")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./iowrappers/ -run TestCreateMapSearchRequest -v`

Expected: FAIL to compile with `assignment mismatch: 2 variables but CreateMapSearchRequest returns 1 value`.

- [ ] **Step 3: Add validation to `CreateMapSearchRequest`**

Replace the function at `iowrappers/nearby_search.go:75-100`:

```go
// CreateMapSearchRequest creates a NearbySearchRequest for maps NearbySearch, adjust key settings such as radius and price levels.
// It rejects any place type the legacy Places API does not define: POI.LocationType is cast
// straight to maps.PlaceType and forwarded as ?type=, and Google responds to an unknown value
// by IGNORING the filter and returning prominence-ranked establishments rather than erroring.
// Those results are then stamped with the queried type, so an unvalidated type silently
// poisons the cache. maps.ParsePlaceType is the SDK's own list of legal values.
func CreateMapSearchRequest(reqIn *PlaceSearchRequest, placeType POI.LocationType, token string) (maps.NearbySearchRequest, error) {
	// LocationTypeAny is the keyword (brand) search case: the type is deliberately unset so
	// Google matches the keyword across all place types.
	if placeType != POI.LocationTypeAny {
		if _, err := maps.ParsePlaceType(string(placeType)); err != nil {
			return maps.NearbySearchRequest{}, fmt.Errorf(
				"place type %q is not a legacy Places API type (Places API (New) types are not accepted by /maps/api/place/nearbysearch): %w",
				placeType, err)
		}
	}

	// Adjust radius, minPrice and maxPrice settings in search request
	var radius = reqIn.Radius
	var exactPriceLevel maps.PriceLevel
	if POI.PriceyEatery(reqIn.PlaceCat, reqIn.PriceLevel) {
		// increase search radius
		radius = min(reqIn.Radius*4, GoogleNearbySearchMaxRadiusInMeters)
		// set price filter
		exactPriceLevel = maps.PriceLevel(fmt.Sprint(reqIn.PriceLevel))
	}

	return maps.NearbySearchRequest{
		Type: maps.PlaceType(placeType),
		Location: &maps.LatLng{
			Lat: reqIn.Location.Latitude,
			Lng: reqIn.Location.Longitude,
		},
		Keyword:   reqIn.Keyword,
		Radius:    radius,
		PageToken: token,
		RankBy:    maps.RankBy("prominence"),
		MinPrice:  exactPriceLevel,
		MaxPrice:  exactPriceLevel,
	}, nil
}
```

- [ ] **Step 4: Handle the error in Phase A and repair the dead retry cap**

In `extensiveNearbySearch`, `iowrappers/nearby_search.go:141`, the cap is computed while `reqTimes` is still 0, so `maxRetries` is always 0 and the `break outer` at `:205` is unreachable. Fix it to the intended "every place type failed this round" meaning:

```go
	var reqTimes uint = 0        // number of queries for each location type
	var totalPlaceCount uint = 0 // number of results so far, keep this number low
	// Bail out once every place type has failed once. This was previously computed as
	// reqTimes * len(placeTypes) while reqTimes was still 0, making the cap 0 and the
	// break below unreachable, so a fully failing search span every retry round.
	maxRetries := uint(len(placeTypes))
```

In the Phase A goroutine at `:170-187`, surface the validation error the same way a fetch error is surfaced:

```go
			go func(i int, placeType POI.LocationType, token string) {
				defer wg.Done()
				searchReq, reqErr := CreateMapSearchRequest(request, placeType, token)
				if reqErr != nil {
					fetched[i].err = reqErr
					return
				}
				select {
				case c.apiSemaphore <- struct{}{}:
					defer func() { <-c.apiSemaphore }()
				case <-ctx.Done():
					fetched[i].err = ctx.Err()
					return
				}
				fetched[i].resp, fetched[i].err = c.GoogleMapsNearbySearchWrapper(ctx, searchReq)
			}(i, placeType, nextPageTokenMap[placeType])
```

Then at `:199-205`, change the equality check to `>=` so a repaired cap cannot be stepped over:

```go
			if fetched[i].err != nil {
				Logger.Error(fmt.Errorf("places nearby search with Maps failed for place type %s with error: %w",
					placeType, fetched[i].err))
				mapsFailuresCount++
				if mapsFailuresCount >= maxRetries {
					break outer
				}
				// we should still retry for the next place type if the number of failures is below maxRetries
				continue
			}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go build -v . && go test ./iowrappers/ -run TestCreateMapSearchRequest -v && go test ./... 2>&1 | tail -20`

Expected: PASS on all three new tests and no regressions. `iowrappers/nearby_search_test.go` and `test/redis_client_mocks/...` must still pass.

- [ ] **Step 6: Commit**

```bash
git add iowrappers/nearby_search.go iowrappers/nearby_search_validation_test.go
git commit -m "fix: reject non-legacy place types before calling Nearby Search

POI.LocationType was cast straight to maps.PlaceType and forwarded as
?type=. Google answers an unknown type by ignoring the filter, not by
erroring, so the call silently returns prominence-ranked establishments
that then get stamped with the queried type. Validate against
maps.ParsePlaceType first and fail loudly.

Also repair the retry cap: maxRetries was computed as
reqTimes * len(placeTypes) while reqTimes was 0, so it was always 0 and
the break was dead code."
```

---

### Task 3: Sort by distance before truncating category results

Independent pre-existing bug, worth its own commit. `planner/planner.go:1413` claims "Redis results are sorted by distance ascending", which is only true on the cache path. On the fresh path, results are appended per place type in `GetPlaceTypes` order, each type's page in Google prominence order (`nearby_search.go:224`). So a cold search produces cafe×20, restaurant×20, bar×20, bakery×20, meal_takeaway×20 and `places[:40]` keeps roughly cafes and restaurants while dropping bar, bakery and meal_takeaway entirely — the exact types commit `e299558` was added to surface.

**Files:**
- Create: `iowrappers/place_distance_sort.go`
- Modify: `planner/planner.go:1411-1417`
- Test: `iowrappers/place_distance_sort_test.go`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: `iowrappers.SortPlacesByDistance(places []POI.Place, lat, lng float64)` — sorts `places` in place, ascending by haversine distance from `(lat, lng)`, stable so equal distances keep their prior order.

- [ ] **Step 1: Write the failing test**

Create `iowrappers/place_distance_sort_test.go`:

```go
package iowrappers

import (
	"testing"

	"github.com/weihesdlegend/Vacation-planner/POI"
)

func placeAt(id string, lat, lng float64) POI.Place {
	var p POI.Place
	p.SetID(id)
	p.SetLocationCoordinates([2]float64{lat, lng})
	return p
}

// TestSortPlacesByDistance pins that truncation keeps the NEAREST places. The fresh
// (Google) path returns results grouped by place type in prominence order, so slicing
// without sorting first drops whole place types and can rank a 3km result above a 250m one.
func TestSortPlacesByDistance(t *testing.T) {
	// State Street Market, Los Altos
	lat, lng := 37.38006, -122.11612

	places := []POI.Place{
		placeAt("far-sunnyvale", 37.3688, -122.0363),  // ~7km east
		placeAt("mid-mountainview", 37.3861, -122.0839), // ~3km east
		placeAt("near-state-st", 37.38025, -122.11655),  // ~40m away
	}

	SortPlacesByDistance(places, lat, lng)

	want := []string{"near-state-st", "mid-mountainview", "far-sunnyvale"}
	for i, id := range want {
		if places[i].GetID() != id {
			t.Errorf("position %d = %q, want %q (full order: %v)", i, places[i].GetID(), id, placeIDs(places))
		}
	}
}

// TestSortPlacesByDistanceEmpty pins that the no-result case does not panic.
func TestSortPlacesByDistanceEmpty(t *testing.T) {
	var places []POI.Place
	SortPlacesByDistance(places, 37.38006, -122.11612)
	if len(places) != 0 {
		t.Errorf("got %d places, want 0", len(places))
	}
}

func placeIDs(places []POI.Place) []string {
	ids := make([]string, 0, len(places))
	for _, p := range places {
		ids = append(ids, p.GetID())
	}
	return ids
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./iowrappers/ -run TestSortPlacesByDistance -v`

Expected: FAIL with `undefined: SortPlacesByDistance`.

- [ ] **Step 3: Write the implementation**

Create `iowrappers/place_distance_sort.go`:

```go
package iowrappers

import (
	"sort"

	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/utils"
)

// SortPlacesByDistance orders places ascending by distance from (lat, lng).
//
// Callers that truncate a candidate list to a limit MUST sort first. Only the Redis
// cache path returns places in distance order; the fresh path appends one place type's
// results after another in Google prominence order, so an unsorted slice[:limit] drops
// the last place types wholesale and can rank a 3km result above a 250m one.
//
// The sort is stable so places at equal distance keep their existing relative order.
func SortPlacesByDistance(places []POI.Place, lat, lng float64) {
	origin := []float64{lat, lng}
	dist := make(map[int]float64, len(places))
	for i := range places {
		loc := places[i].GetLocation()
		dist[i] = utils.HaversineDist(origin, []float64{loc.Latitude, loc.Longitude})
	}
	idx := make([]int, len(places))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool { return dist[idx[a]] < dist[idx[b]] })

	sorted := make([]POI.Place, len(places))
	for newPos, oldPos := range idx {
		sorted[newPos] = places[oldPos]
	}
	copy(places, sorted)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./iowrappers/ -run TestSortPlacesByDistance -v`

Expected: PASS on both tests.

- [ ] **Step 5: Use it in the category handler**

Replace `planner/planner.go:1411-1417`:

```go
				// drop places explicitly marked closed on the requested day
				places = iowrappers.Filter(places, func(place POI.Place) bool { return !place.KnownClosedOnDay(day) })
				// Only the Redis path returns places in distance order. The fresh path
				// appends each place type's results in Google prominence order, so sort
				// before truncating or the last place types get dropped wholesale.
				iowrappers.SortPlacesByDistance(places, req.Location.Latitude, req.Location.Longitude)
				if len(places) > limit {
					places = places[:limit]
				}
				result.Places = places
```

- [ ] **Step 6: Run the full suite**

Run: `go build -v . && go test ./... 2>&1 | tail -20`

Expected: PASS, no regressions.

- [ ] **Step 7: Commit**

```bash
git add iowrappers/place_distance_sort.go iowrappers/place_distance_sort_test.go planner/planner.go
git commit -m "fix: sort category results by distance before truncating

The truncation at places[:limit] assumed distance ordering, which only
holds on the Redis cache path. The fresh path appends each place type's
results in Google prominence order, so a cold search kept roughly cafes
and restaurants and dropped bar, bakery and meal_takeaway entirely, and
could rank a 3km result above one 250m away."
```

---

### Task 4: Admin migration to purge misclassified places from category buckets

Cleans the records already in prod. Must ship **after** Tasks 1-2 are deployed — otherwise the next cold search in any city recreates them. `MinMapsResultRefreshDuration` is 14 days (`iowrappers/poi_searcher.go`), so Los Altos is quiet, but every other city repopulates on its next search.

Reuses the established pattern: an admin-authenticated GET under `v1.Group("/migrate")` (`planner/planner.go:1680-1684`), alongside `RemovePlaces`.

**Files:**
- Modify: `iowrappers/data_migrations.go`
- Modify: `iowrappers/maps_client.go` (the searcher interface the handler calls through)
- Modify: `planner/planner.go` (handler + route)
- Test: `test/redis_client_mocks/bucket_cleanup_test.go` (create)

**Interfaces:**
- Consumes: `POI.GetPlaceCategory(placeType) (PlaceCategory, bool)` from Task 1; `POI.PrimaryLocationType`, `POI.GetPlaceTypes` (existing).
- Produces: `(*RedisClient).RemoveMisclassifiedPlacesFromCategoryBuckets(ctx context.Context, cat POI.PlaceCategory, dryRun bool) (BucketCleanupReport, error)` and `type BucketCleanupReport struct { Scanned, Misclassified, Removed int; RemovedIDs []string }`.

- [ ] **Step 1: Write the failing test**

Create `test/redis_client_mocks/bucket_cleanup_test.go`. This follows the existing miniredis harness in that package (`RedisClient`, `RedisContext`, `RedisMockSvr` are package-level fixtures set up by its `TestMain`):

```go
package redis_client_mocks

import (
	"testing"

	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
)

// TestRemoveMisclassifiedPlacesDryRun pins that a dry run reports the hotels that the
// fast_food_restaurant incident wrote into placeIDs:eatery:level* without deleting them.
func TestRemoveMisclassifiedPlacesDryRun(t *testing.T) {
	hotel := newPlaceWithTypes("hotel-1", "Residence Inn by Marriott Palo Alto",
		POI.LocationType("fast_food_restaurant"), []string{"lodging", "point_of_interest", "establishment"})
	cafe := newPlaceWithTypes("cafe-1", "Peet's Coffee",
		POI.LocationTypeCafe, []string{"cafe", "food", "point_of_interest", "establishment"})
	RedisClient.SetPlacesAddGeoLocations(RedisContext, []POI.Place{cafe})
	seedGeoBucket(t, POI.PlaceCategoryEatery, hotel)

	report, err := RedisClient.RemoveMisclassifiedPlacesFromCategoryBuckets(RedisContext, POI.PlaceCategoryEatery, true)
	if err != nil {
		t.Fatalf("RemoveMisclassifiedPlacesFromCategoryBuckets error: %v", err)
	}
	if report.Misclassified != 1 {
		t.Errorf("Misclassified = %d, want 1 (report: %+v)", report.Misclassified, report)
	}
	if report.Removed != 0 {
		t.Errorf("dry run Removed = %d, want 0", report.Removed)
	}
	if len(report.RemovedIDs) != 1 || report.RemovedIDs[0] != "hotel-1" {
		t.Errorf("RemovedIDs = %v, want [hotel-1]", report.RemovedIDs)
	}
	// the hotel must still be present after a dry run
	if got := countInEateryBuckets(t, "hotel-1"); got == 0 {
		t.Error("dry run deleted hotel-1, want it retained")
	}
}

// TestRemoveMisclassifiedPlacesApply pins that a real run removes only the hotel.
func TestRemoveMisclassifiedPlacesApply(t *testing.T) {
	RedisMockSvr.FlushAll()

	hotel := newPlaceWithTypes("hotel-2", "The Westin Palo Alto",
		POI.LocationType("fast_food_restaurant"), []string{"lodging", "point_of_interest", "establishment"})
	cafe := newPlaceWithTypes("cafe-2", "Red Rock Coffee",
		POI.LocationTypeCafe, []string{"cafe", "food", "point_of_interest", "establishment"})
	RedisClient.SetPlacesAddGeoLocations(RedisContext, []POI.Place{cafe})
	seedGeoBucket(t, POI.PlaceCategoryEatery, hotel)

	report, err := RedisClient.RemoveMisclassifiedPlacesFromCategoryBuckets(RedisContext, POI.PlaceCategoryEatery, false)
	if err != nil {
		t.Fatalf("RemoveMisclassifiedPlacesFromCategoryBuckets error: %v", err)
	}
	if report.Removed != 1 {
		t.Errorf("Removed = %d, want 1 (report: %+v)", report.Removed, report)
	}
	if got := countInEateryBuckets(t, "hotel-2"); got != 0 {
		t.Errorf("hotel-2 still in %d eatery buckets, want 0", got)
	}
	if got := countInEateryBuckets(t, "cafe-2"); got == 0 {
		t.Error("cafe-2 was removed, want it retained")
	}
}

// TestRemoveMisclassifiedPlacesKeepsUntypedRecords pins that older cached records with
// no Types list are left alone, matching ReclassifyForCategory's keep-on-unknown rule.
func TestRemoveMisclassifiedPlacesKeepsUntypedRecords(t *testing.T) {
	RedisMockSvr.FlushAll()

	legacy := newPlaceWithTypes("legacy-1", "Old Cached Diner", POI.LocationTypeRestaurant, nil)
	RedisClient.SetPlacesAddGeoLocations(RedisContext, []POI.Place{legacy})

	report, err := RedisClient.RemoveMisclassifiedPlacesFromCategoryBuckets(RedisContext, POI.PlaceCategoryEatery, false)
	if err != nil {
		t.Fatalf("RemoveMisclassifiedPlacesFromCategoryBuckets error: %v", err)
	}
	if report.Removed != 0 {
		t.Errorf("Removed = %d, want 0 — records without Types must be kept", report.Removed)
	}
}

func newPlaceWithTypes(id, name string, locationType POI.LocationType, types []string) POI.Place {
	var p POI.Place
	p.SetID(id)
	p.SetName(name)
	p.SetType(locationType)
	p.SetStatus(string(POI.Operational))
	p.SetPriceLevel(POI.PriceLevelDefault)
	p.SetUserRatingsTotal(100)
	p.SetLocationCoordinates([2]float64{37.38006, -122.11612})
	p.Types = types
	return p
}

// seedGeoBucket writes a place record plus its eatery geo-bucket membership directly,
// bypassing SetPlacesAddGeoLocations, which after Task 1 refuses unmapped types.
func seedGeoBucket(t *testing.T, cat POI.PlaceCategory, place POI.Place) {
	t.Helper()
	if err := RedisClient.SetPlace(RedisContext, place); err != nil {
		t.Fatalf("SetPlace(%s): %v", place.GetID(), err)
	}
	key := POI.EncodeNearbySearchRedisKey(cat, place.PriceLevel)
	if err := RedisClient.AddGeoLocation(RedisContext, key, place); err != nil {
		t.Fatalf("AddGeoLocation(%s): %v", key, err)
	}
}

func countInEateryBuckets(t *testing.T, placeID string) int {
	t.Helper()
	count := 0
	for _, lvl := range POI.AllPriceLevels {
		key := POI.EncodeNearbySearchRedisKey(POI.PlaceCategoryEatery, lvl)
		if RedisMockSvr.Exists(key) {
			members, err := RedisMockSvr.ZMembers(key)
			if err != nil {
				continue
			}
			for _, m := range members {
				if m == placeID {
					count++
				}
			}
		}
	}
	return count
}
```

Note for the implementer: this test needs two small exported helpers on `RedisClient` that do not exist yet — `SetPlace(ctx, place) error` and `AddGeoLocation(ctx, key string, place POI.Place) error`. `setPlace` already exists unexported (`iowrappers/redis_client.go`, used by `StorePlacesForLocation`); add thin exported wrappers in Step 3 rather than duplicating logic.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./test/redis_client_mocks/ -run TestRemoveMisclassifiedPlaces -v`

Expected: FAIL to compile with `RedisClient.RemoveMisclassifiedPlacesFromCategoryBuckets undefined` (and undefined `SetPlace` / `AddGeoLocation`).

- [ ] **Step 3: Write the implementation**

Append to `iowrappers/data_migrations.go`:

```go
// BucketCleanupReport summarizes a RemoveMisclassifiedPlacesFromCategoryBuckets run.
type BucketCleanupReport struct {
	Scanned       int      `json:"scanned"`
	Misclassified int      `json:"misclassified"`
	Removed       int      `json:"removed"`
	RemovedIDs    []string `json:"removed_ids"`
}

// SetPlace stores a single place record. Exported wrapper over setPlace for migrations and tests.
func (r *RedisClient) SetPlace(ctx context.Context, place POI.Place) error {
	return r.setPlace(ctx, place)
}

// AddGeoLocation adds a place to a geo bucket under an explicit key. Exported for
// migrations and tests that need to write buckets the normal write path would reject.
func (r *RedisClient) AddGeoLocation(ctx context.Context, key string, place POI.Place) error {
	loc := place.GetLocation()
	_, err := r.client.GeoAdd(ctx, key, &redis.GeoLocation{
		Name:      place.ID,
		Latitude:  loc.Latitude,
		Longitude: loc.Longitude,
	}).Result()
	return err
}

// RemoveMisclassifiedPlacesFromCategoryBuckets removes places from cat's geo buckets whose
// PRIMARY Google type does not belong to cat. It repairs the fast_food_restaurant incident:
// two Places-API-(New)-only types were searched against the legacy Nearby Search, which
// ignored the unenforceable type filter and returned prominence-ranked establishments, and
// those were stamped with the queried type and written into placeIDs:eatery:level*.
//
// It uses the same rule as POI.ReclassifyForCategory, which is what already hides these from
// API responses: classify by primary type, and KEEP records with no Types list (older cached
// records written before Types was captured) so coverage never regresses.
//
// dryRun reports what would be removed without deleting anything. Always dry-run first.
func (r *RedisClient) RemoveMisclassifiedPlacesFromCategoryBuckets(ctx context.Context, cat POI.PlaceCategory, dryRun bool) (BucketCleanupReport, error) {
	report := BucketCleanupReport{RemovedIDs: make([]string, 0)}

	levels := []POI.PriceLevel{POI.PriceLevelDefault}
	if cat == POI.PlaceCategoryEatery {
		levels = POI.AllPriceLevels
	}

	for _, level := range levels {
		key := POI.EncodeNearbySearchRedisKey(cat, level)
		members, err := r.client.ZRange(ctx, key, 0, -1).Result()
		if err != nil {
			return report, fmt.Errorf("reading geo bucket %s: %w", key, err)
		}
		for _, placeID := range members {
			report.Scanned++
			place, err := r.getPlace(ctx, placeID)
			if err != nil {
				// no place record backing this bucket member; leave it for RemovePlaces
				Logger.Debugf("RemoveMisclassifiedPlacesFromCategoryBuckets: no record for %s in %s", placeID, key)
				continue
			}
			if _, keep := POI.ReclassifyForCategory(place, cat); keep {
				continue
			}
			report.Misclassified++
			report.RemovedIDs = append(report.RemovedIDs, placeID)
			Logger.Infof("RemoveMisclassifiedPlacesFromCategoryBuckets: %s (%q, LocationType=%q, Types=%v) does not belong in %s",
				placeID, place.Name, place.LocationType, place.Types, key)
			if dryRun {
				continue
			}
			if _, err := r.client.ZRem(ctx, key, placeID).Result(); err != nil {
				return report, fmt.Errorf("removing %s from %s: %w", placeID, key, err)
			}
			report.Removed++
		}
	}
	return report, nil
}
```

Add `"github.com/redis/go-redis/v9"` to the imports of `data_migrations.go` if it is not already present, alongside the existing `context`, `fmt`, and `POI` imports.

Add the method to the migration-capable interface in `iowrappers/maps_client.go` so the Gin handler can call it through `p.Solver.Searcher`. Locate the interface that already declares `RemovePlaces` and add:

```go
	RemoveMisclassifiedPlacesFromCategoryBuckets(context.Context, POI.PlaceCategory, bool) (BucketCleanupReport, error)
```

Then add the forwarding method on `PoiSearcher` in `data_migrations.go`, mirroring the existing `(*PoiSearcher).RemovePlaces`:

```go
func (s *PoiSearcher) RemoveMisclassifiedPlacesFromCategoryBuckets(ctx context.Context, cat POI.PlaceCategory, dryRun bool) (BucketCleanupReport, error) {
	return s.redisClient.RemoveMisclassifiedPlacesFromCategoryBuckets(ctx, cat, dryRun)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go build -v . && go test ./test/redis_client_mocks/ -run TestRemoveMisclassifiedPlaces -v`

Expected: PASS on all three tests.

- [ ] **Step 5: Add the admin handler and route**

Add to `planner/planner.go`, next to `removePlacesMigrationHandler`:

```go
// reclassifyBucketsMigrationHandler removes places from a category's geo buckets whose
// primary Google type does not belong to that category. Dry-run unless ?apply=true.
//
// Usage: GET /v1/migrate/reclassify-buckets?category=Eatery
//        GET /v1/migrate/reclassify-buckets?category=Eatery&apply=true
func (p *MyPlanner) reclassifyBucketsMigrationHandler(ctx *gin.Context) {
	_, authenticationErr := p.UserAuthentication(ctx, user.LevelAdmin)
	if authenticationErr != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": authenticationErr.Error()})
		return
	}
	category, ok := POI.ParsePlaceCategory(ctx.DefaultQuery("category", string(POI.PlaceCategoryEatery)))
	if !ok {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "unknown category"})
		return
	}
	dryRun := ctx.Query("apply") != "true"
	report, err := p.Solver.Searcher.RemoveMisclassifiedPlacesFromCategoryBuckets(ctx.Request.Context(), category, dryRun)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "partial_report": report})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"dry_run": dryRun, "category": category, "report": report})
}
```

Register it at `planner/planner.go:1684`, inside the existing `migrations` group:

```go
			migrations.GET("/reclassify-buckets", p.reclassifyBucketsMigrationHandler)
```

- [ ] **Step 6: Run the full suite**

Run: `go build -v . && go test ./... 2>&1 | tail -20`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add iowrappers/data_migrations.go iowrappers/maps_client.go planner/planner.go test/redis_client_mocks/bucket_cleanup_test.go
git commit -m "feat: add admin migration to purge misclassified places from geo buckets

The fast_food_restaurant incident wrote prominence-ranked hotels into
placeIDs:eatery:level*. ReclassifyForCategory already hides them from API
responses, but they inflate the bucket counts that gate radius expansion.
Remove them using the same primary-type rule, dry-run by default."
```

---

## Deployment order

1. Merge and deploy Tasks 1-3. Verify `go test ./...` green in CI.
2. Dry-run the cleanup and read the report before applying:
   ```bash
   curl -s -H "Authorization: Bearer $ADMIN_JWT" \
     "https://best-vacation-planner.herokuapp.com/v1/migrate/reclassify-buckets?category=Eatery" | jq
   ```
   Expect roughly 17 entries in `removed_ids` for the Los Altos hotels, plus any older misclassifications the primary-type rule catches. Review the list before proceeding.
3. Apply:
   ```bash
   curl -s -H "Authorization: Bearer $ADMIN_JWT" \
     "https://best-vacation-planner.herokuapp.com/v1/migrate/reclassify-buckets?category=Eatery&apply=true" | jq
   ```
4. Spot-check that a cold search is correct now. `MapsLastSearchTime` gates on a 14-day TTL, so force a fresh path by deleting the marker field for the city under test:
   ```bash
   # field format: "<country>:<admin area 1>:<city>:<category>:<price level>"
   redis-cli HDEL MapsLastSearchTime "united states:ca:los altos:eatery:0"
   ```
   Then re-run the category search and confirm the nearest result is the nearest by distance, not by Google prominence.

## Verification checklist

- [ ] `go build -v .` and `go test -v ./...` pass locally and in CI.
- [ ] `TestPlaceCategoryRoundTrip` fails if you temporarily re-add `LocationTypeFastFood` to `GetPlaceTypes(Eatery)` — confirm the guard is now real, then revert the experiment.
- [ ] `grep -rn 'fast_food_restaurant\|food_court' --include='*.go' .` returns nothing.
- [ ] A category search at State Street Market returns Peet's at 367 State St ahead of results in Sunnyvale.
- [ ] Dry-run report reviewed before any `apply=true` call.

## Out of scope, tracked in the follow-up plan

Correctly classifying fast food and food courts needs Places API (New) `searchNearby` with `includedPrimaryTypes`. Legacy `types[]` never contains those values, so `POI.PrimaryLocationType` can never return them either — no amount of client-side work fixes it on the current API. See `2026-07-29-places-api-new-migration.md`.
