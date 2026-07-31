package iowrappers

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/weihesdlegend/Vacation-planner/POI"
	"googlemaps.github.io/maps"
)

// newAddSearchedPlaceFixture builds a PoiSearcher backed by its own miniredis instance, mirroring
// data_migrations_test.go's own-server-per-test harness. mapsClient is left nil: the unexported
// addSearchedPlaceToCache never touches it (that is the entire point of the placeDetailsEnricher
// seam), only the exported AddSearchedPlaceToCache does.
func newAddSearchedPlaceFixture(t *testing.T) (*PoiSearcher, context.Context) {
	t.Helper()
	redisMockSvr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	t.Cleanup(redisMockSvr.Close)

	redisURL, _ := url.Parse("redis://" + redisMockSvr.Addr())
	redisClient := CreateRedisClient(redisURL)
	if err := CreateLogger(); err != nil {
		t.Fatalf("CreateLogger: %v", err)
	}

	return &PoiSearcher{redisClient: redisClient}, context.Background()
}

// stashCandidate writes a search candidate directly, standing in for what
// PoiSearcher.TextSearchPlaces would have stashed.
func stashCandidate(t *testing.T, s *PoiSearcher, ctx context.Context, place POI.Place) {
	t.Helper()
	if err := s.redisClient.SetPlaceSearchCandidate(ctx, place, PlaceSearchCandidateTTL); err != nil {
		t.Fatalf("SetPlaceSearchCandidate: %v", err)
	}
}

func succeedingEnricher(details maps.PlaceDetailsResult) placeDetailsEnricher {
	return func(ctx context.Context, placeID string) (maps.PlaceDetailsResult, error) {
		return details, nil
	}
}

func failingEnricher(err error) placeDetailsEnricher {
	return func(ctx context.Context, placeID string) (maps.PlaceDetailsResult, error) {
		return maps.PlaceDetailsResult{}, err
	}
}

// assertNothingWrittenForPlace checks that neither a place_details record nor any category geo
// bucket membership exists for placeID — the guarantee the refusal path must uphold.
func assertNothingWrittenForPlace(t *testing.T, s *PoiSearcher, ctx context.Context, placeID string) {
	t.Helper()
	cached, err := s.redisClient.CachedPlaces(ctx, []string{placeID})
	if err != nil {
		t.Fatalf("CachedPlaces: %v", err)
	}
	if len(cached) != 0 {
		t.Errorf("place_details record exists for %s, want none written on refusal", placeID)
	}

	for _, cat := range POI.AllPlaceCategories {
		key := POI.EncodeNearbySearchRedisKey(cat)
		score, err := s.redisClient.Get().ZScore(ctx, key, placeID).Result()
		if err == nil {
			t.Errorf("place %s found in bucket %s with score %v, want absent", placeID, key, score)
		}
	}
}

// TestAddSearchedPlaceToCache_UnmappedPrimaryRefusesAndWritesNothing pins the refusal path: a
// candidate whose primary Google type has no POI.PlaceCategory mapping must be rejected with
// ErrUnsupportedPlaceType, and the refusal must be total — no place_details record, no bucket
// membership, before the enricher is even asked to run.
func TestAddSearchedPlaceToCache_UnmappedPrimaryRefusesAndWritesNothing(t *testing.T) {
	s, ctx := newAddSearchedPlaceFixture(t)

	placeID := "unmapped-1"
	candidate := POI.Place{
		ID:    placeID,
		Name:  "Springfield Elementary",
		Types: []string{"school", "point_of_interest", "establishment"}, // "school" is not in placeTypeToCategory
	}
	stashCandidate(t, s, ctx, candidate)

	enricherCalled := false
	enrich := func(ctx context.Context, placeID string) (maps.PlaceDetailsResult, error) {
		enricherCalled = true
		return maps.PlaceDetailsResult{}, nil
	}

	_, err := s.addSearchedPlaceToCache(ctx, placeID, enrich)
	if err == nil {
		t.Fatal("expected an error for an unmapped primary type, got nil")
	}
	if !errors.Is(err, ErrUnsupportedPlaceType) {
		t.Errorf("error %v does not wrap ErrUnsupportedPlaceType", err)
	}
	if enricherCalled {
		t.Error("enricher was called on the refusal path; it must short-circuit before any enrichment or write")
	}

	assertNothingWrittenForPlace(t, s, ctx, placeID)
}

