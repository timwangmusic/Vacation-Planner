package iowrappers

import (
	"Vacation-planner/POI"
	"Vacation-planner/utils"
	"context"
	"flag"
	"fmt"
	"github.com/pkg/errors"
	"googlemaps.github.io/maps"
	"log"
	"strings"
	"time"
)

type LocationType string

const(
	LocationTypeCafe = LocationType("cafe")
	LocationTypeRestaurant = LocationType("restaurant")
	LocationTypeMuseum = LocationType("museum")
	LocationTypeGallery = LocationType("art_gallery")
	LocationTypeAmusementPark = LocationType("amusement_park")
	LocationTypePark = LocationType("park")
)

const(
	GOOGLE_NEARBY_SEARCH_DELAY = time.Duration(2 * time.Second)
)

var detailedSearchFields = flag.String("fields", "opening_hours", "a list of comma-separated fields")

type PlaceSearchRequest struct{
	// "lat,lng"
	Location string
	// "cafe", "museum",...
	PlaceType LocationType
	// search radius
	Radius uint
	// next page token
	NextPageToken string
	// rank by
	RankBy string
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
// Search each location type exactly once
func (c *MapsClient) SimpleNearbySearch(centerLocation string, placeCat POI.PlaceCategory,
	radius uint, rankBy string)(places []POI.Place){
	if rankBy == ""{
		rankBy = "prominence"	// default rankBy value
	}

	placeTypes := getTypes(placeCat)

	for _, placeType := range placeTypes{
		req := PlaceSearchRequest{
			Location:  centerLocation,
			PlaceType: placeType,
			Radius:    radius,
			RankBy:	   rankBy,
		}
		searchResp := NearbySearchSDK(*c, req)

		for k, res := range searchResp.Results{
			if res.OpeningHours == nil || res.OpeningHours.WeekdayText == nil{
				detailedSearchRes, _ := c.PlaceDetailedSearch(res.PlaceID)
				searchResp.Results[k].OpeningHours = detailedSearchRes.OpeningHours
			}
		}

		places = append(places, parsePlacesSearchResponse(searchResp, placeType)...)
	}
	return
}

// ExtensiveNearbySearch tries to find specified number of search results
// from a place category once for each location type in the category
// maxRequestTime specifies the number of times to query for each location type
// having maxRequestTimes provides Google API call protection
func (c *MapsClient) ExtensiveNearbySearch(centerLocation string, placeCat POI.PlaceCategory,
	radius uint, rankBy string, maxResult uint, maxRequestTimes uint)(places []POI.Place){
	if rankBy == ""{
		rankBy = "prominence"	// default rankBy value
	}

	placeTypes := getTypes(placeCat)

	nextPageTokenMap := make(map[LocationType]string)
	for _, placeType := range placeTypes{
		nextPageTokenMap[placeType] = ""
	}

	var reqTimes uint = 0		// number of queries for each location type
	var totalResult uint= 0	// number of results so far

	for totalResult <= maxRequestTimes && reqTimes < maxRequestTimes{
		for _, placeType := range placeTypes{
			if reqTimes > 0 && nextPageTokenMap[placeType] == ""{	// no more result for this location type
				continue
			}
			nextPageToken := nextPageTokenMap[placeType]
			req := PlaceSearchRequest{
				Location:  centerLocation,
				PlaceType: placeType,
				Radius:    radius,
				RankBy:	   rankBy,
				NextPageToken: nextPageToken,
			}
			searchResp := NearbySearchSDK(*c, req)
			places = append(places, parsePlacesSearchResponse(searchResp, placeType)...)
			totalResult += uint(len(searchResp.Results))
			nextPageTokenMap[placeType] = searchResp.NextPageToken
		}
		reqTimes++
		time.Sleep(GOOGLE_NEARBY_SEARCH_DELAY)	// sleep to make sure new next page token comes to effect
	}
	return
}

func (c *MapsClient) PlaceDetailedSearch(placeId string) (maps.PlaceDetailsResult, error){
	if c.client == nil{
		return maps.PlaceDetailsResult{}, errors.New("client does not exist")
	}
	flag.Parse()	// parse detailed search fields

	req := &maps.PlaceDetailsRequest{
		PlaceID: placeId,
	}

	if *detailedSearchFields != ""{
		fieldMask, err := parseFields(*detailedSearchFields)
		utils.CheckErr(err)
		req.Fields = fieldMask
	}

	resp, err := c.client.PlaceDetails(context.Background(), req)

	utils.CheckErr(err)

	return resp, nil
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
		priceLevel := res.PriceLevel
		places = append(places, POI.CreatePlace(name, location, addr, string(locationType), id, priceLevel))
	}
	return
}

// Given a location type return a set of types defined in google maps API
func getTypes (placeCat POI.PlaceCategory) (placeTypes []LocationType){
	switch placeCat{
	case POI.PlaceCategoryVisit:
		placeTypes = append(placeTypes,
			[]LocationType{LocationTypePark, LocationTypeAmusementPark, LocationTypeGallery, LocationTypeMuseum}...)
	case POI.PlaceCategoryEatery:
		placeTypes = append(placeTypes,
			[]LocationType{LocationTypeCafe, LocationTypeRestaurant}...)
	}
	return
}

// refs: maps/examples/places/placedetails/placedetails.go
func parseFields(fields string) ([]maps.PlaceDetailsFieldMask, error) {
	var res []maps.PlaceDetailsFieldMask
	for _, s := range strings.Split(fields, ",") {
		f, err := maps.ParsePlaceDetailsFieldMask(s)
		check(err)
		res = append(res, f)
	}
	return res, nil
}
