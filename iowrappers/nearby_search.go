package iowrappers

import (
	"Vacation-planner/POI"
	"context"
	"fmt"
	"googlemaps.github.io/maps"
	"log"
	"strings"
)

type LocationType string

const(
	LocationTypeCafe = LocationType("cafe")
	LocationTypeRestaurant = LocationType("restaurant")
	LocationTypeMuseum = LocationType("museum")
	LocationTypeGallery = LocationType("art_gallery")
	LocationTypeAmusementPark = LocationType("amusement_park")
)

type PlaceSearchRequest struct{
	// "lat,lng"
	Location string
	// "cafe", "museum",...
	PlaceType LocationType
	// search radius
	Radius uint
	// next page token
	NextPageToken string
}

func check(err error){
	if err != nil{
		log.Fatalf("fatal error: %s", err)
	}
}

func NearbySearchSDK (c MapsClient, req PlaceSearchRequest) (resp maps.PlacesSearchResponse){
	var err error
	latlng, err := maps.ParseLatLng(req.Location)
	check(err)

	mapsReq := maps.NearbySearchRequest{
		Location: &latlng,
		Type: maps.PlaceType(req.PlaceType),
		Radius: req.Radius,
		PageToken:req.NextPageToken,
	}

	resp, err = c.client.NearbySearch(context.Background(), &mapsReq)
	check(err)
	return
}

// create google maps client with api key
func (c *MapsClient) CreateClient (apiKey string){
	var err error
	c.client, err = maps.NewClient(maps.WithAPIKey(apiKey))
	check(err)
}

// SimpleNearbySearch searches results from a place category once for each location type in the category
func (c *MapsClient) SimpleNearbySearch(centerLocation string, placeCat POI.PlaceCategory, radius uint)(places []POI.Place){
	placeTypes := getTypes(placeCat)
	for _, placetype := range placeTypes{
		req := PlaceSearchRequest{
			Location:      centerLocation,
			PlaceType:     placetype,
			Radius:        radius,
		}
		searchResp := NearbySearchSDK(*c, req)
		places = append(places, parsePlacesSearchResponse(searchResp, placetype)...)
	}
	return
}

func parsePlacesSearchResponse(resp maps.PlacesSearchResponse, locationType LocationType) (places []POI.Place){
	searchRes := resp.Results
	for _, res := range searchRes{
		name := res.Name
		lat := fmt.Sprintf("%f", res.Geometry.Location.Lat)
		lng := fmt.Sprintf("%f", res.Geometry.Location.Lng)
		location := strings.Join([]string{lat, lng}, ",")
		addr := res.FormattedAddress
		id := res.PlaceID
		places = append(places, POI.CreatePlace(name, location, addr, string(locationType), id))
	}
	return
}

// getTypes. Given a location type return a set of types defined in google maps API
func getTypes (placeCat POI.PlaceCategory) (places []LocationType){
	switch placeCat{
	case POI.PlaceCategoryVisit:
		places = append(places,
			[]LocationType{LocationTypeAmusementPark, LocationTypeGallery, LocationTypeMuseum}...)
	case POI.PlaceCategoryEatery:
		places = append(places,
			[]LocationType{LocationTypeCafe, LocationTypeRestaurant}...)
	}
	return
}
