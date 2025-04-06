package iowrappers

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/utils"
	"googlemaps.github.io/maps"
)

const (
	GoogleMapsSearchTimeout             = time.Second * 10
	GoogleMapsSearchCallMaxCount        = 5
	GoogleNearbySearchMaxRadiusInMeters = 50000
)

type PlaceSearchRequest struct {
	// "visit", "eatery", etc...
	PlaceCat POI.PlaceCategory

	Location POI.Location
	// search radius
	Radius uint
	// minimum number of results, set this lower limit for reducing risk of zero result in upper-layer computations.
	// suppose a location has more places established over time, this field would help trigger new searches to get those new establishments.
	MinNumResults uint

	BusinessStatus POI.BusinessStatus
	// true if using precise geolocation instead of using a grander administrative area
	UsePreciseLocation bool

	PriceLevel POI.PriceLevel
}

// CreateMapSearchRequest creates a NearbySearchRequest for maps NearbySearch, adjust key settings such as radius and price levels
func CreateMapSearchRequest(reqIn *PlaceSearchRequest, placeType POI.LocationType, token string) (reqOut maps.NearbySearchRequest) {
	// Adjust radius, minPrice and maxPrice settings in search request
	var radius = reqIn.Radius
	var exactPriceLevel maps.PriceLevel
	if POI.PriceyEatery(reqIn.PlaceCat, reqIn.PriceLevel) {
		// increase search radius
		radius = min(reqIn.Radius*4, GoogleNearbySearchMaxRadiusInMeters)
		// set price filter
		exactPriceLevel = maps.PriceLevel(fmt.Sprint(reqIn.PriceLevel))
	}

	return maps.NearbySearchRequest{
		Type: maps.PlaceType(placeType),
		Location: &maps.LatLng{
			Lat: reqIn.Location.Latitude,
			Lng: reqIn.Location.Longitude,
		},
		Radius:    radius,
		PageToken: token,
		RankBy:    maps.RankBy("prominence"),
		MinPrice:  exactPriceLevel,
		MaxPrice:  exactPriceLevel,
	}
}

func (c *MapsClient) GoogleMapsNearbySearchWrapper(ctx context.Context, mapsReq maps.NearbySearchRequest) (resp maps.PlacesSearchResponse, err error) {
	resp, err = c.client.NearbySearch(ctx, &mapsReq)
	logErr(err, utils.LogError)
	return
}

func (c *MapsClient) NearbySearch(ctx context.Context, request *PlaceSearchRequest) ([]POI.Place, error) {
	var places = make([]POI.Place, 0)
	var searchDone = make(chan bool)
	ctx, cancelFunc := context.WithTimeout(ctx, GoogleMapsSearchTimeout)
	defer cancelFunc()

	go c.extensiveNearbySearch(ctx, GoogleMapsSearchCallMaxCount, request, &places, searchDone)

	select {
	case <-searchDone:
		return places, nil
	case <-ctx.Done():
		return places, errors.New("maps search time out")
	}
}

func (c *MapsClient) extensiveNearbySearch(ctx context.Context, maxRequestTimes uint, request *PlaceSearchRequest, places *[]POI.Place, done chan bool) {
	searchStartTime := time.Now()
	placeTypes := POI.GetPlaceTypes(request.PlaceCat) // get place types in a category

	nextPageTokenMap := make(map[POI.LocationType]string) // map for place type to search token
	placeCountPerPlaceType := make(map[POI.LocationType]int)
	for _, placeType := range placeTypes {
		nextPageTokenMap[placeType] = ""
		placeCountPerPlaceType[placeType] = 0
	}

	var reqTimes uint = 0        // number of queries for each location type
	var totalPlaceCount uint = 0 // number of results so far, keep this number low

	microAddrMap := make(map[string]string) // map place ID to its micro-address
	placeMap := make(map[string]bool)       // remove duplication for place with same ID
	urlMap := make(map[string]string)       // map place ID to url
	summaryMap := make(map[string]string)   // map place ID to summary

	var err error
	for totalPlaceCount < request.MinNumResults {
		reqTimes++
		for _, placeType := range placeTypes {
			if reqTimes > 1 && nextPageTokenMap[placeType] == "" { // no more result for this location type
				continue
			}

			singlePlaceTypeSearchStartTime := time.Now()
			nextPageToken := nextPageTokenMap[placeType]
			var searchReq = CreateMapSearchRequest(request, placeType, nextPageToken)
			var searchResp maps.PlacesSearchResponse
			searchResp, err = c.GoogleMapsNearbySearchWrapper(ctx, searchReq)
			if err != nil {
				Logger.Error(fmt.Errorf("places nearby search with Maps error: %w", err))
				// we should still retry for the same place type but with a maximum being maxRequestTimes
				continue
			}

			// places for Google Maps place details search (https://developers.google.com/maps/documentation/places/web-service/details)
			// the original purpose of doing a details search is getting opening hours info
			// later on we added more fields of interest as specified in config/config.yaml file
			placeIdMap := make(map[int]string)
			for k, res := range searchResp.Results {
				if res.OpeningHours == nil || res.OpeningHours.WeekdayText == nil {
					placeIdMap[k] = res.PlaceID
				}
			}

			detailsSearchResults := make([]PlaceDetailsSearchResult, len(placeIdMap))
			var wg sync.WaitGroup
			wg.Add(len(placeIdMap))
			for idx, placeId := range placeIdMap {
				go PlaceDetailsSearchWrapper(ctx, c, idx, placeId, c.DetailedSearchFields, &detailsSearchResults[idx], &wg)
			}
			wg.Wait()
			searchDuration := time.Since(singlePlaceTypeSearchStartTime)

			// fill fields from detail search results to nearby search results
			for _, placeDetails := range detailsSearchResults {
				idx := placeDetails.idx
				placeId := searchResp.Results[idx].PlaceID
				summary := placeDetails.res.EditorialSummary
				if summary != nil {
					summaryMap[placeId] = summary.Overview
					Logger.Debugf("editorial summary for place %s is: %s", placeId, summary.Overview)
				}
				searchResp.Results[idx].OpeningHours = placeDetails.res.OpeningHours
				searchResp.Results[idx].FormattedAddress = placeDetails.res.FormattedAddress
				microAddrMap[placeId] = placeDetails.res.AdrAddress
				urlMap[placeId] = placeDetails.res.URL
			}

			*places = append(*places, parsePlacesSearchResponse(searchResp, placeType, microAddrMap, placeMap, urlMap, summaryMap)...)
			totalPlaceCount += uint(len(searchResp.Results))
			placeCountPerPlaceType[placeType] += len(searchResp.Results)
			nextPageTokenMap[placeType] = searchResp.NextPageToken

			Logger.Infow("Logging nearby search for individual place types",
				"center location (lat,lng)", request.Location,
				"place type:", placeType,
				"price level", request.PriceLevel,
				"place count so far", placeCountPerPlaceType[placeType],
				"API call time", searchDuration,
			)
		}
		if reqTimes == maxRequestTimes {
			break
		}
	}

	Logger.Infow("Logging nearby search for a complete Google Maps search",
		"center location (lat, lng)", request.Location,
		"place category", request.PlaceCat,
		"price level", request.PriceLevel,
		"total place count", totalPlaceCount,
		"total processing time", time.Since(searchStartTime),
	)
	done <- true
}

