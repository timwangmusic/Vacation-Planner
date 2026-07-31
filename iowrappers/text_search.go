package iowrappers

// Free-text place search: a user types a query ("Joe's Pizza"), Google Text Search returns
// candidates, and each candidate is stashed server-side under its place ID so a subsequent
// confirm (AddSearchedPlaceToCache) can look it up by ID alone rather than trusting anything an
// HTTP caller sends back. The category a candidate would land in is always derived server-side
// from POI.GetPlaceCategory(POI.PrimaryLocationType(...)) — never accepted from a caller — and an
// unmapped primary type refuses the write entirely rather than guessing a bucket.

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/weihesdlegend/Vacation-planner/POI"
	"googlemaps.github.io/maps"
)

const (
	// PlaceSearchCandidateRedisKeyPrefix stores the full POI.Place a text search returned, keyed
	// by place ID, so a later confirm can resolve it without a second Google call and without
	// trusting any place data the confirm request itself might carry.
	PlaceSearchCandidateRedisKeyPrefix = "place_search_candidate:place_ID:"
	// PlaceSearchCandidateTTL bounds how long a search result stays confirmable. Long enough for
	// a user to review results and pick one, short enough that stale candidates do not accumulate.
	PlaceSearchCandidateTTL = 30 * time.Minute
	// PlaceTextSearchMaxResults is one legacy Text Search page; the legacy API does not support
	// paging through more without a page token round trip, which this feature does not need.
	PlaceTextSearchMaxResults = 20
)

var (
	// ErrSearchCandidateNotFound means the place ID either was never searched or its stash entry
	// expired (see PlaceSearchCandidateTTL).
	ErrSearchCandidateNotFound = errors.New("place search candidate not found or expired")
	// ErrUnsupportedPlaceType means the candidate's primary Google type does not map to any
	// POI.PlaceCategory. The insert refuses entirely on this path: writing the place would force
	// a guess at which geo bucket it belongs in, which is exactly how the fast_food_restaurant
	// incident poisoned the eatery bucket with hotels (see POI/categories.go).
	ErrUnsupportedPlaceType = errors.New("place type does not map to a supported category")
)

// TextSearchRequest is the input to MapsClient.TextSearchPlaces / PoiSearcher.TextSearchPlaces.
type TextSearchRequest struct {
	Query    string
	Location POI.Location
	Radius   uint
	// Limit caps the number of results returned. <= 0 means PlaceTextSearchMaxResults.
	Limit int
}

// PlaceSearchCandidate is one text-search result, annotated with the category it would be filed
// under if confirmed. Category is "" and Insertable is false when the place's primary Google type
// is not one POI.GetPlaceCategory recognizes; such candidates are still returned and stashed so a
// confirm attempt gets an accurate 422 instead of a confusing 404 (see AddSearchedPlaceToCache).
type PlaceSearchCandidate struct {
	Place      POI.Place         `json:"place"`
	Category   POI.PlaceCategory `json:"category"`
	Insertable bool              `json:"insertable"`
}

// AddSearchedPlaceResult is the outcome of confirming a search candidate into the shared cache.
type AddSearchedPlaceResult struct {
	Place         POI.Place
	Category      POI.PlaceCategory
	AlreadyCached bool
}

// placeDetailsEnricher fetches Place Details for a place ID. It exists as an injectable seam so
// tests can confirm a candidate without a real Google call: CreatePoiSearcher always builds a
// real maps.Client, even when constructed with a fake API key, so there is no way to make a
// PoiSearcher's mapsClient a test double directly.
type placeDetailsEnricher func(ctx context.Context, placeID string) (maps.PlaceDetailsResult, error)

// TextSearchPlaces issues a Google Places Text Search and parses the response into POI.Place
// values. The request deliberately sets nothing beyond Query, Location and Radius: no Type, no
// OpenNow, no MinPrice/MaxPrice/Language/Region. A Type filter is especially dangerous here —
// Google silently ignores an unenforceable type on the legacy API rather than erroring, which is
// how hotels once got written into the eatery cache (see POI/categories.go's GetPlaceCategory
// doc). Text search has no type to filter on in the first place; the category is decided entirely
// after the fact, server-side, from what Google actually returns.
func (c *MapsClient) TextSearchPlaces(ctx context.Context, req *TextSearchRequest) ([]POI.Place, error) {
	ctx, cancel := context.WithTimeout(ctx, GoogleMapsSearchTimeout)
	defer cancel()

	mapsReq := &maps.TextSearchRequest{
		Query: req.Query,
		Location: &maps.LatLng{
			Lat: req.Location.Latitude,
			Lng: req.Location.Longitude,
		},
		Radius: req.Radius,
	}

	// Acquire semaphore for API rate limiting, mirroring Geocode/ReverseGeocode above.
	c.apiSemaphore <- struct{}{}
	defer func() { <-c.apiSemaphore }()

	resp, err := c.client.TextSearch(ctx, mapsReq)
	if err != nil {
		return nil, err
	}

	return parseTextSearchResponse(resp, req.Limit), nil
}

