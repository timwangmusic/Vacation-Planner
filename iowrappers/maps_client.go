package iowrappers

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"

	"go.uber.org/zap/zapcore"

	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/utils"
	"go.uber.org/zap"
	"googlemaps.github.io/maps"
)

// SearchClient defines an interface of a client that performs location-based operations such as nearby search
type SearchClient interface {
	Geocode(context.Context, *GeocodeQuery) (float64, float64, error)        // translate a textual location to latitude and longitude
	ReverseGeocode(context.Context, float64, float64) (*GeocodeQuery, error) // look up a textual location based on latitude and longitude
	NearbySearch(context.Context, *PlaceSearchRequest) ([]POI.Place, error)  // search nearby places in a category around a central location
}

type MapsClient struct {
	client               *maps.Client
	apiKey               string
	DetailedSearchFields []string
	apiSemaphore         chan struct{}
}

func (c *MapsClient) SetDetailedSearchFields(fields []string) {
	c.DetailedSearchFields = fields
	Logger.Debugf("Set the following fields for detailed place searches: %s",
		strings.Join(c.DetailedSearchFields, ", "))
}

// CreateMapsClient is a factory method for MapsClient
func CreateMapsClient(apiKey string) *MapsClient {
	mapsClient, err := maps.NewClient(maps.WithAPIKey(apiKey))
	if err != nil {
		Logger.Fatal(err)
	}
	if reflect.ValueOf(mapsClient).IsNil() {
		Logger.Fatal(errors.New("maps client does not exist"))
	}
	// Initialize semaphore with MaxConcurrentAPIRequests capacity
	semaphore := make(chan struct{}, 5) // Using constant value directly to avoid import cycle
	return &MapsClient{
		client:       mapsClient,
		apiKey:       apiKey,
		apiSemaphore: semaphore,
	}
}

func CreateLogger() error {
	env := strings.ToUpper(os.Getenv("ENVIRONMENT"))
	var err error
	var logger *zap.Logger

	if env == "" || env == "DEVELOPMENT" {
		logger, err = zap.NewDevelopment() // logging at debug level and above
	} else {
		cfg := zap.NewProductionConfig()
		cfg.Level.SetLevel(zapcore.DebugLevel) // logging at debug level and above
		logger, err = cfg.Build()
	}
	if err != nil {
		return err
	}

	Logger = logger.Sugar()
	if env == "" {
		env = "DEVELOPMENT"
	}
	Logger.Infof("->CreateLogger: the current environment is %s", env)
	return nil
}

func (c *MapsClient) ReverseGeocode(context context.Context, latitude, longitude float64) (*GeocodeQuery, error) {
	request := &maps.GeocodingRequest{
		LatLng: &maps.LatLng{
			Lat: latitude,
			Lng: longitude,
		},
		ResultType: []string{"country", "administrative_area_level_1", "locality"},
	}
	Logger.Debugf("reverse geocoding for latitude/longitude: %.2f/%.2f", latitude, longitude)

	// Acquire semaphore for API rate limiting
	c.apiSemaphore <- struct{}{}
	defer func() { <-c.apiSemaphore }() // Release semaphore

	geocodingResults, err := c.client.ReverseGeocode(context, request)
	if err != nil {
		return nil, err
	}
	if len(geocodingResults) == 0 {
		return nil, errors.New("no geocoding results found")
	}
	var query GeocodeQuery
	geocodingResultsToGeocodeQuery(&query, geocodingResults)
	return &query, nil
}

func (c *MapsClient) Geocode(ctx context.Context, query *GeocodeQuery) (lat float64, lng float64, err error) {
	Logger.Debugf("Geocoding for query %+v", *query)
	req := &maps.GeocodingRequest{
		Components: map[maps.Component]string{
			maps.ComponentLocality: query.City,
			maps.ComponentCountry:  query.Country,
		}}

	if strings.TrimSpace(query.AdminAreaLevelOne) != "" {
		req.Components[maps.ComponentAdministrativeArea] = strings.TrimSpace(query.AdminAreaLevelOne)
	}

	// Acquire semaphore for API rate limiting
	c.apiSemaphore <- struct{}{}
	defer func() { <-c.apiSemaphore }() // Release semaphore

	resp, err := c.client.Geocode(ctx, req)
	if err != nil {
		utils.LogErrorWithLevel(err, utils.LogError)
		return
	}

	if len(resp) < 1 {
		err = errors.New("maps geo-coding response invalid")
		utils.LogErrorWithLevel(err, utils.LogError)
		return
	}

	location := resp[0].Geometry.Location
	lat = location.Lat
	lng = location.Lng

	geocodingResultsToGeocodeQuery(query, resp)
	Logger.Debugf("Address components for the 1st response is %+v", resp[0].AddressComponents)

	return
}
