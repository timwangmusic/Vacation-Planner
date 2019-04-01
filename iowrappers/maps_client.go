package iowrappers

import (
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	"googlemaps.github.io/maps"
)

type SearchClient interface {
	Create (apiKey string) error
}

type MapsClient struct{
	client *maps.Client
	apiKey string
	logger *logrus.Logger
}

// create google maps client with api key
func (c *MapsClient) Create(apiKey string) error{
	var err error
	c.client, err = maps.NewClient(maps.WithAPIKey(apiKey))
	if err != nil{
		return err
	}
	c.createLogger("")
	return nil
}

func (c *MapsClient) createLogger (formatterSelection string){
	c.logger = log.New()
	if formatterSelection == "JSON"{	// TextFormatter by default
		c.logger.Formatter = &log.JSONFormatter{
			PrettyPrint: true,
		}
	} else {
		c.logger.Formatter = &log.TextFormatter{
			DisableColors: false,
			FullTimestamp: true,
		}
	}
}
