package iowrappers

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bobg/go-generics/set"

	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/utils"
	"googlemaps.github.io/maps"
)

const (
	GoogleMapsSearchTimeout             = time.Second * 10
	GoogleMapsSearchCallMaxCount        = 5
	GoogleNearbySearchMaxRadiusInMeters = 50000
	MaxConcurrentAPIRequests            = 5
)

type PlaceSearchRequest struct {
	// "visit", "eatery", etc...
	PlaceCat POI.PlaceCategory

	Location POI.Location
	// search radius
	Radius uint
	// minimum number of results, set this lower limit for reducing risk of zero result in upper-layer computations.
	// suppose a location has more places established over time, this field would help trigger new searches to get those new establishments.
	MinNumResults uint

	// IncludeClosedPlaces opts a caller INTO receiving non-Operational places. The zero value
	// filters them from both cache reads and cold-search responses: closed places are persisted
	// in the cache (their closure is the signal that retires them), so a request that forgot to
	// ask for filtering must never serve one — planners consume these results directly. This
	// replaces the old BusinessStatus field, whose zero value was the UNFILTERED behavior and
	// therefore one forgotten assignment away from leaking closures.
	IncludeClosedPlaces bool
	// true if using precise geolocation instead of using a grander administrative area
	UsePreciseLocation bool

	PriceLevel POI.PriceLevel

	// Keyword is a brand or merchant keyword (e.g. "Dunkin'"). When set, the search is
	// keyword-based instead of place-type-based, and results are cached under a
	// brand-scoped Redis key (see POI.EncodeBrandNearbySearchRedisKey).
	Keyword string

	// StrictNameMatch only keeps results whose names match Keyword (after normalization).
	// It applies before results are cached so brand-scoped Redis buckets stay brand-pure.
	StrictNameMatch bool

	// DetailsLimit caps how many places get the expensive Place Details API call, chosen
	// by proximity to the request location. Zero means no cap (previous behavior).
	DetailsLimit int
}

// MatchesBrandName reports whether a place name matches a brand keyword after normalization,
// e.g. "Dunkin' Donuts #1234" matches keyword "Dunkin'"
func MatchesBrandName(placeName, keyword string) bool {
	normalizedKeyword := POI.NormalizeBrandKey(keyword)
	if normalizedKeyword == "" {
		return false
	}
	return strings.Contains(POI.NormalizeBrandKey(placeName), normalizedKeyword)
}

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

func (c *MapsClient) GoogleMapsNearbySearchWrapper(ctx context.Context, mapsReq maps.NearbySearchRequest) (resp maps.PlacesSearchResponse, err error) {
	resp, err = c.client.NearbySearch(ctx, &mapsReq)
	logErr(err, utils.LogError)
	return
}

func (c *MapsClient) NearbySearch(ctx context.Context, request *PlaceSearchRequest) ([]POI.Place, error) {
	var places = make([]POI.Place, 0)
	var searchDone = make(chan bool)
	ctx, cancelFunc := context.WithTimeout(ctx, GoogleMapsSearchTimeout)
	defer cancelFunc()

	go c.extensiveNearbySearch(ctx, GoogleMapsSearchCallMaxCount, request, &places, searchDone)

	select {
	case <-searchDone:
		return places, nil
	case <-ctx.Done():
		return places, errors.New("maps search time out")
	}
}