// TestAddSearchedPlaceToCache_MappedPrimarySucceeds pins the happy path: a mapped primary type
// lands the place in its category's geo bucket and place_details record, tagged with the primary
// type.
func TestAddSearchedPlaceToCache_MappedPrimarySucceeds(t *testing.T) {
	s, ctx := newAddSearchedPlaceFixture(t)

	placeID := "museum-1"
	candidate := POI.Place{
		ID:           placeID,
		Name:         "City History Museum",
		LocationType: POI.LocationType("tourist_attraction"), // deliberately NOT the primary type
		Types:        []string{"museum", "point_of_interest", "establishment"},
		Location:     POI.Location{Latitude: 37.4, Longitude: -122.1},
	}
	stashCandidate(t, s, ctx, candidate)

	result, err := s.addSearchedPlaceToCache(ctx, placeID, failingEnricher(errors.New("no network in this test")))
	if err != nil {
		t.Fatalf("addSearchedPlaceToCache: %v", err)
	}

	if result.Category != POI.PlaceCategoryVisit {
		t.Errorf("Category = %q, want %q", result.Category, POI.PlaceCategoryVisit)
	}
	if result.AlreadyCached {
		t.Error("AlreadyCached = true on a first confirm, want false")
	}
	if result.Place.LocationType != POI.LocationType("museum") {
		t.Errorf("LocationType = %q, want the primary type %q", result.Place.LocationType, "museum")
	}

	key := POI.EncodeNearbySearchRedisKey(POI.PlaceCategoryVisit)
	if _, err := s.redisClient.Get().ZScore(ctx, key, placeID).Result(); err != nil {
		t.Errorf("expected %s to be a member of %s, ZScore error: %v", placeID, key, err)
	}

	cached, err := s.redisClient.CachedPlaces(ctx, []string{placeID})
	if err != nil {
		t.Fatalf("CachedPlaces: %v", err)
	}
	if _, ok := cached[placeID]; !ok {
		t.Errorf("place_details record missing for %s after a successful confirm", placeID)
	}
}

// TestAddSearchedPlaceToCache_CrossCategoryLandsInWellnessNotEatery pins that a pharmacy-primary
// candidate is filed under placeIDs:wellness and never under placeIDs:eatery.
func TestAddSearchedPlaceToCache_CrossCategoryLandsInWellnessNotEatery(t *testing.T) {
	s, ctx := newAddSearchedPlaceFixture(t)

	placeID := "pharmacy-1"
	candidate := POI.Place{
		ID:       placeID,
		Name:     "Corner Drugstore",
		Types:    []string{"pharmacy", "point_of_interest", "establishment"},
		Location: POI.Location{Latitude: 37.4, Longitude: -122.1},
	}
	stashCandidate(t, s, ctx, candidate)

	result, err := s.addSearchedPlaceToCache(ctx, placeID, failingEnricher(errors.New("no network in this test")))
	if err != nil {
		t.Fatalf("addSearchedPlaceToCache: %v", err)
	}
	if result.Category != POI.PlaceCategoryWellness {
		t.Fatalf("Category = %q, want %q", result.Category, POI.PlaceCategoryWellness)
	}

	wellnessKey := POI.EncodeNearbySearchRedisKey(POI.PlaceCategoryWellness)
	if _, err := s.redisClient.Get().ZScore(ctx, wellnessKey, placeID).Result(); err != nil {
		t.Errorf("expected %s in %s, ZScore error: %v", placeID, wellnessKey, err)
	}

	eateryKey := POI.EncodeNearbySearchRedisKey(POI.PlaceCategoryEatery)
	if _, err := s.redisClient.Get().ZScore(ctx, eateryKey, placeID).Result(); err == nil {
		t.Errorf("place %s unexpectedly present in %s", placeID, eateryKey)
	}
}

