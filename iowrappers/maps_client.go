package iowrappers

import (
	"go.uber.org/zap"
	"googlemaps.github.io/maps"
	"os"
)

type SearchClient interface {
	Init(apiKey string) error
}

type MapsClient struct {
	client *maps.Client
	apiKey string
}

// create google maps client with api key
func (c *MapsClient) Init(apiKey string) error {
	var err error
	c.client, err = maps.NewClient(maps.WithAPIKey(apiKey))
	if err != nil {
		return err
	}
	return createLogger()
}

func createLogger() error {
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
