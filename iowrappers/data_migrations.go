package iowrappers

import (
	"context"
	"strings"
	"sync"
)

// a generic migration method
// returns place details results for the calling function to extract and use specific fields
func (poiSearcher *PoiSearcher) addDataFieldsToPlaces(context context.Context, field string) (map[string]PlaceDetailSearchResult, error) {
	mapsClient := poiSearcher.GetMapsClient()
	redisClient := poiSearcher.GetRedisClient()
	placeDetailsKeys, _, err := redisClient.GetPlaceCountInRedis(context)
	if err != nil {
		return nil, err
	}

	// persist updated places in a Redis Set
	// we cannot rely on checking the value of the new field
	// to determine if the place is updated. The default value is 0,
	// and some places may not have any rating.
	updatedPlacesRedisKey := "migration:" + field

	// store place IDs
	placesNeedUpdate := make([]string, 0)
	for _, placeDetailsKey := range placeDetailsKeys {
		placeId := strings.Split(placeDetailsKey, ":")[2]
		updated, _ := redisClient.client.SIsMember(context, updatedPlacesRedisKey, placeId).Result()
		if !updated {
			placesNeedUpdate = append(placesNeedUpdate, placeId)
		}
	}

	fields := []string{field}

	placeDetails := make([]PlaceDetailSearchResult, len(placesNeedUpdate))

	wg := sync.WaitGroup{}
	wg.Add(len(placesNeedUpdate))
	for idx, placeId := range placesNeedUpdate {
		redisClient.client.SAdd(context, updatedPlacesRedisKey, placeId)

		go PlaceDetailsSearchWrapper(context, mapsClient, idx, placeId, fields, &placeDetails[idx], &wg)
	}

	wg.Wait()
	results := make(map[string]PlaceDetailSearchResult)

	for idx, placeId := range placesNeedUpdate {
		placeDetails := placeDetails[idx]
		results[placeId] = placeDetails
	}
	return results, nil
}

// add user_ratings_total field to Places
func (poiSearcher *PoiSearcher) AddUserRatingsTotal(context context.Context) error {
	placeIdToDetailedSearchResults, err := poiSearcher.addDataFieldsToPlaces(context, "user_ratings_total")
	if err != nil {
		return err
	}

	redisClient := poiSearcher.GetRedisClient()
	wg := sync.WaitGroup{}
	wg.Add(len(placeIdToDetailedSearchResults))
	for placeId, detailedResult := range placeIdToDetailedSearchResults {
		place, err := redisClient.getPlace(context, placeId)
		if err != nil {
			continue
		}
		place.SetUserRatingsTotal(detailedResult.Res.UserRatingsTotal)
		go redisClient.setPlace(context, place, &wg)
	}
	wg.Wait()
	return nil
}
