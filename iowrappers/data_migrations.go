package iowrappers

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/bobg/go-generics/set"
	"github.com/redis/go-redis/v9"
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
		go func(pid string, result PlaceDetailsSearchResult) {
			defer wg.Done()
			place, err := redisClient.getPlace(context, pid)
			if err != nil {
				Logger.Error(err)
				return
			}
			// FIXME: figure out the reason for maps client return null pointer as result
			if reflect.ValueOf(result.res).IsNil() {
				place.SetUserRatingsTotal(0)
			} else {
				place.SetUserRatingsTotal(result.res.UserRatingsTotal)
			}
			if err := redisClient.setPlace(context, place); err != nil {
				Logger.Error(err)
			}
		}(placeId, detailedResult)
	}
	wg.Wait()
	return nil
}

// BucketCleanupReport summarizes a RemoveMisclassifiedPlacesFromCategoryBuckets run.
type BucketCleanupReport struct {
	Scanned       int      `json:"scanned"`
	Misclassified int      `json:"misclassified"`
	Removed       int      `json:"removed"`
	RemovedIDs    []string `json:"removed_ids"`
}

// SetPlace stores a single place record. Exported wrapper over setPlace for migrations and tests.
func (r *RedisClient) SetPlace(ctx context.Context, place POI.Place) error {
	return r.setPlace(ctx, place)
}

// AddGeoLocation adds a place to a geo bucket under an explicit key. Exported for
// migrations and tests that need to write buckets the normal write path would reject.
func (r *RedisClient) AddGeoLocation(ctx context.Context, key string, place POI.Place) error {
	loc := place.GetLocation()
	_, err := r.client.GeoAdd(ctx, key, &redis.GeoLocation{
		Name:      place.ID,
		Latitude:  loc.Latitude,
		Longitude: loc.Longitude,
	}).Result()
	return err
}

// RemoveMisclassifiedPlacesFromCategoryBuckets removes places from cat's geo buckets whose
// PRIMARY Google type does not belong to cat. It repairs the fast_food_restaurant incident:
// two Places-API-(New)-only types were searched against the legacy Nearby Search, which
// ignored the unenforceable type filter and returned prominence-ranked establishments, and
// those were stamped with the queried type and written into placeIDs:eatery:level*.
//
// It uses the same rule as POI.ReclassifyForCategory, which is what already hides these from
// API responses: classify by primary type, and KEEP records with no Types list (older cached
// records written before Types was captured) so coverage never regresses.
//
// dryRun reports what would be removed without deleting anything. Always dry-run first.
func (r *RedisClient) RemoveMisclassifiedPlacesFromCategoryBuckets(ctx context.Context, cat POI.PlaceCategory, dryRun bool) (BucketCleanupReport, error) {
	report := BucketCleanupReport{RemovedIDs: make([]string, 0)}

	levels := []POI.PriceLevel{POI.PriceLevelDefault}
	if cat == POI.PlaceCategoryEatery {
		levels = POI.AllPriceLevels
	}

	for _, level := range levels {
		key := POI.EncodeNearbySearchRedisKey(cat, level)
		members, err := r.client.ZRange(ctx, key, 0, -1).Result()
		if err != nil {
			return report, fmt.Errorf("reading geo bucket %s: %w", key, err)
		}
		for _, placeID := range members {
			report.Scanned++
			place, err := r.getPlace(ctx, placeID)
			if err != nil {
				// no place record backing this bucket member; leave it for RemovePlaces
				Logger.Debugf("RemoveMisclassifiedPlacesFromCategoryBuckets: no record for %s in %s", placeID, key)
				continue
			}
			if _, keep := POI.ReclassifyForCategory(place, cat); keep {
				continue
			}
			report.Misclassified++
			report.RemovedIDs = append(report.RemovedIDs, placeID)
			Logger.Infof("RemoveMisclassifiedPlacesFromCategoryBuckets: %s (%q, LocationType=%q, Types=%v) does not belong in %s",
				placeID, place.Name, place.LocationType, place.Types, key)
			if dryRun {
				continue
			}
			if _, err := r.client.ZRem(ctx, key, placeID).Result(); err != nil {
				return report, fmt.Errorf("removing %s from %s: %w", placeID, key, err)
			}
			report.Removed++
		}
	}
	return report, nil
}

// RemoveMisclassifiedPlacesFromCategoryBuckets forwards to the RedisClient method so the
// admin handler can call it through the concrete PoiSearcher (p.Solver.Searcher).
func (s *PoiSearcher) RemoveMisclassifiedPlacesFromCategoryBuckets(ctx context.Context, cat POI.PlaceCategory, dryRun bool) (BucketCleanupReport, error) {
	return s.redisClient.RemoveMisclassifiedPlacesFromCategoryBuckets(ctx, cat, dryRun)
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
		go func(pid string, result PlaceDetailsSearchResult) {
			defer wg.Done()
			place, err := redisClient.getPlace(context, pid)
			if err != nil {
				Logger.Error(err)
				return
			}
			// FIXME: figure out the reason for maps client return null pointer as result
			if reflect.ValueOf(result.res).IsNil() {
				place.SetURL("")
			} else {
				place.SetURL(result.res.URL)
			}
			if err := redisClient.setPlace(context, place); err != nil {
				Logger.Error(err)
			}
		}(placeId, detailedResult)
	}
	wg.Wait()
	return nil
}
