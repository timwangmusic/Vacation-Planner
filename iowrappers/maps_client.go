package iowrappers

import "googlemaps.github.io/maps"

type MapsClient struct{
	client *maps.Client
	apiKey string
}