func (c *MapsClient) extensiveNearbySearch(ctx context.Context, maxRequestTimes uint, request *PlaceSearchRequest, places *[]POI.Place, done chan bool) {
	searchStartTime := time.Now()
	placeTypes := POI.GetPlaceTypes(request.PlaceCat) // get place types in a category
	if request.Keyword != "" {
		// keyword (brand) searches leave the place type unset so Google Maps matches the
		// keyword across all place types in a single search
		placeTypes = []POI.LocationType{POI.LocationTypeAny}
	}

	nextPageTokenMap := make(map[POI.LocationType]string) // map of place types to search token
	placeCountPerPlaceType := make(map[POI.LocationType]int)
	for _, placeType := range placeTypes {
		nextPageTokenMap[placeType] = ""
		placeCountPerPlaceType[placeType] = 0
	}

	var reqTimes uint = 0        // number of queries for each location type
	var totalPlaceCount uint = 0 // number of results so far, keep this number low
	// Bail out of the whole search once a single round sees every place type fail.
	// mapsFailuresCount is reset at the top of each round below (it must NOT be hoisted
	// out here), so reaching maxRetries means every place type failed in THAT round, not
	// cumulatively across rounds. A per-round counter is required: one persistently-failing
	// type must not be able to spend down a shared budget and cut off its healthy siblings'
	// remaining rounds. This was previously computed as reqTimes * len(placeTypes) while
	// reqTimes was still 0, making the cap 0 and the break below unreachable.
	maxRetries := uint(len(placeTypes))

	microAddrMap := make(map[string]string) // map place ID to its micro-address
	placeMap := make(map[string]bool)       // remove duplication for place with same ID
	urlMap := make(map[string]string)       // map place ID to url
	summaryMap := make(map[string]string)   // map place ID to summary

	detailsBudget := request.DetailsLimit // remaining Place Details calls across all pages; only enforced when DetailsLimit > 0

	// One place type's Nearby Search HTTP result (Phase A). Holds no shared state,
	// so goroutines can fill it by index without synchronization.
	type fetchResult struct {
		resp maps.PlacesSearchResponse
		err  error
		skip bool // this place type had no more pages this pass
	}
outer:
	for totalPlaceCount < request.MinNumResults {
		reqTimes++
		// Reset every round: this must count failures within the CURRENT round only, so
		// one place type failing round after round cannot exhaust the shared budget and
		// cut off healthy sibling types (see maxRetries comment above).
		var mapsFailuresCount uint = 0

		// Phase A — fetch every eligible place type's Nearby Search CONCURRENTLY.
		// These are independent, slow HTTP calls and the dominant cold-cache cost;
		// running them in parallel (bounded by the shared API semaphore) collapses a
		// category's N serial searches into one round. Each goroutine writes only its
		// own fetched[i], so no shared state is touched here.
		fetched := make([]fetchResult, len(placeTypes))
		var wg sync.WaitGroup
		for i, placeType := range placeTypes {
			if reqTimes > 1 && nextPageTokenMap[placeType] == "" { // no more results for this type
				fetched[i].skip = true
				continue
			}
			wg.Add(1)
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
		}
		wg.Wait()

		// Phase B — process the fetched responses SEQUENTIALLY in placeTypes order.
		// This is the original per-type body, so cross-type dedup (placeMap), the
		// shared Place Details budget, and the output order are all identical to the
		// previous sequential implementation.
		for i, placeType := range placeTypes {
			if fetched[i].skip {
				continue
			}
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

			processingStartTime := time.Now()
			searchResp := fetched[i].resp

			// What we already hold for these places, in one round trip. Place Details is the
			// dominant cost of a cold search (one call per place), so this is what keeps a
			// re-search over already-covered ground from re-buying all of it.
			cached := c.lookupCachedPlaces(ctx, searchResp.Results)

			// places for Google Maps place details search (https://developers.google.com/maps/documentation/places/web-service/details)
			// the original purpose of doing a details search is getting opening hours info
			// later on we added more fields of interest as specified in the config/config.yaml file
			placeIdMap := selectPlacesForDetails(request, &searchResp, &detailsBudget, cached, processingStartTime)

			placesToUpdate := set.Of[string]{}
			for _, placeId := range placeIdMap {
				placesToUpdate.Add(placeId)
			}
			searchDuration := c.searchPlaceDetails(ctx, placeIdMap, processingStartTime, &searchResp, summaryMap, microAddrMap, urlMap, placesToUpdate)

			parsed := parsePlacesSearchResponse(searchResp, placeType, microAddrMap, placeMap, urlMap, summaryMap)
			restoreCachedDetails(parsed, cached)
			*places = append(*places, parsed...)
			totalPlaceCount += uint(len(searchResp.Results))
			placeCountPerPlaceType[placeType] += len(searchResp.Results)
			nextPageTokenMap[placeType] = searchResp.NextPageToken

			Logger.Infow("Logging nearby search for individual place types",
				"center location (lat,lng)", request.Location,
				"place type:", placeType,
				"price level", request.PriceLevel,
				"place count so far", placeCountPerPlaceType[placeType],
				"API call time", searchDuration,
			)
		}
		if reqTimes == maxRequestTimes {
			break
		}
	}

	Logger.Infow("Logging nearby search for a complete Google Maps search",
		"center location (lat, lng)", request.Location,
		"place category", request.PlaceCat,
		"price level", request.PriceLevel,
		"total place count", totalPlaceCount,
		"total processing time", time.Since(searchStartTime),
	)
	done <- true
}

