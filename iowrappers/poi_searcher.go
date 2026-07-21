package iowrappers

import (
	"context"
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
	GoogleSearchHomePageURL      = "https://www.google.com/"
	ContextRequestIdKey          = ContextKey("request_id")
	ContextRequestUserId         = ContextKey("user_id")
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

func (s *PoiSearcher) ReverseGeocode(ctx context.Context, lat, lng float64) (*GeocodeQuery, error) {
	Logger.Debugf("PoiSearcher ->ReverseGeocode: decoding latitude %.2f, longitude %.2f", lat, lng)
	return s.mapsClient.ReverseGeocode(ctx, lat, lng)
}

func (s *PoiSearcher) NearbySearch(context context.Context, request *PlaceSearchRequest) ([]POI.Place, error) {
	if err := s.processLocation(context, request); err != nil {
		return nil, err
	}
	location := request.Location

	var savedPlaces, places []POI.Place
	var placesErr error
	savedPlaces, placesErr = s.redisClient.NearbySearch(context, request)
	if placesErr != nil {
		Logger.Error(placesErr)
	}

	Logger.Debugf("(PoiSearcher)NearbySearch: [request_id: %s] the number of results from redis is %d", context.Value(ContextRequestIdKey), len(savedPlaces))

	// update last search time for the city
	var lastSearchTime time.Time
	var lastSearchTimeMiss error
	if request.Keyword != "" {
		lastSearchTime, lastSearchTimeMiss = s.redisClient.GetBrandMapsLastSearchTime(context, location, request.Keyword)
	} else {
		lastSearchTime, lastSearchTimeMiss = s.redisClient.GetMapsLastSearchTime(context, location, request.PlaceCat, request.PriceLevel)
	}

	currentTime := time.Now()

	isSavedPlacesFresh := func() bool {
		return currentTime.Sub(lastSearchTime) <= MinMapsResultRefreshDuration && lastSearchTimeMiss == nil
	}
	// use place data from the database if it is fresh and at least one saved place satisfies the request
	if isSavedPlacesFresh() && placesErr == nil && len(savedPlaces) > 0 {
		Logger.Infof("(PoiSearcher)NearbySearch: [request_id: %s] Using Redis to fulfill request for location %+v with category %s, keyword %q and price level %d",
			context.Value(ContextRequestIdKey),
			request.Location,
			request.PlaceCat,
			request.Keyword,
			request.PriceLevel)
		places = append(places, savedPlaces...)
		return places, nil
	}

	if request.Keyword != "" {
		utils.LogErrorWithLevel(s.redisClient.SetBrandMapsLastSearchTime(context, location, request.Keyword, currentTime.Format(time.RFC3339)), utils.LogError)
	} else {
		utils.LogErrorWithLevel(s.redisClient.SetMapsLastSearchTime(context, location, request.PlaceCat, request.PriceLevel, currentTime.Format(time.RFC3339)), utils.LogError)
	}

	// initiate a new external search
	newPlaces, searchErr := s.searchPlacesWithMaps(context, request)
	if searchErr != nil {
		return nil, searchErr
	}

	if request.Keyword != "" && request.StrictNameMatch {
		// drop keyword-search results that are merely related to the brand (Google Maps
		// matches keywords against reviews and other content, not just names) before they
		// reach the brand-scoped cache
		newPlaces = Filter(newPlaces, func(place POI.Place) bool { return MatchesBrandName(place.Name, request.Keyword) })
	}

	// safeguard on accessing elements in a nil slice
	if len(newPlaces) > 0 {
		// update Redis with all the new places obtained
		if request.Keyword != "" {
			s.redisClient.SetPlacesAddGeoLocationsForBrand(context, request.Keyword, newPlaces)
		} else {
			s.UpdateRedis(context, newPlaces)
		}

		// include places from cache in the result
		places = append(places, newPlaces...)
	}

	return places, nil
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
		geoQuery, err := s.GetMapsClient().ReverseGeocode(ctx, req.Location.Latitude, req.Location.Longitude)
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

	if req.BusinessStatus == POI.Operational {
		totalPlacesCount := len(places)
		places = Filter(places, func(place POI.Place) bool { return place.Status == POI.Operational })
		Logger.Debugf("%d places out of %d left after business status filtering", len(places), totalPlacesCount)
	}

	if uint(len(places)) < req.MinNumResults {
		Logger.Debugf("Found %d POI results for place type %s, less than requested number of %d",
			len(places), req.PlaceCat, req.MinNumResults)
	}
	if len(places) == 0 {
		Logger.Debugf("No qualified POI result found in the given location %v, radius %d, and place type: %s. The location may be invalid",
			req.Location, req.Radius, req.PlaceCat)
	}
	return places, nil
}

func (s *PoiSearcher) UpdateRedis(context context.Context, places []POI.Place) {
	s.redisClient.SetPlacesAddGeoLocations(context, places)
	requestId := context.Value(ContextRequestIdKey)
	Logger.Debugf("[request_id: %s]Redis update complete", requestId)
}