// TestAddSearchedPlaceToCache_AlreadyCachedRealHoursPreserved pins restoreCachedDetails' role in
// this flow: confirming a place that is already cached with real opening hours must never
// overwrite those hours with the lean (default-hours) stashed candidate, even when the Details
// enrich call fails.
func TestAddSearchedPlaceToCache_AlreadyCachedRealHoursPreserved(t *testing.T) {
	s, ctx := newAddSearchedPlaceFixture(t)

	placeID := "museum-hours-1"
	existing := POI.Place{
		ID:           placeID,
		Name:         "City History Museum",
		LocationType: POI.LocationType("museum"),
		Types:        []string{"museum", "point_of_interest", "establishment"},
		Location:     POI.Location{Latitude: 37.4, Longitude: -122.1},
		Hours: [7]string{
			"9:00 am – 5:00 pm", "9:00 am – 5:00 pm", "9:00 am – 5:00 pm", "9:00 am – 5:00 pm",
			"9:00 am – 5:00 pm", "9:00 am – 5:00 pm", "Closed",
		},
	}
	if !existing.HasRealOpeningHours() {
		t.Fatal("fixture setup bug: existing.HasRealOpeningHours() should be true")
	}
	s.redisClient.SetPlacesAddGeoLocations(ctx, []POI.Place{existing})

	// The stashed candidate is lean: zero-value Hours, which is not "real" per HasRealOpeningHours
	// (empty strings do not count).
	candidate := POI.Place{
		ID:       placeID,
		Name:     "City History Museum",
		Types:    []string{"museum", "point_of_interest", "establishment"},
		Location: POI.Location{Latitude: 37.4, Longitude: -122.1},
	}
	stashCandidate(t, s, ctx, candidate)

	result, err := s.addSearchedPlaceToCache(ctx, placeID, failingEnricher(errors.New("enrich down")))
	if err != nil {
		t.Fatalf("addSearchedPlaceToCache: %v", err)
	}
	if !result.AlreadyCached {
		t.Error("AlreadyCached = false, want true (the place was cached before this confirm)")
	}
	if !result.Place.HasRealOpeningHours() {
		t.Fatal("result.Place lost its real opening hours")
	}

	cached, err := s.redisClient.CachedPlaces(ctx, []string{placeID})
	if err != nil {
		t.Fatalf("CachedPlaces: %v", err)
	}
	stored, ok := cached[placeID]
	if !ok {
		t.Fatal("place_details record missing after confirm")
	}
	if !stored.HasRealOpeningHours() {
		t.Error("stored record's real opening hours were clobbered by the lean confirm")
	}
	if stored.Hours != existing.Hours {
		t.Errorf("stored hours = %+v, want unchanged %+v", stored.Hours, existing.Hours)
	}
}

// TestAddSearchedPlaceToCache_ConfirmTwiceIsIdempotent pins that confirming the same place twice
// leaves a single geo-bucket member and flips AlreadyCached from false to true.
func TestAddSearchedPlaceToCache_ConfirmTwiceIsIdempotent(t *testing.T) {
	s, ctx := newAddSearchedPlaceFixture(t)

	placeID := "museum-twice-1"
	candidate := POI.Place{
		ID:       placeID,
		Name:     "City History Museum",
		Types:    []string{"museum", "point_of_interest", "establishment"},
		Location: POI.Location{Latitude: 37.4, Longitude: -122.1},
	}
	stashCandidate(t, s, ctx, candidate)

	first, err := s.addSearchedPlaceToCache(ctx, placeID, failingEnricher(errors.New("down")))
	if err != nil {
		t.Fatalf("first confirm: %v", err)
	}
	if first.AlreadyCached {
		t.Error("first confirm: AlreadyCached = true, want false")
	}

	// Re-stash: the candidate stash is what a second TextSearchPlaces call (or the same result
	// still within TTL) would have left behind.
	stashCandidate(t, s, ctx, candidate)

	second, err := s.addSearchedPlaceToCache(ctx, placeID, failingEnricher(errors.New("down")))
	if err != nil {
		t.Fatalf("second confirm: %v", err)
	}
	if !second.AlreadyCached {
		t.Error("second confirm: AlreadyCached = false, want true")
	}

	key := POI.EncodeNearbySearchRedisKey(POI.PlaceCategoryVisit)
	members, err := s.redisClient.Get().ZRange(ctx, key, 0, -1).Result()
	if err != nil {
		t.Fatalf("ZRange: %v", err)
	}
	count := 0
	for _, m := range members {
		if m == placeID {
			count++
		}
	}
	if count != 1 {
		t.Errorf("place %s appears %d times in %s, want exactly 1", placeID, count, key)
	}
}