// selectPlacesForDetails picks the search results worth a Place Details API call: places
// missing opening hours, excluding results a strict brand-name match would later drop
// (details on those are wasted spend), excluding places whose stored record already carries
// current Details data, and — when the request sets DetailsLimit — capped to the remaining
// budget by proximity to the request location.
//
// The cache check runs BEFORE the budget cap, not after, so a limited budget is spent entirely
// on places we do not already have instead of being consumed by ones we do.
func selectPlacesForDetails(request *PlaceSearchRequest, searchResp *maps.PlacesSearchResponse, detailsBudget *int, cached map[string]POI.Place, now time.Time) map[int]string {
	type candidate struct {
		idx  int
		dist float64
	}
	candidates := make([]candidate, 0, len(searchResp.Results))
	for k, res := range searchResp.Results {
		if res.OpeningHours != nil && res.OpeningHours.WeekdayText != nil {
			continue
		}
		if request.Keyword != "" && request.StrictNameMatch && !MatchesBrandName(res.Name, request.Keyword) {
			continue
		}
		if place, ok := cached[res.PlaceID]; ok && placeDetailsAreCurrent(place, now) {
			continue
		}
		dist := utils.HaversineDist(
			[]float64{request.Location.Latitude, request.Location.Longitude},
			[]float64{res.Geometry.Location.Lat, res.Geometry.Location.Lng})
		candidates = append(candidates, candidate{idx: k, dist: dist})
	}

	if request.DetailsLimit > 0 {
		sort.Slice(candidates, func(i, j int) bool { return candidates[i].dist < candidates[j].dist })
		if len(candidates) > *detailsBudget {
			candidates = candidates[:*detailsBudget]
		}
		*detailsBudget -= len(candidates)
	}

	placeIdMap := make(map[int]string)
	for _, cand := range candidates {
		placeIdMap[cand.idx] = searchResp.Results[cand.idx].PlaceID
	}
	return placeIdMap
}

// restoreCachedDetails puts back the Details-sourced fields of places we already had stored,
// for any place that did not come away from this search with fresh ones.
//
// Required because the write path is a blind upsert: SetPlacesAddGeoLocations unconditionally
// Sets every place handed to it. Without this, a place whose Details call was skipped would be
// rebuilt from the bare Nearby result — CreatePlace backfilling default opening hours
// ("8:30 am – 9:30 pm") and leaving URL empty — and then overwrite the very record that caused
// it to be skipped, so the optimisation would quietly destroy the data it was exploiting.
//
// Restoring is per field and only ever fills a gap, never overwrites something this search
// obtained. That matters most for hours: a place whose hours arrived with the Nearby response is
// skipped for Details and so has no URL, and a blanket copy would replace those real hours with
// the stored record's DefaultOpeningHours placeholder. Hours cannot be tested for emptiness
// because CreatePlace always fills them, hence HasRealOpeningHours.
func restoreCachedDetails(places []POI.Place, cached map[string]POI.Place) {
	if len(cached) == 0 {
		return
	}
	for i := range places {
		stored, ok := cached[places[i].ID]
		if !ok {
			continue
		}
		if places[i].URL == "" {
			places[i].URL = stored.URL
		}
		if places[i].Summary == "" {
			places[i].Summary = stored.Summary
		}
		if places[i].FormattedAddress == "" {
			places[i].FormattedAddress = stored.FormattedAddress
		}
		if places[i].Address == (POI.Address{}) {
			places[i].Address = stored.Address
		}
		if !places[i].HasRealOpeningHours() && stored.HasRealOpeningHours() {
			places[i].Hours = stored.Hours
		}
	}
}

func (c *MapsClient) searchPlaceDetails(
	ctx context.Context,
	placeIdMap map[int]string,
	singlePlaceTypeSearchStartTime time.Time,
	searchResp *maps.PlacesSearchResponse,
	summaryMap, microAddrMap, urlMap map[string]string,
	placesToUpdate set.Of[string],
) time.Duration {
	detailsSearchResults := make([]PlaceDetailsSearchResult, len(placeIdMap))
	var wg sync.WaitGroup
	wg.Add(len(placeIdMap))
	// placeIdMap keys are indices into searchResp.Results and may be sparse (e.g. when a
	// details budget filters candidates), so each goroutine gets its own compact slot and
	// records the result index in PlaceDetailsSearchResult.idx
	slot := 0
	for idx, placeId := range placeIdMap {
		go PlaceDetailsSearchWrapper(ctx, c, idx, placeId, c.DetailedSearchFields, &detailsSearchResults[slot], &wg, placesToUpdate)
		slot++
	}
	wg.Wait()
	searchDuration := time.Since(singlePlaceTypeSearchStartTime)

	// fill fields from detail search results to nearby search results
	for _, placeDetails := range detailsSearchResults {
		if placeDetails.res == nil {
			// the details lookup failed or was skipped; leave the nearby-search data as is
			continue
		}
		idx := placeDetails.idx
		placeId := searchResp.Results[idx].PlaceID
		if !placesToUpdate.Has(placeId) {
			continue
		}

		summary := placeDetails.res.EditorialSummary
		if summary != nil {
			summaryMap[placeId] = summary.Overview
			Logger.Debugf("editorial summary for place %s is: %s", placeId, summary.Overview)
		}
		searchResp.Results[idx].OpeningHours = placeDetails.res.OpeningHours
		searchResp.Results[idx].FormattedAddress = placeDetails.res.FormattedAddress
		microAddrMap[placeId] = placeDetails.res.AdrAddress
		urlMap[placeId] = placeDetails.res.URL
	}
	return searchDuration
}

