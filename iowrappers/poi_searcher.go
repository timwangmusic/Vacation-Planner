package iowrappers

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	gogeonames "github.com/timwangmusic/go-geonames"
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/utils"
	"go.uber.org/zap"
)

type ContextKey string

const (
	MaxSearchRadius              = 16000               // 10 miles, upper bound for radius requested by callers
	ColdStartSearchRadius        = 8000                // 5 miles, radius used for external maps searches that populate the cache
	MinMapsResultRefreshDuration = time.Hour * 24 * 14 // 14 days
	// MinEmptyResultRefreshDuration is the refresh window for a search that came back with
	// nothing. Shorter than MinMapsResultRefreshDuration so an area that has just been built
	// out is retried within a day, rather than being frozen for the full two weeks.
	MinEmptyResultRefreshDuration = time.Hour * 24 // 1 day
	// PlaceDetailsRefreshDuration is how long a stored place's Details-sourced fields (opening
	// hours, formatted address, URL, editorial summary) are trusted before a cold search buys
	// them again. Business status arrives with every Nearby Search, so closures are still caught
	// by the Operational filter regardless of this window.
	PlaceDetailsRefreshDuration = time.Hour * 24 * 90 // 90 days
	GoogleSearchHomePageURL     = "https://www.google.com/"
	ContextRequestIdKey         = ContextKey("request_id")
	ContextRequestUserId        = ContextKey("user_id")
)

type PoiSearcher struct {
	mapsClient  *MapsClient
	redisClient *RedisClient
}

// GeocodeQuery can also be used as the result of reverse geocoding
type GeocodeQuery struct {
	City              string `json:"city"`
	AdminAreaLevelOne string `json:"admin_area_level_one"`
	Country           string `json:"country"`
}

func (gq *GeocodeQuery) String() string {
	c := cases.Title(language.English)
	return strings.Join([]string{c.String(gq.City), strings.ToUpper(gq.AdminAreaLevelOne), c.String(gq.Country)}, ", ")
}

type NearbyCityRequest struct {
	ApiKey   string                  `json:"apiKey"`
	Location POI.Location            `json:"location"`
	Radius   float64                 `json:"radius"`
	Filter   gogeonames.SearchFilter `json:"filter"`
}

type NearbyCityResponse struct {
	Cities []City `json:"cities"`
}

var Logger *zap.SugaredLogger

func CreatePoiSearcher(mapsApiKey string, redisUrl *url.URL) *PoiSearcher {
	poiSearcher := PoiSearcher{
		mapsClient:  CreateMapsClient(mapsApiKey),
		redisClient: CreateRedisClient(redisUrl),
	}
	// Let external searches consult the cache before buying Place Details for a place we already
	// have. Wired here rather than in CreateMapsClient so the maps client keeps no dependency on
	// Redis.
	poiSearcher.mapsClient.SetCachedPlaceLookup(poiSearcher.redisClient.CachedPlaces)
	return &poiSearcher
}

func (s *PoiSearcher) GetMapsClient() *MapsClient {
	return s.mapsClient
}

func (s *PoiSearcher) GetRedisClient() *RedisClient {
	return s.redisClient
}

func DestroyLogger() {
	_ = Logger.Sync()
}

func (s *PoiSearcher) NearbyCities(ctx context.Context, req *NearbyCityRequest) (NearbyCityResponse, error) {
	Logger.Debugf("->NearbyCities: processing request %+v", *req)
	knownCities, err := s.redisClient.NearbyCities(ctx, req.Location.Latitude, req.Location.Longitude, req.Radius, req.Filter)
	if err != nil {
		Logger.Error(err)
	} else if len(knownCities) > 0 {
		return NearbyCityResponse{Cities: knownCities}, nil
	}

	c := gogeonames.Client{Username: req.ApiKey}

	cities, err := c.GetNearbyCities(&gogeonames.SearchRequest{
		Latitude:  req.Location.Latitude,
		Longitude: req.Location.Longitude,
		Radius:    req.Radius,
	}, req.Filter)
	if err != nil {
		return NearbyCityResponse{}, err
	}

	citiesToSave := make([]City, 0)
	for _, city := range cities {
		var c City
		if c, err = toCity(city); err != nil {
			Logger.Error(err)
		} else {
			citiesToSave = append(citiesToSave, c)
		}
	}

	if err = s.redisClient.AddCities(ctx, citiesToSave); err != nil {
		Logger.Error(err)
	}

	return NearbyCityResponse{Cities: citiesToSave}, err
}

// Geocode performs geocoding, mapping city and country to latitude and longitude
func (s *PoiSearcher) Geocode(context context.Context, query *GeocodeQuery) (lat float64, lng float64, err error) {
	originalGeocodeQuery := GeocodeQuery{}
	originalGeocodeQuery.City = query.City
	originalGeocodeQuery.Country = query.Country
	originalGeocodeQuery.AdminAreaLevelOne = query.AdminAreaLevelOne
	var geocodeMissingErr error
	lat, lng, geocodeMissingErr = s.redisClient.Geocode(context, query)
	if geocodeMissingErr != nil {
		lat, lng, err = s.mapsClient.Geocode(context, query)
		if err != nil {
			return
		}
		// either redisClient or mapsClient may have corrected location fields in the query
		s.redisClient.SetGeocode(context, *query, lat, lng, originalGeocodeQuery)
		Logger.Debugf("Geolocation (lat,lng) Cache miss for location %s, %s is %.4f, %.4f",
			query.City, query.Country, lat, lng)
	}
	return
}