// TestAddSearchedPlaceToCache_MissingCandidateReturnsNotFound pins that confirming a place ID that
// was never searched (or whose stash entry expired) fails with ErrSearchCandidateNotFound.
func TestAddSearchedPlaceToCache_MissingCandidateReturnsNotFound(t *testing.T) {
	s, ctx := newAddSearchedPlaceFixture(t)

	_, err := s.addSearchedPlaceToCache(ctx, "never-searched", failingEnricher(errors.New("should not be called")))
	if err == nil {
		t.Fatal("expected an error for a missing candidate, got nil")
	}
	if !errors.Is(err, ErrSearchCandidateNotFound) {
		t.Errorf("error %v does not wrap ErrSearchCandidateNotFound", err)
	}
}

// TestAddSearchedPlaceToCache_EnricherErrorStillCaches pins that a Details enrich failure is
// best-effort: the place is still confirmed into the cache using the lean stashed record.
func TestAddSearchedPlaceToCache_EnricherErrorStillCaches(t *testing.T) {
	s, ctx := newAddSearchedPlaceFixture(t)

	placeID := "museum-enrich-fail-1"
	candidate := POI.Place{
		ID:       placeID,
		Name:     "City History Museum",
		Types:    []string{"museum", "point_of_interest", "establishment"},
		Location: POI.Location{Latitude: 37.4, Longitude: -122.1},
	}
	stashCandidate(t, s, ctx, candidate)

	result, err := s.addSearchedPlaceToCache(ctx, placeID, failingEnricher(errors.New("Google is down")))
	if err != nil {
		t.Fatalf("addSearchedPlaceToCache should succeed despite an enrich failure, got: %v", err)
	}
	if result.Place.ID != placeID {
		t.Errorf("result.Place.ID = %q, want %q", result.Place.ID, placeID)
	}

	cached, err := s.redisClient.CachedPlaces(ctx, []string{placeID})
	if err != nil {
		t.Fatalf("CachedPlaces: %v", err)
	}
	if _, ok := cached[placeID]; !ok {
		t.Error("place was not cached despite the enrich failure being best-effort")
	}
}

// TestAddSearchedPlaceToCache_EnricherSuccessFoldsDetails pins that a successful enrich call folds
// weekday hours, formatted address, URL and editorial summary onto the confirmed place.
func TestAddSearchedPlaceToCache_EnricherSuccessFoldsDetails(t *testing.T) {
	s, ctx := newAddSearchedPlaceFixture(t)

	placeID := "museum-enrich-ok-1"
	candidate := POI.Place{
		ID:       placeID,
		Name:     "City History Museum",
		Types:    []string{"museum", "point_of_interest", "establishment"},
		Location: POI.Location{Latitude: 37.4, Longitude: -122.1},
	}
	stashCandidate(t, s, ctx, candidate)

	weekdayText := []string{
		"Monday: 9:00 am – 5:00 pm", "Tuesday: 9:00 am – 5:00 pm", "Wednesday: 9:00 am – 5:00 pm",
		"Thursday: 9:00 am – 5:00 pm", "Friday: 9:00 am – 5:00 pm", "Saturday: 10:00 am – 4:00 pm",
		"Sunday: Closed",
	}
	details := maps.PlaceDetailsResult{
		OpeningHours:     &maps.OpeningHours{WeekdayText: weekdayText},
		FormattedAddress: "1 Museum Way, Springfield",
		AdrAddress:       `<span class="street-address">1 Museum Way</span>`,
		URL:              "https://maps.google.com/?cid=123",
		EditorialSummary: &maps.PlaceEditorialSummary{Overview: "A fine local museum."},
	}

	result, err := s.addSearchedPlaceToCache(ctx, placeID, succeedingEnricher(details))
	if err != nil {
		t.Fatalf("addSearchedPlaceToCache: %v", err)
	}
	if !result.Place.HasRealOpeningHours() {
		t.Error("enriched place should have real opening hours")
	}
	if result.Place.FormattedAddress != details.FormattedAddress {
		t.Errorf("FormattedAddress = %q, want %q", result.Place.FormattedAddress, details.FormattedAddress)
	}
	if result.Place.URL != details.URL {
		t.Errorf("URL = %q, want %q", result.Place.URL, details.URL)
	}
	if result.Place.Summary != details.EditorialSummary.Overview {
		t.Errorf("Summary = %q, want %q", result.Place.Summary, details.EditorialSummary.Overview)
	}
}

