package POI

func FilterPlacesOnPriceLevel(places []Place, level PriceLevel) ([]Place, error) {
	var results []Place
	for _, place := range places {
		if place.GetPriceLevel() == level {
			results = append(results, place)
		}
	}
	return results, nil
}
