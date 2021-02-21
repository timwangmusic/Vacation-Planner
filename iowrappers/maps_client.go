package iowrappers

import (
	"context"
	"errors"
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/utils"
	"go.uber.org/zap"
	"googlemaps.github.io/maps"
	"os"
	"reflect"
	"strings"
)

// abstraction of a client that performs location-based operations such as nearby search
type SearchClient interface {
	GetGeocode(context.Context, *GeocodeQuery) (float64, float64, error)     // translate a textual location to latitude and longitude
	NearbySearch(context.Context, *PlaceSearchRequest) ([]POI.Place, error)  // search nearby places in a category around a central location
	PlaceDetailsSearch(context.Context, string) (place POI.Place, err error) // search place details with place ID
}

type MapsClient struct {
	client               *maps.Client
	apiKey               string
	DetailedSearchFields []string
}

func (mapsClient *MapsClient) SetDetailedSearchFields(fields []string) {
	mapsClient.DetailedSearchFields = fields
	Logger.Debugf("Set the following fields in detailed place search: %s",
		strings.Join(mapsClient.DetailedSearchFields, ", "))
}

// factory method for MapsClient
func CreateMapsClient(apiKey string) MapsClient {
	logErr(CreateLogger(), utils.LogError)
	mapsClient, err := maps.NewClient(maps.WithAPIKey(apiKey))
	if err != nil {
		Logger.Fatal(err)
	}
	if reflect.ValueOf(mapsClient).IsNil() {
		Logger.Fatal(errors.New("maps client does not exist"))
	}
	return MapsClient{client: mapsClient, apiKey: apiKey}
}

func CreateLogger() error {
	currentEnv := os.Getenv("ENVIRONMENT")
	var err error
	var logger *zap.Logger

	if currentEnv == "" || currentEnv == "development" {
		logger, err = zap.NewDevelopment() // logging at debug level and above
	} else {
		logger, err = zap.NewProduction() // logging at info level and above
	}
	if err != nil {
		return err
	}

	Logger = logger.Sugar()

	return nil
}

func (mapsClient *MapsClient) ReverseGeocoding(context context.Context, latitude, longitude float64) (GeocodeQuery, error) {
	request := &maps.GeocodingRequest{
		LatLng: &maps.LatLng{
			Lat: latitude,
			Lng: longitude,
		},
		// currently we only need country and city info
		ResultType: []string{"country", "locality"},
	}
	Logger.Debugf("reverse geocoding for latitude/longitude: %.2f/%.2f", latitude, longitude)
	geocodingResults, err := mapsClient.client.ReverseGeocode(context, request)
	if err != nil {
		return GeocodeQuery{}, err
	}
	if geocodingResults == nil || len(geocodingResults) == 0 {
		return GeocodeQuery{}, errors.New("no geocoding results found")
	}
	return geocodingResultsToGeocodeQuery(geocodingResults), nil
}