// TestAddSearchedPlaceToCache_AlreadyCachedPhotoPreserved pins the Finding-1 fix: restoreCachedDetails
// (iowrappers/nearby_search.go) restores URL/Summary/FormattedAddress/Address/Hours but NOT Photo,
// because it is shared with the nearby-search write path and this task must not touch it. Without a
// local gap-fill, a lean re-confirm of an already-cached place (e.g. a text search result that
// carried no photos, combined with a failed Details enrich) would silently overwrite a real
// Photo.Reference with the zero value while still reporting success.
func TestAddSearchedPlaceToCache_AlreadyCachedPhotoPreserved(t *testing.T) {
	s, ctx := newAddSearchedPlaceFixture(t)

	placeID := "museum-photo-1"
	existing := POI.Place{
		ID:           placeID,
		Name:         "City History Museum",
		LocationType: POI.LocationType("museum"),
		Types:        []string{"museum", "point_of_interest", "establishment"},
		Location:     POI.Location{Latitude: 37.4, Longitude: -122.1},
		Photo:        POI.PlacePhoto{Reference: "existing-photo-ref", Height: 400, Width: 600},
	}
	s.redisClient.SetPlacesAddGeoLocations(ctx, []POI.Place{existing})

	// The stashed candidate is lean: no photo, as a text-search result commonly has none.
	candidate := POI.Place{
		ID:       placeID,
		Name:     "City History Museum",
		Types:    []string{"museum", "point_of_interest", "establishment"},
		Location: POI.Location{Latitude: 37.4, Longitude: -122.1},
	}
	stashCandidate(t, s, ctx, candidate)

	// Enrich fails too (Google down), so nothing supplies a fresh photo either — the only source
	// of truth for a photo here is the previously-cached record.
	result, err := s.addSearchedPlaceToCache(ctx, placeID, failingEnricher(errors.New("enrich down")))
	if err != nil {
		t.Fatalf("addSearchedPlaceToCache: %v", err)
	}
	if result.Place.Photo != existing.Photo {
		t.Errorf("result.Place.Photo = %+v, want the preserved %+v", result.Place.Photo, existing.Photo)
	}

	cached, err := s.redisClient.CachedPlaces(ctx, []string{placeID})
	if err != nil {
		t.Fatalf("CachedPlaces: %v", err)
	}
	stored, ok := cached[placeID]
	if !ok {
		t.Fatal("place_details record missing after confirm")
	}
	if stored.Photo != existing.Photo {
		t.Errorf("stored.Photo = %+v, want the preserved %+v (a lean confirm silently clobbered it)", stored.Photo, existing.Photo)
	}
}

