package iowrappers

import (
	"context"
	"errors"
	"googlemaps.github.io/maps"
)

// Translate city, country to its central location
func (c MapsClient) Geocode(query GeocodeQuery) (lat float64, lng float64, err error) {
	req := &maps.GeocodingRequest{
		Components: map[maps.Component]string{
			maps.ComponentLocality: query.City,
			maps.ComponentCountry:  query.Country,
		}}

	resp, err := c.client.Geocode(context.Background(), req)
	if err != nil {
		return
	}

	if len(resp) < 1 {
		err = errors.New("maps geo-coding response invalid")
		return
	}

	location := resp[0].Geometry.Location
	lat = location.Lat
	lng = location.Lng
	return
}
