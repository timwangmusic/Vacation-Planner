package matching

import (
	"context"
	"errors"
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
)

type Matcher interface {
	Match(context context.Context, request Request) (places []Place, err error)
}

// FilterCriteria is an enum for various points of interest filtering criteria
type FilterCriteria string

const (
	MinResultsForTimePeriodMatching                = 20
	FilterByTimePeriod              FilterCriteria = "filterByTimePeriod"
	FilterByPriceRange              FilterCriteria = "filterByPriceRange"
)

type Request struct {
	Radius             uint
	Location           POI.Location
	Criteria           FilterCriteria
	Params             map[FilterCriteria]interface{}
	UsePreciseLocation bool
}

type MatcherForPriceRange struct {
	Searcher *iowrappers.PoiSearcher
}

func (matcher MatcherForPriceRange) Match(ctx context.Context, request Request) ([]Place, error) {
	var results []Place
	filterParams := request.Params[request.Criteria]

	if _, ok := filterParams.(PriceRangeFilterParams); !ok {
		return results, errors.New("price range matcher received wrong filter params")
	}

	priceRangeFilterParams := filterParams.(PriceRangeFilterParams)
	placeSearchRequest := &iowrappers.PlaceSearchRequest{
		PlaceCat:           priceRangeFilterParams.Category,
		Location:           request.Location,
		Radius:             request.Radius,
		MinNumResults:      MinResultsForTimePeriodMatching,
		BusinessStatus:     POI.Operational,
		UsePreciseLocation: request.UsePreciseLocation,
	}

	basicPlaces, err := matcher.Searcher.NearbySearch(ctx, placeSearchRequest)
	if err != nil {
		return results, err
	}
	iowrappers.Logger.Infof("obtained %d places before filtering price", len(basicPlaces))

	var filteredPlaces []POI.Place
	filteredPlaces = basicPlaces
	// POI data from Google API does not have price range, therefore we only filter catering places on price
	if priceRangeFilterParams.Category == POI.PlaceCategoryEatery {
		filteredPlaces, _ = POI.FilterPlacesOnPriceLevel(basicPlaces, priceRangeFilterParams.PriceLevel)
	}

	for _, place := range filteredPlaces {
		results = append(results, CreatePlace(place, priceRangeFilterParams.Category))
	}
	return results, nil
}

type PriceRangeFilterParams struct {
	Category   POI.PlaceCategory
	PriceLevel POI.PriceLevel
}

type TimeFilterParams struct {
	Category     POI.PlaceCategory
	Day          POI.Weekday
	TimeInterval POI.TimeInterval
}

type MatcherForTime struct {
	Searcher *iowrappers.PoiSearcher
}

func (matcher MatcherForTime) Match(ctx context.Context, request Request) ([]Place, error) {
	var results []Place
	filterParams := request.Params[request.Criteria]

	if _, ok := filterParams.(TimeFilterParams); !ok {
		return results, errors.New("time matcher received wrong filter params")
	}

	timeFilterParams := filterParams.(TimeFilterParams)
	placeSearchRequest := &iowrappers.PlaceSearchRequest{
		PlaceCat:           timeFilterParams.Category,
		Location:           request.Location,
		Radius:             request.Radius,
		MinNumResults:      MinResultsForTimePeriodMatching,
		BusinessStatus:     POI.Operational,
		UsePreciseLocation: request.UsePreciseLocation,
	}

	basicPlaces, err := matcher.Searcher.NearbySearch(ctx, placeSearchRequest)
	if err != nil {
		return results, err
	}

	filteredBasicPlaces := matcher.filterPlaces(timeFilterParams, basicPlaces)

	for _, place := range filteredBasicPlaces {
		results = append(results, CreatePlace(place, timeFilterParams.Category))
	}
	return results, nil
}

func (matcher MatcherForTime) filterPlaces(timeFilterParams TimeFilterParams, places []POI.Place) []POI.Place {
	var results []POI.Place
	for _, place := range places {
		openingHourForDay := place.GetHour(timeFilterParams.Day)
		timeInterval, err := POI.ParseTimeInterval(openingHourForDay)
		if err != nil {
			continue
		}
		if timeInterval.Inclusive(&timeFilterParams.TimeInterval) {
			results = append(results, place)
		}
	}
	return results
}