// TestAddSearchedPlaceToCache_EnricherPhotoFoldedWhenCandidateHasNone pins the second half of the
// Finding-1 fix: "photos" is already requested in config/config.yml's detailed_search_fields, so
// the confirm path already pays for it in the enrich call; a candidate with no photo of its own
// should pick up the Details photo rather than the call's cost being thrown away.
func TestAddSearchedPlaceToCache_EnricherPhotoFoldedWhenCandidateHasNone(t *testing.T) {
	s, ctx := newAddSearchedPlaceFixture(t)

	placeID := "museum-photo-2"
	candidate := POI.Place{
		ID:       placeID,
		Name:     "City History Museum",
		Types:    []string{"museum", "point_of_interest", "establishment"},
		Location: POI.Location{Latitude: 37.4, Longitude: -122.1},
	}
	stashCandidate(t, s, ctx, candidate)

	details := maps.PlaceDetailsResult{
		Photos: []maps.Photo{{PhotoReference: "fresh-photo-ref", Height: 300, Width: 500}},
	}

	result, err := s.addSearchedPlaceToCache(ctx, placeID, succeedingEnricher(details))
	if err != nil {
		t.Fatalf("addSearchedPlaceToCache: %v", err)
	}
	want := POI.PlacePhoto{Reference: "fresh-photo-ref", Height: 300, Width: 500}
	if result.Place.Photo != want {
		t.Errorf("result.Place.Photo = %+v, want %+v (Details photo should be folded in when the candidate had none)", result.Place.Photo, want)
	}
}

// TestNewPlaceDetailsEnricher_BoundsContextWithDeadline pins the Finding-2 fix: the real Google
// Place Details call must be bounded by GoogleMapsSearchTimeout before the semaphore is acquired.
// The maps SDK's HTTP client has no timeout of its own and an inbound request context carries no
// deadline by default, so without this bound a single hung call would park a process-wide
// apiSemaphore slot indefinitely. Uses a stub search func rather than a real client or a hang
// harness — CreatePoiSearcher always builds a real maps.Client even with a fake key, so there is
// no way to observe this via the real enricher without either a live network call or an actual
// hang.
func TestNewPlaceDetailsEnricher_BoundsContextWithDeadline(t *testing.T) {
	sem := make(chan struct{}, 1)
	var sawDeadline bool
	var sawWithin time.Duration
	stub := func(ctx context.Context, placeID string, fields []string) (maps.PlaceDetailsResult, error) {
		deadline, ok := ctx.Deadline()
		sawDeadline = ok
		if ok {
			sawWithin = time.Until(deadline)
		}
		return maps.PlaceDetailsResult{}, nil
	}

	enrich := newPlaceDetailsEnricher(sem, []string{"name"}, stub)
	if _, err := enrich(context.Background(), "some-id"); err != nil {
		t.Fatalf("enrich: %v", err)
	}

	if !sawDeadline {
		t.Fatal("search func's ctx had no deadline; a hung Google call could hold the semaphore slot forever")
	}
	if sawWithin <= 0 || sawWithin > GoogleMapsSearchTimeout {
		t.Errorf("ctx deadline is %v from now, want in (0, %v]", sawWithin, GoogleMapsSearchTimeout)
	}
}

// TestNewPlaceDetailsEnricher_AcquiresAndReleasesSemaphore pins that the semaphore is held for the
// duration of the search call and released afterward, preserving the existing rate-limiting
// behavior across the refactor that extracted newPlaceDetailsEnricher for testability.
func TestNewPlaceDetailsEnricher_AcquiresAndReleasesSemaphore(t *testing.T) {
	sem := make(chan struct{}, 1)
	var sawSemaphoreHeld bool
	stub := func(ctx context.Context, placeID string, fields []string) (maps.PlaceDetailsResult, error) {
		sawSemaphoreHeld = len(sem) == 1
		return maps.PlaceDetailsResult{}, nil
	}

	enrich := newPlaceDetailsEnricher(sem, nil, stub)
	if _, err := enrich(context.Background(), "some-id"); err != nil {
		t.Fatalf("enrich: %v", err)
	}

	if !sawSemaphoreHeld {
		t.Error("semaphore was not held while the search call was in flight")
	}
	if len(sem) != 0 {
		t.Errorf("semaphore not released after enrich returned, len(sem) = %d, want 0", len(sem))
	}
}
