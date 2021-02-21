package iowrappers

import "googlemaps.github.io/maps"

func geocodingResultsToGeocodeQuery(results []maps.GeocodingResult) GeocodeQuery {
	reverseGeocodingResult := GeocodeQuery{}
	// take the most specific result
	firstGeocodingResult := results[0]
	for _, component := range firstGeocodingResult.AddressComponents {
		for _, addressType := range component.Types {
			switch addressType {
			case "locality":
				reverseGeocodingResult.City = component.LongName
			case "country":
				reverseGeocodingResult.Country = component.LongName
			}
		}
	}
	return reverseGeocodingResult
}