// parseTextSearchResponse converts a Text Search response into POI.Place values. Pure and
// side-effect free so the parsing rules can be pinned by table tests without a Redis or Google
// dependency.
//
// Deliberately does NOT filter UserRatingsTotal == 0, unlike parsePlacesSearchResponse (nearby
// search). That filter drops exactly the new-and-obscure places this feature exists to let a user
// add by name.
func parseTextSearchResponse(resp maps.PlacesSearchResponse, limit int) []POI.Place {
	effectiveLimit := PlaceTextSearchMaxResults
	if limit > 0 && limit < PlaceTextSearchMaxResults {
		effectiveLimit = limit
	}

	seen := make(map[string]bool, len(resp.Results))
	places := make([]POI.Place, 0, len(resp.Results))
	for _, res := range resp.Results {
		if len(places) >= effectiveLimit {
			break
		}
		if res.PlaceID == "" {
			continue
		}
		if res.Geometry.Location == (maps.LatLng{}) {
			continue
		}
		if seen[res.PlaceID] {
			continue
		}
		if res.BusinessStatus == string(POI.ClosedPermanently) {
			continue
		}
		seen[res.PlaceID] = true

		businessStatus := res.BusinessStatus
		if businessStatus == "" {
			// A blank status becomes StatusNotAvailable via Place.SetStatus, and the read path
			// (RedisClient.NearbySearch) filters to Operational-only, so a blank status would
			// make the place permanently invisible. A place the user just free-text-searched is
			// operational by observation, so treat a missing status as Operational rather than
			// as "unknown, hide it".
			Logger.Debugf("parseTextSearchResponse: place %s (%q) has no business_status from Google Text Search; treating as Operational", res.PlaceID, res.Name)
			businessStatus = string(POI.Operational)
		}

		locationType := POI.PrimaryLocationType(res.Types)

		var photo *maps.Photo
		if len(res.Photos) > 0 {
			photo = &res.Photos[0]
		}

		// No weekday hours exist in a Text Search response (openingHours is nil here), so the
		// resulting place always has CreatePlace's DefaultOpeningHours placeholder and
		// HasRealOpeningHours() is false. That is intentional: it is why confirming a candidate
		// buys a Place Details call (see AddSearchedPlaceToCache).
		place := POI.CreatePlace(res.Name, "", res.FormattedAddress, businessStatus, locationType, nil,
			res.PlaceID, res.PriceLevel, res.Rating, "", photo, res.UserRatingsTotal,
			res.Geometry.Location.Lat, res.Geometry.Location.Lng, nil)
		// Preserve Google's actual feature types so callers (and the confirm path) can classify
		// the place by its primary function rather than by the free-text query that found it.
		place.Types = res.Types
		places = append(places, place)
	}
	return places
}

// TextSearchPlaces runs a free-text Google search and stashes every result — insertable or not —
// as a confirmable candidate keyed by place ID. Non-insertable candidates are stashed too: doing
// so is cheap, and it lets a later confirm attempt on that ID report an accurate 422 (unsupported
// type) instead of a confusing 404 (candidate not found).
func (s *PoiSearcher) TextSearchPlaces(ctx context.Context, req *TextSearchRequest) ([]PlaceSearchCandidate, error) {
	places, err := s.mapsClient.TextSearchPlaces(ctx, req)
	if err != nil {
		return nil, err
	}

	candidates := make([]PlaceSearchCandidate, 0, len(places))
	for _, place := range places {
		// The category is always derived server-side from the place's own Types, never trusted
		// from place.LocationType (which parseTextSearchResponse also derives the same way, but
		// this recomputation keeps the rule in one place regardless of how the place arrived).
		primary := POI.PrimaryLocationType(place.Types)
		cat, ok := POI.GetPlaceCategory(primary)

		candidates = append(candidates, PlaceSearchCandidate{
			Place:      place,
			Category:   cat,
			Insertable: ok,
		})

		if stashErr := s.redisClient.SetPlaceSearchCandidate(ctx, place, PlaceSearchCandidateTTL); stashErr != nil {
			Logger.Errorf("TextSearchPlaces: failed to stash search candidate %s: %v", place.ID, stashErr)
		}
	}

	return candidates, nil
}

// AddSearchedPlaceToCache confirms a previously text-searched candidate into the shared Redis geo
// cache. See addSearchedPlaceToCache for the step-by-step behavior; this exported form supplies
// the real Google Place Details enricher.
func (s *PoiSearcher) AddSearchedPlaceToCache(ctx context.Context, placeID string) (AddSearchedPlaceResult, error) {
	enrich := func(ctx context.Context, placeID string) (maps.PlaceDetailsResult, error) {
		s.mapsClient.apiSemaphore <- struct{}{}
		defer func() { <-s.mapsClient.apiSemaphore }()
		return s.mapsClient.PlaceDetailedSearch(ctx, placeID, s.mapsClient.DetailedSearchFields)
	}
	return s.addSearchedPlaceToCache(ctx, placeID, enrich)
}

