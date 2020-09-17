package iowrappers

import (
	"errors"
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/utils"
	"go.uber.org/zap"
	"googlemaps.github.io/maps"
	"os"
	"reflect"
)

// abstraction of a client that performs location-based operations such as nearby search
type SearchClient interface {
	GetGeocode(*GeocodeQuery) (float64, float64, error)    // translate a textual location to latitude and longitude
	NearbySearch(*PlaceSearchRequest) ([]POI.Place, error) // search nearby places in a category around a central location
}

type MapsClient struct {
	client *maps.Client
	apiKey string
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