// ReverseGeocode resolves city-level info for a coordinate, consulting the per-cell Redis cache
// before Google. This runs on EVERY nearby scan (processLocation), and on a warm place cache it
// was the only Google call left — caching it takes a warm scan's Google spend to zero.
func (s *PoiSearcher) ReverseGeocode(ctx context.Context, lat, lng float64) (*GeocodeQuery, error) {
	Logger.Debugf("PoiSearcher ->ReverseGeocode: decoding latitude %.2f, longitude %.2f", lat, lng)
	if cached, err := s.redisClient.ReverseGeocode(ctx, lat, lng); err == nil {
		return cached, nil
	}
	query, err := s.mapsClient.ReverseGeocode(ctx, lat, lng)
	if err != nil {
		return nil, err
	}
	s.redisClient.SetReverseGeocode(ctx, lat, lng, *query)
	return query, nil
}

// canServeFromCache decides whether cached places can satisfy a request without an external
// search. cachedCount is how many places the geo read returned, readErr its failure if any,
// markerMiss whether this cell has no freshness marker, and markerAge how long ago the marker
// says an external search last covered the cell.
//
// A fresh marker is honoured even at cachedCount == 0: it records that we already asked Google
// about this cell, including when the honest answer was "nothing here". The previous version
// additionally required cachedCount > 0, which left the marker unable to suppress anything — a
// bucket that read back zero fell through to Google, re-stamped the marker, and did the same
// again on the very next request, forever rather than once. That is what made sparse categories
// re-search on every single request.
//
// An empty result gets a shorter window than a populated one so an area that has just been built
// out is retried within a day instead of being frozen for the full refresh period.
func canServeFromCache(cachedCount int, readErr, markerMiss error, markerAge time.Duration) bool {
	if readErr != nil || markerMiss != nil {
		return false
	}
	refreshWindow := MinMapsResultRefreshDuration
	if cachedCount == 0 {
		refreshWindow = MinEmptyResultRefreshDuration
	}
	return markerAge <= refreshWindow
}

func (s *PoiSearcher) NearbySearch(context context.Context, request *PlaceSearchRequest) ([]POI.Place, error) {
	if err := s.processLocation(context, request); err != nil {
		return nil, err
	}
	lat, lng := request.Location.Latitude, request.Location.Longitude

	var savedPlaces, places []POI.Place
	var placesErr error
	savedPlaces, placesErr = s.redisClient.NearbySearch(context, request)
	if placesErr != nil {
		Logger.Error(placesErr)
	}

	// when an external search last covered this location cell
	var lastSearchTime time.Time
	var lastSearchTimeMiss error
	if request.Keyword != "" {
		lastSearchTime, lastSearchTimeMiss = s.redisClient.GetBrandMapsLastSearchTime(context, lat, lng, request.Keyword)
	} else {
		lastSearchTime, lastSearchTimeMiss = s.redisClient.GetMapsLastSearchTime(context, lat, lng, request.PlaceCat, request.PriceLevel)
	}

	currentTime := time.Now()

	markerAge := currentTime.Sub(lastSearchTime)
	isFresh := canServeFromCache(len(savedPlaces), placesErr, lastSearchTimeMiss, markerAge)

	// Log everything needed to tell an empty bucket from a stale marker from a wrong key. The
	// count alone cannot distinguish them.
	Logger.Debugw("(PoiSearcher)NearbySearch: redis lookup",
		"request_id", context.Value(ContextRequestIdKey),
		"places_from_redis", len(savedPlaces),
		"bucket_key", nearbySearchRedisKey(request),
		"category", request.PlaceCat,
		"price_level", request.PriceLevel,
		"keyword", request.Keyword,
		"search_center", fmt.Sprintf("%.4f,%.4f", lat, lng),
		"cell", POI.EncodeSearchCell(lat, lng),
		"radius", request.Radius,
		"marker_age", markerAge,
		"marker_missing", lastSearchTimeMiss != nil,
		"fresh", isFresh,
	)

	if isFresh {
		Logger.Infof("(PoiSearcher)NearbySearch: [request_id: %s] Using Redis to fulfill request for location %+v with category %s, keyword %q and price level %d",
			context.Value(ContextRequestIdKey),
			request.Location,
			request.PlaceCat,
			request.Keyword,
			request.PriceLevel)
		places = append(places, savedPlaces...)
		return places, nil
	}

	// initiate a new external search
	newPlaces, searchErr := s.searchPlacesWithMaps(context, request)
	if searchErr != nil {
		return nil, searchErr
	}

	// Stamp the marker only after a search that actually succeeded. Stamping before the call
	// was harmless while empty results were ignored, but now that a fresh marker suppresses the
	// call, a failed search would silence retries for a full day.
	if request.Keyword != "" {
		utils.LogErrorWithLevel(s.redisClient.SetBrandMapsLastSearchTime(context, lat, lng, request.Keyword, currentTime.Format(time.RFC3339)), utils.LogError)
	} else {
		utils.LogErrorWithLevel(s.redisClient.SetMapsLastSearchTime(context, lat, lng, request.PlaceCat, request.PriceLevel, currentTime.Format(time.RFC3339)), utils.LogError)
	}

	places = append(places, s.persistAndFilterSearchResults(context, request, newPlaces)...)

	return places, nil
}

