package iowrappers

import (
	"context"
	"github.com/weihesdlegend/Vacation-planner/utils"
	"reflect"
	"strings"
	"sync"
)

const (
	BatchSize = 300
)
// a generic migration method
// returns place details results for the calling function to extract and use specific fields
func (poiSearcher *PoiSearcher) addDataFieldsToPlaces(context context.Context, field string, batchSize int) (map[string]PlaceDetailSearchResult, error) {
	mapsClient := poiSearcher.GetMapsClient()
	redisClient := poiSearcher.GetRedisClient()
	placeDetailsKeys, totalPlacesCount, err := redisClient.GetPlaceCountInRedis(context)
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
	Logger.Infof("[data migration] The number of places need update is %d with target field: %s", len(placesNeedUpdate), field)

	placesToUpdateCount := utils.MinInt(len(placesNeedUpdate), batchSize)
	newPlaceDetailsResults := make([]PlaceDetailSearchResult, placesToUpdateCount)
	Logger.Infof("[data migration] Place to update count: %d, batch size is: %d", placesToUpdateCount, batchSize)
	Logger.Infof("[data migration] Getting %d place details with target field: %s", placesToUpdateCount, field)

	fields := []string{field}

	wg := sync.WaitGroup{}
	wg.Add(placesToUpdateCount)
	for idx, placeId := range placesNeedUpdate[:placesToUpdateCount] {
		redisClient.client.SAdd(context, updatedPlacesRedisKey, placeId)

		go PlaceDetailsSearchWrapper(context, mapsClient, idx, placeId, fields, &newPlaceDetailsResults[idx], &wg)
	}

	wg.Wait()
	results := make(map[string]PlaceDetailSearchResult)

	for idx, placeId := range placesNeedUpdate[:placesToUpdateCount] {
		results[placeId] = newPlaceDetailsResults[idx]
	}
	Logger.Infof("[data migration] %d places left to update out of total of %d",
		len(placesNeedUpdate)-placesToUpdateCount,
		totalPlacesCount)

	return results, nil
}

// add user_ratings_total field to Places
func (poiSearcher *PoiSearcher) AddUserRatingsTotal(context context.Context) error {
	placeIdToDetailedSearchResults, err := poiSearcher.addDataFieldsToPlaces(context, "user_ratings_total", BatchSize)
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
		// FIXME: figure out the reason for maps client return null pointer as result
		if reflect.ValueOf(detailedResult.Res).IsNil() {
			place.SetUserRatingsTotal(0)
		} else {
			place.SetUserRatingsTotal(detailedResult.Res.UserRatingsTotal)
		}
		go redisClient.setPlace(context, place, &wg)
	}
	wg.Wait()
	return nil
}

func (poiSearcher *PoiSearcher) AddUrl(context context.Context) error {
	placeIdToDetailedSearchResults, err := poiSearcher.addDataFieldsToPlaces(context, "url", BatchSize)
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
		// FIXME: figure out the reason for maps client return null pointer as result
		if reflect.ValueOf(detailedResult.Res).IsNil() {
			place.SetURL("")
		} else {
			place.SetURL(detailedResult.Res.URL)
		}
		go redisClient.setPlace(context, place, &wg)
	}
	wg.Wait()
	return nil
}
