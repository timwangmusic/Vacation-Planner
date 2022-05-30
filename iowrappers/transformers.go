package iowrappers

import "googlemaps.github.io/maps"

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