type PlaceDetailsSearchResult struct {
	res *maps.PlaceDetailsResult
	idx int
}

func PlaceDetailsSearchWrapper(context context.Context, mapsClient *MapsClient, idx int, placeId string, fields []string, detailSearchRes *PlaceDetailsSearchResult, wg *sync.WaitGroup) {
	defer wg.Done()
	searchRes, err := mapsClient.PlaceDetailedSearch(context, placeId, fields)
	if err != nil {
		Logger.Error(err)
		return
	}
	*detailSearchRes = PlaceDetailsSearchResult{res: &searchRes, idx: idx}
}

func (c *MapsClient) PlaceDetailedSearch(context context.Context, placeId string, fields []string) (maps.PlaceDetailsResult, error) {
	if reflect.ValueOf(c.client).IsNil() {
		err := errors.New("client does not exist")
		return maps.PlaceDetailsResult{}, err
	}
	detailedSearchFields := strings.Join(fields, ",")
	req := &maps.PlaceDetailsRequest{
		PlaceID: placeId,
	}
	if detailedSearchFields != "" {
		fieldMask, err := parseFields(detailedSearchFields)
		if err != nil {
			return maps.PlaceDetailsResult{}, err
		}
		req.Fields = fieldMask
	}

	resp, err := c.client.PlaceDetails(context, req)
	return resp, err
}

func parsePlacesSearchResponse(resp maps.PlacesSearchResponse, locationType POI.LocationType, microAddrMap map[string]string, placeMap map[string]bool, urlMap map[string]string, summaryMap map[string]string) (places []POI.Place) {
	for _, res := range resp.Results {
		id := res.PlaceID
		if seen := placeMap[id]; !seen {
			placeMap[id] = true
		} else {
			continue
		}
		name := res.Name
		latitude := res.Geometry.Location.Lat
		longitude := res.Geometry.Location.Lng
		addr := ""
		if microAddrMap != nil {
			addr = microAddrMap[id]
		}
		priceLevel := res.PriceLevel
		h := &POI.OpeningHours{}
		if res.OpeningHours != nil && res.OpeningHours.WeekdayText != nil && len(res.OpeningHours.WeekdayText) > 0 {
			h.Hours = append(h.Hours, res.OpeningHours.WeekdayText...)
		}
		rating := res.Rating
		url := urlMap[id]
		var photo *maps.Photo
		if len(res.Photos) > 0 {
			photo = &res.Photos[0]
		}
		userRatingsTotal := res.UserRatingsTotal
		// filter places with zero user ratings
		if userRatingsTotal == 0 {
			continue
		}
		var placeSummary *string
		if summary, ok := summaryMap[id]; ok {
			placeSummary = &summary
		}

		places = append(places, POI.CreatePlace(name, addr, res.FormattedAddress, res.BusinessStatus, locationType, h, id, priceLevel, rating, url, photo, userRatingsTotal, latitude, longitude, placeSummary))
	}
	return
}

// refs: maps/examples/places/placedetails/placedetails.go
func parseFields(fields string) ([]maps.PlaceDetailsFieldMask, error) {
	var res []maps.PlaceDetailsFieldMask
	for _, s := range strings.Split(fields, ",") {
		f, err := maps.ParsePlaceDetailsFieldMask(s)
		if logErr(err, utils.LogError) {
			return res, err
		}
		res = append(res, f)
	}
	return res, nil
}

func logErr(err error, logLevel uint) bool {
	return utils.LogErrorWithLevel(err, logLevel)
}