// persistAndFilterSearchResults writes a cold search's results to the cache and returns the slice
// the caller may serve. The write deliberately includes non-Operational places: a permanent
// closure reported by Google is a signal we must PERSIST (so the read-side Operational filters
// retire the stale record everywhere), not discard — the previous pre-write filter meant a closed
// place kept its OPERATIONAL record and cache membership forever. The response filter then keeps
// closures out of what the caller sees, same as before.
func (s *PoiSearcher) persistAndFilterSearchResults(ctx context.Context, req *PlaceSearchRequest, newPlaces []POI.Place) []POI.Place {
	if req.Keyword != "" && req.StrictNameMatch {
		// drop keyword-search results that are merely related to the brand (Google Maps
		// matches keywords against reviews and other content, not just names) before they
		// reach the brand-scoped cache
		newPlaces = Filter(newPlaces, func(place POI.Place) bool { return MatchesBrandName(place.Name, req.Keyword) })
	}

	// safeguard on accessing elements in a nil slice
	if len(newPlaces) > 0 {
		// update Redis with all the new places obtained, closures included
		if req.Keyword != "" {
			s.redisClient.SetPlacesAddGeoLocationsForBrand(ctx, req.Keyword, newPlaces)
		} else {
			s.UpdateRedis(ctx, newPlaces)
		}
	}

	if !req.IncludeClosedPlaces {
		totalPlacesCount := len(newPlaces)
		newPlaces = Filter(newPlaces, func(place POI.Place) bool { return place.Status == POI.Operational })
		Logger.Debugf("%d places out of %d left after business status filtering", len(newPlaces), totalPlacesCount)
	}

	if uint(len(newPlaces)) < req.MinNumResults {
		Logger.Debugf("Found %d POI results for place type %s, less than requested number of %d",
			len(newPlaces), req.PlaceCat, req.MinNumResults)
	}
	if len(newPlaces) == 0 {
		Logger.Debugf("No qualified POI result found in the given location %v, radius %d, and place type: %s. The location may be invalid",
			req.Location, req.Radius, req.PlaceCat)
	}
	return newPlaces
}

// processLocation performs reverse geocoding for precise location to find city-level information and performs geocoding to find precise latitude and longitude values
func (s *PoiSearcher) processLocation(ctx context.Context, req *PlaceSearchRequest) error {
	location := &req.Location
	// the location is already fully resolved (precise coordinates plus city-level info);
	// callers fanning out multiple searches around one coordinate resolve it once up front
	if !req.UsePreciseLocation && location.City != "" && (location.Latitude != 0 || location.Longitude != 0) {
		return nil
	}
	if req.UsePreciseLocation {
		Logger.Debugf("->NearbySearch: using precise location")
		geoQuery, err := s.ReverseGeocode(ctx, req.Location.Latitude, req.Location.Longitude)
		if err != nil {
			return err
		}
		location.City = geoQuery.City
		location.AdminAreaLevelOne = geoQuery.AdminAreaLevelOne
		location.Country = geoQuery.Country
		return nil
	}

	lat, lng, err := s.Geocode(ctx, &GeocodeQuery{
		City:              location.City,
		AdminAreaLevelOne: location.AdminAreaLevelOne,
		Country:           location.Country,
	})
	if err != nil {
		return err
	}
	location.Latitude = lat
	location.Longitude = lng
	return nil
}

func (s *PoiSearcher) searchPlacesWithMaps(ctx context.Context, req *PlaceSearchRequest) ([]POI.Place, error) {
	originalRadius := req.Radius

	// use a larger-than-requested search radius whenever we call external maps services,
	// so one API spend populates the cache for a whole area
	req.Radius = ColdStartSearchRadius

	places, err := s.GetMapsClient().NearbySearch(ctx, req)

	// restore search radius upon search completion
	req.Radius = originalRadius
	if err != nil {
		return nil, err
	}

	// No status filtering here: closures must reach the cache write so they are persisted —
	// persistAndFilterSearchResults owns both the write and the response-side filter.
	return places, nil
}

func (s *PoiSearcher) UpdateRedis(context context.Context, places []POI.Place) {
	s.redisClient.SetPlacesAddGeoLocations(context, places)
	requestId := context.Value(ContextRequestIdKey)
	Logger.Debugf("[request_id: %s]Redis update complete", requestId)
}
