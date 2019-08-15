package iowrappers

import (
	"Vacation-planner/utils"
	"context"
	"errors"
	log "github.com/sirupsen/logrus"
	"googlemaps.github.io/maps"
)

// Translate city, country to its central location
func (c MapsClient) Geocode(query GeocodeQuery) (lat float64, lng float64) {
	req := &maps.GeocodingRequest{
		Components: map[maps.Component]string{
			maps.ComponentLocality: query.City,
			maps.ComponentCountry:  query.Country,
		}}

	resp, err := c.client.Geocode(context.Background(), req)
	utils.CheckErr(err)

	if len(resp) < 1 {
		log.Fatal(errors.New("Maps geocoding response invalid"))
	}

	location := resp[0].Geometry.Location
	lat = location.Lat
	lng = location.Lng
	return
}
