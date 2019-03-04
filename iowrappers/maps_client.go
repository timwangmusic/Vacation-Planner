package iowrappers

import (
	"github.com/sirupsen/logrus"
	"googlemaps.github.io/maps"
)

type MapsClient struct{
	client *maps.Client
	apiKey string
	logger *logrus.Logger
}
