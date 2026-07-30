package iowrappers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

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
	// Remove the member from every category bucket, since a place can be filed under more than
	// one. This used to hardcode "placeIDs:visit" plus "placeIDs:eatery:"+priceLevel — which
	// produced "placeIDs:eatery:2" while the write path used "placeIDs:eatery:level2", and never
	// touched the Shopping/Lodging/Wellness buckets at all. The result was a deleted
	// place_details record with its geo members left behind: orphans that count toward the
	// MinNumResults radius gate and then resolve to nothing on read. Going through the shared
	// encoder is what stops the two sides drifting again.
	for _, cat := range POI.AllPlaceCategories {
		if _, err := r.client.ZRem(context, POI.EncodeNearbySearchRedisKey(cat), placeID).Result(); err != nil {
			return fmt.Errorf("removing place %s from bucket %s: %w", placeID, POI.EncodeNearbySearchRedisKey(cat), err)
		}
	}

	return r.RemoveKeys(context, []string{placeRedisKey})
}

// detailsSourcedFields are the stored-record fields that only a Place Details call can supply.
//
// URL is the whole list on purpose. Opening hours look like the obvious signal but cannot be
// used: POI.CreatePlace backfills every missing weekday with a default string
// ("8:30 am – 9:30 pm"), so hours are never empty on a stored record and the check would always
// pass. FormattedAddress is also unusable because the Nearby Search response carries one of its
// own. URL has exactly one source — urlMap, populated only from a Details result — so a
// non-empty URL is proof a Details call has landed on this record.
var detailsSourcedFields = []PlaceDetailsFields{PlaceDetailsFieldURL}

