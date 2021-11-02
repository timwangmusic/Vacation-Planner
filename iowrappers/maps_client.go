package iowrappers

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"

	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/utils"
	"go.uber.org/zap"
	"googlemaps.github.io/maps"
)

// SearchClient defines an interface of a client that performs location-based operations such as nearby search
type SearchClient interface {
	Geocode(context.Context, *GeocodeQuery) (float64, float64, error)       // translate a textual location to latitude and longitude
	NearbySearch(context.Context, *PlaceSearchRequest) ([]POI.Place, error) // search nearby places in a category around a central location
}

type MapsClient struct {
	client               *maps.Client
	apiKey               string
	DetailedSearchFields []string
}

func (mapsClient *MapsClient) SetDetailedSearchFields(fields []string) {
	mapsClient.DetailedSearchFields = fields
	Logger.Debugf("Set the following fields for detailed place searches: %s",
		strings.Join(mapsClient.DetailedSearchFields, ", "))
}

// CreateMapsClient is a factory method for MapsClient
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
	currentEnv := strings.ToUpper(os.Getenv("ENVIRONMENT"))
	var err error
	var logger *zap.Logger

	if currentEnv == "" || currentEnv == "DEVELOPMENT" {
		logger, err = zap.NewDevelopment() // logging at debug level and above
	} else {
		logger, err = zap.NewProduction() // logging at info level and above
	}
	if err != nil {
		return err
	}

	Logger = logger.Sugar()
	Logger.Infof("current environment is %s", currentEnv)
	return nil
}

func (mapsClient *MapsClient) ReverseGeocoding(context context.Context, latitude, longitude float64) (GeocodeQuery, error) {
	request := &maps.GeocodingRequest{
		LatLng: &maps.LatLng{
			Lat: latitude,
			Lng: longitude,
		},
		// currently we require country, admistrative area level 1 and city info
		ResultType: []string{"country", "administrative_area_level_1", "locality"},
	}
	Logger.Debugf("reverse geocoding for latitude/longitude: %.2f/%.2f", latitude, longitude)
	geocodingResults, err := mapsClient.client.ReverseGeocode(context, request)
	if err != nil {
		return GeocodeQuery{}, err
	}
	if len(geocodingResults) == 0 {
		return GeocodeQuery{}, errors.New("no geocoding results found")
	}
	return geocodingResultsToGeocodeQuery(geocodingResults), nil
}

// Geocode converts city, country to its central location
func (mapsClient MapsClient) Geocode(ctx context.Context, query *GeocodeQuery) (lat float64, lng float64, err error) {
	Logger.Debugf("Geocoding for query %+v", *query)
	req := &maps.GeocodingRequest{
		Components: map[maps.Component]string{
			maps.ComponentLocality: query.City,
			maps.ComponentCountry:  query.Country,
		}}

	if strings.TrimSpace(query.AdminAreaLevelOne) != "" {
		req.Components[maps.ComponentAdministrativeArea] = strings.TrimSpace(query.AdminAreaLevelOne)
	}

	resp, err := mapsClient.client.Geocode(ctx, req)
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

	*query = geocodingResultsToGeocodeQuery(resp)
	Logger.Debugf("Address components for the 1st response is %+v", resp[0].AddressComponents)

	return
}