// addSearchedPlaceToCache does the real work, taking the Place Details lookup as a seam so tests
// can exercise it without a real Google client (see placeDetailsEnricher).
//
// Order of operations:
//  1. Resolve the stashed candidate by place ID; a miss wraps ErrSearchCandidateNotFound.
//  2. Derive the category from the candidate's own primary Google type. An unmapped type refuses
//     the whole operation before anything is written — this is the guarantee later tests assert:
//     no geo bucket member, no place_details record.
//  3. Read whether the place was already cached, BEFORE this confirm writes anything, so
//     AlreadyCached reflects prior state.
//  4. Best-effort enrich via Place Details for hours/address/URL/summary; a failure here is
//     logged and the lean (stash-only) record is used instead of failing the confirm.
//  5. Restore any Details-sourced fields (esp. real opening hours) a previously cached record had
//     that this pass didn't obtain, so confirming an already-cached place can never regress it to
//     placeholder data.
//  6. Tag the place with its true primary type and write it via the audited
//     SetPlacesAddGeoLocations, which is what actually buckets it under placeIDs:<category>.
//  7. SetPlacesAddGeoLocations only logs on failure, so read back via CachedPlaces and fail loudly
//     if the record did not land — otherwise a Redis failure would look like a success.
func (s *PoiSearcher) addSearchedPlaceToCache(ctx context.Context, placeID string, enrich placeDetailsEnricher) (AddSearchedPlaceResult, error) {
	candidate, err := s.redisClient.PlaceSearchCandidate(ctx, placeID)
	if err != nil {
		return AddSearchedPlaceResult{}, err
	}

	primary := POI.PrimaryLocationType(candidate.Types)
	cat, ok := POI.GetPlaceCategory(primary)
	if !ok {
		// NOTHING is written past this point: no CachedPlaces read result is used, no enrich
		// call, no SetPlacesAddGeoLocations. The refusal is total.
		return AddSearchedPlaceResult{}, fmt.Errorf("%w: %q", ErrUnsupportedPlaceType, primary)
	}

	cached, cacheErr := s.redisClient.CachedPlaces(ctx, []string{placeID})
	if cacheErr != nil {
		Logger.Debugf("addSearchedPlaceToCache: cached-place lookup failed for %s, proceeding as not-cached: %v", placeID, cacheErr)
	}
	alreadyCached := len(cached) > 0

	place := candidate
	details, enrichErr := enrich(ctx, placeID)
	if enrichErr != nil {
		Logger.Errorf("addSearchedPlaceToCache: Place Details enrich failed for %s, continuing with the lean stashed record: %v", placeID, enrichErr)
	} else {
		foldPlaceDetailsIntoPlace(&place, details)
	}

	places := []POI.Place{place}
	restoreCachedDetails(places, cached)
	place = places[0]

	// Re-tag with the true primary type (never trust candidate.LocationType, which may reflect
	// whatever the original search happened to be querying for).
	place.LocationType = primary
	s.redisClient.SetPlacesAddGeoLocations(ctx, []POI.Place{place})

	verify, verifyErr := s.redisClient.CachedPlaces(ctx, []string{placeID})
	if verifyErr != nil {
		return AddSearchedPlaceResult{}, fmt.Errorf("verifying cache write for place %s: %w", placeID, verifyErr)
	}
	if _, ok := verify[placeID]; !ok {
		return AddSearchedPlaceResult{}, fmt.Errorf("place %s was not persisted to the cache", placeID)
	}

	return AddSearchedPlaceResult{Place: place, Category: cat, AlreadyCached: alreadyCached}, nil
}

// foldPlaceDetailsIntoPlace folds Place Details fields onto an existing place, the same fields
// extensiveNearbySearch folds in for a nearby-search Details call. Details responses carry no
// geometry, so latitude/longitude are left untouched — they come from the stashed text-search
// candidate.
func foldPlaceDetailsIntoPlace(place *POI.Place, details maps.PlaceDetailsResult) {
	if details.OpeningHours != nil && len(details.OpeningHours.WeekdayText) == 7 {
		for weekday := POI.DateMonday; weekday <= POI.DateSunday; weekday++ {
			place.SetHour(weekday, details.OpeningHours.WeekdayText[weekday])
		}
	}
	if details.FormattedAddress != "" {
		place.FormattedAddress = details.FormattedAddress
	}
	if details.AdrAddress != "" {
		place.SetAddress(details.AdrAddress)
	}
	if details.URL != "" {
		place.URL = details.URL
	}
	if details.EditorialSummary != nil {
		place.Summary = details.EditorialSummary.Overview
	}
}
