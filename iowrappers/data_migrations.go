package iowrappers

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/bobg/go-generics/set"
	"github.com/weihesdlegend/Vacation-planner/POI"
)

const (
	BatchSize = 300
)

type PlaceDetailsFields string

const (
	PlaceDetailsFieldURL              PlaceDetailsFields = "URL"
	PlaceDetailsFieldPhoto            PlaceDetailsFields = "photo"
	PlaceDetailsFieldUserRatingsCount PlaceDetailsFields = "ratings_count"
)

func (s *PoiSearcher) RemovePlaces(context context.Context, nonEmptyFields []PlaceDetailsFields) error {
	if err := s.redisClient.RemovePlaces(context, nonEmptyFields); err != nil {
		Logger.Error(err)
		return fmt.Errorf("failed to removed places: %s", err.Error())
	}
	return nil
}

func (r *RedisClient) RemovePlaces(context context.Context, nonEmptyFields []PlaceDetailsFields) error {
	var placeDetailsKeys []string
	var err error
	placeDetailsKeys, err = scanRedisKeys(context, r, PlaceDetailsRedisKeyPrefix)
	if err != nil {
		return err
	}

	var count uint64
	Logger.Debugf("RemovePlaces -> obtained keys for %d places", len(placeDetailsKeys))
	for idx, key := range placeDetailsKeys {
		if err = r.removePlace(context, key, nonEmptyFields, &count); err != nil {
			return err
		}
		if (idx+1)%100 == 0 {
			Logger.Debugf("RemovePlaces -> completed processing %d places", idx+1)
		}
	}
	Logger.Infof("RemovePlaces -> removed %d bad places", count)
	return nil
}

func (r *RedisClient) removePlace(context context.Context, placeRedisKey string, nonEmptyFields []PlaceDetailsFields, count *uint64) error {
	segments := strings.Split(placeRedisKey, ":")
	var placeID string
	if len(segments) > 0 {
		placeID = segments[len(segments)-1]
	}

	var place POI.Place
	var err error
	place, err = r.getPlace(context, placeID)
	if err != nil {
		return err
	}

	if isPlaceDetailsValid(place, nonEmptyFields) {
		return nil
	}

	Logger.Debugf("removing place %+v from Redis", place)
	*count++
	// remove keys from all categorized sorted lists in case a place belongs to multiple categories
	_, _ = r.client.ZRem(context, "placeIDs:visit", placeID).Result()
	_, _ = r.client.ZRem(context, "placeIDs:eatery:"+strconv.Itoa(int(place.PriceLevel)), placeID).Result()

	return r.RemoveKeys(context, []string{placeRedisKey})
}

func isPlaceDetailsValid(place POI.Place, nonEmptyFields []PlaceDetailsFields) bool {
	for _, field := range nonEmptyFields {
		switch field {
		case PlaceDetailsFieldPhoto:
			if reflect.ValueOf(place.Photo).IsZero() {
				return false
			}
		case PlaceDetailsFieldURL:
			if reflect.ValueOf(place.URL).IsZero() {
				return false
			}
		case PlaceDetailsFieldUserRatingsCount:
			if place.UserRatingsTotal == 0 {
				return false
			}
		}
	}
	return true
}

// a generic migration method
// returns place details results for the calling function to extract and use specific fields
func (s *PoiSearcher) addDataFieldsToPlaces(context context.Context, field string, batchSize int) (map[string]PlaceDetailsSearchResult, error) {
	mapsClient := s.GetMapsClient()
	redisClient := s.GetRedisClient()
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

	placesToUpdateCount := min(len(placesNeedUpdate), batchSize)
	newPlaceDetailsResults := make([]PlaceDetailsSearchResult, placesToUpdateCount)
	Logger.Infof("[data migration] Place to update count: %d, batch size is: %d", placesToUpdateCount, batchSize)
	Logger.Infof("[data migration] Getting %d place details with target field: %s", placesToUpdateCount, field)

	fields := []string{field}

	wg := sync.WaitGroup{}
	wg.Add(placesToUpdateCount)
	for idx, placeId := range placesNeedUpdate[:placesToUpdateCount] {
		redisClient.client.SAdd(context, updatedPlacesRedisKey, placeId)
		toUpdate := set.Of[string]{}
		toUpdate.Add(placeId)
		go PlaceDetailsSearchWrapper(context, mapsClient, idx, placeId, fields, &newPlaceDetailsResults[idx], &wg, toUpdate)
	}

	wg.Wait()
	results := make(map[string]PlaceDetailsSearchResult)

	for idx, placeId := range placesNeedUpdate[:placesToUpdateCount] {
		results[placeId] = newPlaceDetailsResults[idx]
	}
	Logger.Infof("[data migration] %d places left to update out of total of %d",
		len(placesNeedUpdate)-placesToUpdateCount,
		totalPlacesCount)

	return results, nil
}

func (s *PoiSearcher) AddUserRatingsTotal(context context.Context) error {
	placeIdToDetailedSearchResults, err := s.addDataFieldsToPlaces(context, "user_ratings_total", BatchSize)
	if err != nil {
		return err
	}

	redisClient := s.GetRedisClient()
	wg := sync.WaitGroup{}
	wg.Add(len(placeIdToDetailedSearchResults))
	for placeId, detailedResult := range placeIdToDetailedSearchResults {
		place, err := redisClient.getPlace(context, placeId)
		if err != nil {
			continue
		}
		// FIXME: figure out the reason for maps client return null pointer as result
		if reflect.ValueOf(detailedResult.res).IsNil() {
			place.SetUserRatingsTotal(0)
		} else {
			place.SetUserRatingsTotal(detailedResult.res.UserRatingsTotal)
		}
		go redisClient.setPlace(context, place, &wg)
	}
	wg.Wait()
	return nil
}

func (s *PoiSearcher) AddUrl(context context.Context) error {
	placeIdToDetailedSearchResults, err := s.addDataFieldsToPlaces(context, "url", BatchSize)
	if err != nil {
		return err
	}

	redisClient := s.GetRedisClient()
	wg := sync.WaitGroup{}
	wg.Add(len(placeIdToDetailedSearchResults))
	for placeId, detailedResult := range placeIdToDetailedSearchResults {
		place, err := redisClient.getPlace(context, placeId)
		if err != nil {
			continue
		}
		// FIXME: figure out the reason for maps client return null pointer as result
		if reflect.ValueOf(detailedResult.res).IsNil() {
			place.SetURL("")
		} else {
			place.SetURL(detailedResult.res.URL)
		}
		go redisClient.setPlace(context, place, &wg)
	}
	wg.Wait()
	return nil
}