// placeDetailsAreCurrent reports whether a stored record can stand in for a Place Details call,
// so the call can be skipped. Requires both that a Details call has populated the record and
// that it is recent: skipping on mere existence would freeze a record permanently, since the
// external-search refresh is the only thing that ever rewrites it.
func placeDetailsAreCurrent(place POI.Place, now time.Time) bool {
	if !isPlaceDetailsValid(place, detailsSourcedFields) {
		return false
	}
	lastUpdated, err := time.Parse(time.RFC3339, place.LastUpdatedAt)
	if err != nil {
		// Records written before LastUpdatedAt was populated, or with an unparsable value,
		// cannot be aged — refresh them rather than trusting them forever.
		return false
	}
	return now.Sub(lastUpdated) <= PlaceDetailsRefreshDuration
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

// bucketCleanupReadBatchSize is how many place records the bucket scan reads per pipeline
// round trip, matching the batch size SetPlacesAddGeoLocations uses on the write side.
const bucketCleanupReadBatchSize = 100

// BucketCleanupReport summarizes a RemoveMisclassifiedPlacesFromCategoryBuckets run.
// BucketSizes/TotalMembers are measured with ZCARD before the scan begins so a dry-run
// report states the scale of the job even for buckets an operator has never sized.
type BucketCleanupReport struct {
	BucketSizes   map[string]int64 `json:"bucket_sizes"`
	TotalMembers  int64            `json:"total_members"`
	Scanned       int              `json:"scanned"`
	Misclassified int              `json:"misclassified"`
	Removed       int              `json:"removed"`
	RemovedIDs    []string         `json:"removed_ids"`
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

// EateryBucketUnionReport describes a run of UnionEateryPriceBucketsIntoCategoryBucket.
// ExpectedAfter is computed by reading members rather than by writing, so a dry run states the
// exact resulting size without touching anything.
type EateryBucketUnionReport struct {
	SourceKeys    []string         `json:"source_keys"`
	SourceSizes   map[string]int64 `json:"source_sizes"`
	SourceTotal   int64            `json:"source_total"`
	TargetKey     string           `json:"target_key"`
	TargetBefore  int64            `json:"target_before"`
	ExpectedAfter int64            `json:"expected_after"`
	TargetAfter   int64            `json:"target_after"`
}

// UnionEateryPriceBucketsIntoCategoryBucket merges the legacy per-price eatery geo indexes
// (placeIDs:eatery:level0..4) into the single placeIDs:eatery bucket that
// POI.EncodeNearbySearchRedisKey now names.
//
// Run this BEFORE deploying the code that reads the collapsed key. It is purely additive and
// invisible to the running code, whereas deploying first would point every eatery read at a key
// that does not exist yet and trigger a global cold-search burst.
//
// The legacy key format is spelled out literally here on purpose: the encoder no longer emits
// it, and a migration is the one place a retired format belongs.
//
// AGGREGATE MIN is mandatory. redis.ZStore.Aggregate defaults to SUM, and a GEO member's score
// IS its 52-bit geohash — summing the scores of a place that appears in two source buckets
// would silently relocate it, in our case to somewhere in the ocean. MIN keeps a real geohash,
// and since a place's coordinates are identical across buckets, which one survives is
// immaterial.
//
// The target is included in the union sources so the migration is re-runnable and cannot drop
// members that new code has already written to the collapsed key.
func (r *RedisClient) UnionEateryPriceBucketsIntoCategoryBucket(ctx context.Context, dryRun bool) (EateryBucketUnionReport, error) {
	target := POI.EncodeNearbySearchRedisKey(POI.PlaceCategoryEatery)
	sources := make([]string, 0, len(POI.AllPriceLevels))
	for _, level := range POI.AllPriceLevels {
		sources = append(sources, fmt.Sprintf("%s:level%d", target, level))
	}

	report := EateryBucketUnionReport{
		SourceKeys:  sources,
		SourceSizes: make(map[string]int64, len(sources)),
		TargetKey:   target,
	}

	distinct := set.Of[string]{}
	for _, key := range sources {
		size, err := r.client.ZCard(ctx, key).Result()
		if err != nil {
			return report, fmt.Errorf("sizing legacy bucket %s: %w", key, err)
		}
		report.SourceSizes[key] = size
		report.SourceTotal += size

		members, err := r.client.ZRange(ctx, key, 0, -1).Result()
		if err != nil {
			return report, fmt.Errorf("reading legacy bucket %s: %w", key, err)
		}
		distinct.Add(members...)
	}

	before, err := r.client.ZCard(ctx, target).Result()
	if err != nil {
		return report, fmt.Errorf("sizing target bucket %s: %w", target, err)
	}
	report.TargetBefore = before

	targetMembers, err := r.client.ZRange(ctx, target, 0, -1).Result()
	if err != nil {
		return report, fmt.Errorf("reading target bucket %s: %w", target, err)
	}
	distinct.Add(targetMembers...)
	report.ExpectedAfter = int64(distinct.Len())

	if dryRun {
		report.TargetAfter = before
		return report, nil
	}

	unionKeys := make([]string, 0, len(sources)+1)
	unionKeys = append(unionKeys, sources...)
	unionKeys = append(unionKeys, target)
	if _, err := r.client.ZUnionStore(ctx, target, &redis.ZStore{
		Keys:      unionKeys,
		Aggregate: "MIN",
	}).Result(); err != nil {
		return report, fmt.Errorf("unioning legacy eatery buckets into %s: %w", target, err)
	}

	after, err := r.client.ZCard(ctx, target).Result()
	if err != nil {
		return report, fmt.Errorf("re-sizing target bucket %s: %w", target, err)
	}
	report.TargetAfter = after
	if after != report.ExpectedAfter {
		Logger.Errorf("UnionEateryPriceBucketsIntoCategoryBucket: %s has %d members, expected %d",
			target, after, report.ExpectedAfter)
	}
	return report, nil
}

// UnionEateryPriceBucketsIntoCategoryBucket forwards to the RedisClient method so the admin
// handler can call it through the concrete PoiSearcher.
func (s *PoiSearcher) UnionEateryPriceBucketsIntoCategoryBucket(ctx context.Context, dryRun bool) (EateryBucketUnionReport, error) {
	return s.redisClient.UnionEateryPriceBucketsIntoCategoryBucket(ctx, dryRun)
}

// RemoveMisclassifiedPlacesFromCategoryBuckets removes places from cat's geo buckets whose
// PRIMARY Google type does not belong to cat. It repairs the fast_food_restaurant incident:
// two Places-API-(New)-only types were searched against the legacy Nearby Search, which
// ignored the unenforceable type filter and returned prominence-ranked establishments, and
// those were stamped with the queried type and written into placeIDs:eatery:level*.
//
// The removal rule is the exact inverse of the WRITE rule, not of POI.ReclassifyForCategory.
// SetPlacesAddGeoLocations files a place under GetPlaceCategory(place.LocationType) and
// refuses to write anything whose type maps to no category, so this migration removes a
// member only when its primary type positively maps to a DIFFERENT category (lodging ->
// Lodging, supermarket -> Shopping). An UNMAPPED primary type is deliberately kept: types
// like "meal_delivery" and "night_club" are legal legacy types that Google routinely lists
// first for genuine eateries (a delivery-first restaurant, a bar that is also a club), as are
// records with no Types at all (cached before Types was captured). Those places carry a
// stamped LocationType the write path maps straight back to this category, so removing them
// would only delete rows the next cold search re-creates — while shrinking the trip-planning
// candidate pool for up to MinMapsResultRefreshDuration, because the trip-planning path
// (planner/solver.go -> matching.NearbySearchForCategory) reads these buckets with no
// reclassification at all.
//
// Note this is a broader keep-set than POI.ReclassifyForCategory applies on the merchant
// endpoint: that function keeps a place only when its primary type is one of the category's
// five search types. Divergence is intended — one function decides what to show in a single
// response, this one decides what may exist in the shared cache.
//
// dryRun reports what would be removed without deleting anything. Always dry-run first.
func (r *RedisClient) RemoveMisclassifiedPlacesFromCategoryBuckets(ctx context.Context, cat POI.PlaceCategory, dryRun bool) (BucketCleanupReport, error) {
	report := BucketCleanupReport{RemovedIDs: make([]string, 0), BucketSizes: make(map[string]int64)}

	// One bucket per category since the eatery price split was collapsed; the loops below still
	// take a slice so the shape survives if a category is ever partitioned again.
	keys := []string{POI.EncodeNearbySearchRedisKey(cat)}

	// Size the job before doing it. The scan cost is linear in bucket membership and the
	// handler runs under the caller's request context, so an operator needs the member count
	// to judge whether a dry run can complete inside their client/router timeout.
	for _, key := range keys {
		size, err := r.client.ZCard(ctx, key).Result()
		if err != nil {
			return report, fmt.Errorf("sizing geo bucket %s: %w", key, err)
		}
		report.BucketSizes[key] = size
		report.TotalMembers += size
	}

	for _, key := range keys {
		members, err := r.client.ZRange(ctx, key, 0, -1).Result()
		if err != nil {
			return report, fmt.Errorf("reading geo bucket %s: %w", key, err)
		}
		// Read place records in pipelined batches rather than one GET per member: a serial
		// N+1 over a real bucket cannot finish inside a 30s request timeout, which would
		// make the mandatory dry-run review impossible and defeat the safety property.
		for start := 0; start < len(members); start += bucketCleanupReadBatchSize {
			batch := members[start:min(start+bucketCleanupReadBatchSize, len(members))]
			places, found, err := r.getPlacesPipelined(ctx, batch)
			if err != nil {
				return report, fmt.Errorf("reading place records for %s: %w", key, err)
			}
			for i, placeID := range batch {
				report.Scanned++
				if !found[i] {
					// no place record backing this bucket member; leave it for RemovePlaces
					Logger.Debugf("RemoveMisclassifiedPlacesFromCategoryBuckets: no record for %s in %s", placeID, key)
					continue
				}
				place := places[i]
				primary := POI.PrimaryLocationType(place.Types)
				// Only remove members whose primary type positively belongs to a DIFFERENT
				// category. An unmapped primary type (meal_delivery, night_club, or no Types
				// at all) is not evidence of misclassification — the write path would
				// legitimately place it here.
				if c, ok := POI.GetPlaceCategory(primary); !ok || c == cat {
					continue
				}
				report.Misclassified++
				report.RemovedIDs = append(report.RemovedIDs, placeID)
				Logger.Infof("RemoveMisclassifiedPlacesFromCategoryBuckets: %s (%q, LocationType=%q, primary=%q, Types=%v) does not belong in %s",
					placeID, place.Name, place.LocationType, primary, place.Types, key)
				if dryRun {
					continue
				}
				if _, err := r.client.ZRem(ctx, key, placeID).Result(); err != nil {
					return report, fmt.Errorf("removing %s from %s: %w", placeID, key, err)
				}
				report.Removed++
			}
		}
	}
	return report, nil
}

// getPlacesPipelined fetches the place records for placeIDs in a single round trip.
// found[i] reports whether placeIDs[i] had a usable record: a bucket member with no (or an
// unparsable) backing record is not this migration's problem to fix, so it is reported as
// missing rather than failing the whole batch. A transport-level failure is returned as an
// error, because then nothing in the batch was actually read.
func (r *RedisClient) getPlacesPipelined(ctx context.Context, placeIDs []string) ([]POI.Place, []bool, error) {
	cmds := make([]*redis.StringCmd, len(placeIDs))
	_, err := r.client.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		for i, placeID := range placeIDs {
			cmds[i] = pipe.Get(ctx, PlaceDetailsRedisKeyPrefix+placeID)
		}
		return nil
	})
	// Pipelined surfaces the first non-nil command error, and a missing key is reported as
	// redis.Nil — an expected outcome here, not a failure.
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, nil, err
	}

	places := make([]POI.Place, len(placeIDs))
	found := make([]bool, len(placeIDs))
	for i, cmd := range cmds {
		res, cmdErr := cmd.Result()
		if cmdErr != nil {
			continue
		}
		if unmarshalErr := json.Unmarshal([]byte(res), &places[i]); unmarshalErr != nil {
			Logger.Debugf("getPlacesPipelined: cannot parse record for %s: %v", placeIDs[i], unmarshalErr)
			continue
		}
		found[i] = true
	}
	return places, found, nil
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
