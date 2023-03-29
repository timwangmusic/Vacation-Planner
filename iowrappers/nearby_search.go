package iowrappers

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/utils"
	"googlemaps.github.io/maps"
)

const (
	GoogleNearbySearchDelay           = time.Second
	GoogleMapsSearchTimeout           = time.Second * 10
	GoogleMapsSearchCallMaxCount      = 5
	GoogleNearbySearchMaxRadiusMeters = 50000
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

// Create NearbySearchRequest for maps NearbySearch, adjust key settings such as radius and price levels
func CreateMapSearchRequest(reqIn *PlaceSearchRequest, placeType POI.LocationType, token string) (reqOut maps.NearbySearchRequest) {
	// Adjust radius, minPrice and maxPrice settings in search request
	var radius uint = reqIn.Radius
	var minPrice maps.PriceLevel
	if POI.PriceyEatery(reqIn.PlaceCat, reqIn.PriceLevel) {
		// increase search radius
		radius = uint(math.Min(float64(reqIn.Radius*4), GoogleNearbySearchMaxRadiusMeters))
		// set price filter
		minPrice = maps.PriceLevel(fmt.Sprint(reqIn.PriceLevel))
	}

	return maps.NearbySearchRequest{
		Type: maps.PlaceType(string(placeType)),
		Location: &maps.LatLng{
			Lat: reqIn.Location.Latitude,
			Lng: reqIn.Location.Longitude,
		},
		Radius:    radius,
		PageToken: token,
		RankBy:    maps.RankBy("prominence"),
		MinPrice:  minPrice, // filter places with price >= minPrice
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
	placeTypes := POI.GetPlaceTypes(request.PlaceCat) // get place types in a category

	nextPageTokenMap := make(map[POI.LocationType]string) // map for place type to search token
	for _, placeType := range placeTypes {
		nextPageTokenMap[placeType] = ""
	}

	var reqTimes uint = 0    // number of queries for each location type
	var totalResult uint = 0 // number of results so far, keep this number low

	microAddrMap := make(map[string]string) // map place ID to its micro-address
	placeMap := make(map[string]bool)       // remove duplication for place with same ID
	urlMap := make(map[string]string)       // map place ID to url

	searchStartTime := time.Now()

	var err error
	for totalResult < request.MinNumResults {
		// if error, return regardless of number of results obtained
		if err != nil {
			Logger.Error(err)
			done <- true
			return
		}
		for _, placeType := range placeTypes {
			if reqTimes > 0 && nextPageTokenMap[placeType] == "" { // no more result for this location type
				continue
			}

			nextPageToken := nextPageTokenMap[placeType]
			var searchReq = CreateMapSearchRequest(request, placeType, nextPageToken)
			var searchResp maps.PlacesSearchResponse
			searchResp, err = c.GoogleMapsNearbySearchWrapper(ctx, searchReq)

			placeIdMap := make(map[int]string) // maps index in search response to place ID
			for k, res := range searchResp.Results {
				if res.OpeningHours == nil || res.OpeningHours.WeekdayText == nil {
					placeIdMap[k] = res.PlaceID
				}
			}

			detailSearchResults := make([]PlaceDetailSearchResult, len(placeIdMap))
			var wg sync.WaitGroup
			wg.Add(len(placeIdMap))
			for idx, placeId := range placeIdMap {
				go PlaceDetailsSearchWrapper(ctx, c, idx, placeId, c.DetailedSearchFields, &detailSearchResults[idx], &wg)
			}
			wg.Wait()

			// fill fields from detail search results to nearby search results
			for _, placeDetails := range detailSearchResults {
				searchRespIdx := placeDetails.RespIdx
				placeId := searchResp.Results[searchRespIdx].PlaceID
				searchResp.Results[searchRespIdx].OpeningHours = placeDetails.Res.OpeningHours
				searchResp.Results[searchRespIdx].FormattedAddress = placeDetails.Res.FormattedAddress
				microAddrMap[placeId] = placeDetails.Res.AdrAddress
				urlMap[placeId] = placeDetails.Res.URL
			}

			*places = append(*places, parsePlacesSearchResponse(searchResp, placeType, microAddrMap, placeMap, urlMap)...)
			totalResult += uint(len(searchResp.Results))
			nextPageTokenMap[placeType] = searchResp.NextPageToken
		}
		reqTimes++
		if reqTimes == maxRequestTimes {
			break
		}
		time.Sleep(GoogleNearbySearchDelay) // sleep to make sure new next page token comes to effect
	}

	searchDuration := time.Since(searchStartTime)

	Logger.Infow("request:", "requestId", "Logging nearby search",
		"Maps API call time", searchDuration,
		"center location (lat,lng)", request.Location,
		"place category", request.PlaceCat,
		"total results", totalResult,
	)
	done <- true
}

type PlaceDetailSearchResult struct {
	Res     *maps.PlaceDetailsResult
	RespIdx int
}

func PlaceDetailsSearchWrapper(context context.Context, mapsClient *MapsClient, idx int, placeId string, fields []string, detailSearchRes *PlaceDetailSearchResult, wg *sync.WaitGroup) {
	defer wg.Done()
	searchRes, err := PlaceDetailedSearch(context, mapsClient, placeId, fields)
	if err != nil {
		Logger.Error(err)
		return
	}
	*detailSearchRes = PlaceDetailSearchResult{Res: &searchRes, RespIdx: idx}
}

func PlaceDetailedSearch(context context.Context, mapsClient *MapsClient, placeId string, fields []string) (maps.PlaceDetailsResult, error) {
	if reflect.ValueOf(mapsClient).IsNil() {
		err := errors.New("client does not exist")
		Logger.Error(err)
		return maps.PlaceDetailsResult{}, err
	}
	detailedSearchFields := strings.Join(fields, ",")
	req := &maps.PlaceDetailsRequest{
		PlaceID: placeId,
	}
	if detailedSearchFields != "" {
		fieldMask, err := parseFields(detailedSearchFields)
		utils.LogErrorWithLevel(err, utils.LogError)
		req.Fields = fieldMask
	}

	startSearchTime := time.Now()
	resp, err := mapsClient.client.PlaceDetails(context, req)
	utils.LogErrorWithLevel(err, utils.LogError)

	searchDuration := time.Since(startSearchTime)

	// logging
	//requestId := context.Value(RequestIdKey).(string)
	Logger.Debugw("request:", "requestId", "Logging place details search",
		"Maps API call time", searchDuration,
		"place ID", resp.PlaceID,
		"place name", resp.Name,
		"place formatted address", resp.FormattedAddress,
		"place user rating total", resp.UserRatingsTotal,
	)
	return resp, err
}

func parsePlacesSearchResponse(resp maps.PlacesSearchResponse, locationType POI.LocationType, microAddrMap map[string]string, placeMap map[string]bool, urlMap map[string]string) (places []POI.Place) {
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
		places = append(places, POI.CreatePlace(name, addr, res.FormattedAddress, res.BusinessStatus, locationType, h, id, priceLevel, rating, url, photo, userRatingsTotal, latitude, longitude))
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