type PlaceDetailsSearchResult struct {
	res *maps.PlaceDetailsResult
	idx int
}

func PlaceDetailsSearchWrapper(context context.Context, mapsClient *MapsClient, idx int, placeId string, fields []string, detailSearchRes *PlaceDetailsSearchResult, wg *sync.WaitGroup, toUpdate set.Of[string]) {
	defer wg.Done()
	if !toUpdate.Has(placeId) {
		return
	}

	// Acquire semaphore (rate limiting)
	mapsClient.apiSemaphore <- struct{}{}
	defer func() { <-mapsClient.apiSemaphore }() // Release semaphore

	searchRes, err := mapsClient.PlaceDetailedSearch(context, placeId, fields)
	if err != nil {
		Logger.Error(err)
		return
	}
	*detailSearchRes = PlaceDetailsSearchResult{res: &searchRes, idx: idx}
}

func (c *MapsClient) PlaceDetailedSearch(context context.Context, placeId string, fields []string) (maps.PlaceDetailsResult, error) {
	if reflect.ValueOf(c.client).IsNil() {
		err := errors.New("client does not exist")
		return maps.PlaceDetailsResult{}, err
	}
	detailedSearchFields := strings.Join(fields, ",")
	req := &maps.PlaceDetailsRequest{
		PlaceID: placeId,
	}
	if detailedSearchFields != "" {
		fieldMask, err := parseFields(detailedSearchFields)
		if err != nil {
			return maps.PlaceDetailsResult{}, err
		}
		req.Fields = fieldMask
	}

	resp, err := c.client.PlaceDetails(context, req)
	return resp, err
}

func parsePlacesSearchResponse(resp maps.PlacesSearchResponse, locationType POI.LocationType, microAddrMap map[string]string, placeMap map[string]bool, urlMap map[string]string, summaryMap map[string]string) (places []POI.Place) {
	for _, res := range resp.Results {
		id := res.PlaceID
		if seen := placeMap[id]; !seen {
			placeMap[id] = true
		} else {
			continue
		}
		name := res.Name
		latitude := res.Geometry.Location.Lat
		longitude := res.Geometry.Location.Lng
		addr := ""
		if microAddrMap != nil {
			addr = microAddrMap[id]
		}
		priceLevel := res.PriceLevel
		h := &POI.OpeningHours{}
		if res.OpeningHours != nil && res.OpeningHours.WeekdayText != nil && len(res.OpeningHours.WeekdayText) > 0 {
			h.Hours = append(h.Hours, res.OpeningHours.WeekdayText...)
		}
		rating := res.Rating
		url := urlMap[id]
		var photo *maps.Photo
		if len(res.Photos) > 0 {
			photo = &res.Photos[0]
		}
		userRatingsTotal := res.UserRatingsTotal
		// filter places with zero user ratings
		if userRatingsTotal == 0 {
			continue
		}
		var placeSummary *string
		if summary, ok := summaryMap[id]; ok {
			placeSummary = &summary
		}

		place := POI.CreatePlace(name, addr, res.FormattedAddress, res.BusinessStatus, locationType, h, id, priceLevel, rating, url, photo, userRatingsTotal, latitude, longitude, placeSummary)
		// Preserve Google's actual feature types so callers can classify the place by
		// its primary function, not the type this search happened to query for.
		place.Types = res.Types
		places = append(places, place)
	}
	return
}

// refs: maps/examples/places/placedetails/placedetails.go
func parseFields(fields string) ([]maps.PlaceDetailsFieldMask, error) {
	var res []maps.PlaceDetailsFieldMask
	for _, s := range strings.Split(fields, ",") {
		f, err := maps.ParsePlaceDetailsFieldMask(s)
		if logErr(err, utils.LogError) {
			return res, err
		}
		res = append(res, f)
	}
	return res, nil
}

func logErr(err error, logLevel uint) bool {
	return utils.LogErrorWithLevel(err, logLevel)
}
