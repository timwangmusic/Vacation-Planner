package matching

import (
	"context"
	"errors"

	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
)

type Matcher interface {
	Match(req *FilterRequest) (places []Place, err error)
}

// FilterCriteria is an enum for various points of interest filtering criteria
type FilterCriteria string

const (
	MinResultsForTimePeriodMatching                = 20
	FilterByTimePeriod              FilterCriteria = "filterByTimePeriod"
	FilterByPriceRange              FilterCriteria = "filterByPriceRange"
	FilterByUserRating              FilterCriteria = "filterByUserRating"
)

type Request struct {
	Radius             uint
	Location           POI.Location
	Category           POI.PlaceCategory
	UsePreciseLocation bool
	PriceLevel         POI.PriceLevel
}

type FilterRequest struct {
	Places []Place
	Params map[FilterCriteria]interface{}
}

type MatcherForPriceRange struct {
}

func (matcher MatcherForPriceRange) Match(req *FilterRequest) ([]Place, error) {
	filterParams := req.Params[FilterByPriceRange]

	if _, ok := filterParams.(PriceRangeFilterParams); !ok {
		return nil, errors.New("price range matcher received wrong filter params")
	}

	priceRangeFilterParams := filterParams.(PriceRangeFilterParams)

	// POI data from Google API does not have price range, therefore we only filter catering places on price
	if priceRangeFilterParams.Category == POI.PlaceCategoryEatery {
		return filterPlacesOnPriceLevel(req.Places, priceRangeFilterParams.PriceLevel), nil
	}

	return req.Places, nil
}

type PriceRangeFilterParams struct {
	Category   POI.PlaceCategory
	PriceLevel POI.PriceLevel
}

type TimeFilterParams struct {
	Day          POI.Weekday
	TimeInterval POI.TimeInterval
}

type MatcherForTime struct {
}

func NearbySearchForCategory(ctx context.Context, searcher iowrappers.SearchClient, req *Request) ([]Place, error) {
	placeSearchRequest := &iowrappers.PlaceSearchRequest{
		PlaceCat:           req.Category,
		Location:           req.Location,
		Radius:             req.Radius,
		MinNumResults:      MinResultsForTimePeriodMatching,
		BusinessStatus:     POI.Operational,
		UsePreciseLocation: req.UsePreciseLocation,
		PriceLevel:         req.PriceLevel,
	}
	basicPlaces, err := searcher.NearbySearch(ctx, placeSearchRequest)
	if err != nil {
		return nil, err
	}

	var results []Place
	for _, place := range basicPlaces {
		results = append(results, CreatePlace(place, req.Category))
	}
	return results, nil
}

func (m MatcherForTime) Match(req *FilterRequest) ([]Place, error) {
	var results []Place
	filterParams := req.Params[FilterByTimePeriod]

	if _, ok := filterParams.(TimeFilterParams); !ok {
		return results, errors.New("time m received wrong filter params")
	}
	timeFilterParams := filterParams.(TimeFilterParams)

	return filterPlacesOnTime(req.Places, timeFilterParams.Day, timeFilterParams.TimeInterval), nil
}

func filterPlacesOnTime(places []Place, day POI.Weekday, interval POI.TimeInterval) []Place {
	var results []Place
	for _, place := range places {
		openingHourForDay := place.Hours()[day]
		timeInterval, err := POI.ParseTimeInterval(openingHourForDay)
		if err != nil {
			continue
		}
		if timeInterval.Inclusive(&interval) {
			results = append(results, place)
		}
	}
	return results
}

func filterPlacesOnPriceLevel(places []Place, level POI.PriceLevel) []Place {
	var results []Place
	for _, place := range places {
		if place.Place.PriceLevel == level {
			results = append(results, place)
		}
	}
	return results
}

type UserRatingFilterParams struct {
	MinUserRatings int
}

type MatcherForUserRatings struct {
}

func (m MatcherForUserRatings) Match(req *FilterRequest) ([]Place, error) {
	var results []Place
	filterParams := req.Params[FilterByUserRating]

	if _, ok := filterParams.(UserRatingFilterParams); !ok {
		return results, errors.New("user rating matcher received wrong filter params")
	}
	params := filterParams.(UserRatingFilterParams)

	userRatingCountFilter := func(minRating int) func(place Place) bool {
		return func(place Place) bool {
			return place.UserRatingsCount() >= minRating
		}
	}
	return iowrappers.Filter(req.Places, userRatingCountFilter(params.MinUserRatings)), nil
}
