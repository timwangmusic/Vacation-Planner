package iowrappers

import (
	"strconv"

	"github.com/google/uuid"
	gogeonames "github.com/timwangmusic/go-geonames"
	"github.com/weihesdlegend/Vacation-planner/user"
	"googlemaps.github.io/maps"
)

func geocodingResultsToGeocodeQuery(query *GeocodeQuery, results []maps.GeocodingResult) {
	// take the most specific result
	firstGeocodingResult := results[0]
	for _, component := range firstGeocodingResult.AddressComponents {
		for _, addressType := range component.Types {
			switch addressType {
			case "locality":
				query.City = component.LongName
			case "administrative_area_level_1":
				query.AdminAreaLevelOne = component.ShortName
			case "country":
				query.Country = component.LongName
			}
		}
	}
}

func toRedisUserData(view *user.View) map[string]interface{} {
	return map[string]interface{}{
		"id":            view.ID,
		"username":      view.Username,
		"user_level":    view.UserLevel,
		"password":      view.Password,
		"email":         view.Email,
		"favorites":     view.Favorites,
		"lastLoginTime": view.LastLoginTime,
	}
}

func toCity(city gogeonames.City) (City, error) {
	lat, err := strconv.ParseFloat(city.Latitude, 64)
	if err != nil {
		return City{}, err
	}

	lng, err := strconv.ParseFloat(city.Longitude, 64)
	if err != nil {
		return City{}, err
	}

	return City{
		ID:         uuid.New().String(),
		Name:       city.Name,
		GeonameID:  city.ID,
		Latitude:   lat,
		Longitude:  lng,
		Population: city.Population,
		AdminArea1: city.AdminArea1,
		Country:    city.Country,
	}, nil
}

func searchFilterToPopulation(filter gogeonames.SearchFilter) int64 {
	switch filter {
	case gogeonames.CityWithPopulationGreaterThan1000:
		return 1000
	case gogeonames.CityWithPopulationGreaterThan5000:
		return 5000
	case gogeonames.CityWithPopulationGreaterThan15000:
		return 15000
	}
	return 0
}
