package iowrappers

import (
	"context"
	"errors"
	"strings"

	"github.com/redis/go-redis/v9"
	"github.com/weihesdlegend/Vacation-planner/POI"
)

const ReverseIndexStringPrefix string = "reverse_index"

type IndexMetadata struct {
	Location POI.Location
	Weekday  POI.Weekday
	Interval POI.TimeInterval
}

type ReverseIndex struct {
	Metadata *IndexMetadata
	Scores   []float64
	PlaceIDs []string
}

func reverseIndexKey(metadata *IndexMetadata) string {
	start, end := metadata.Interval.Start.ToString(), metadata.Interval.End.ToString()
	return strings.Join([]string{ReverseIndexStringPrefix, metadata.Location.Country, metadata.Location.AdminAreaLevelOne, metadata.Location.City, metadata.Weekday.String(), start, end}, ":")
}

func (r *RedisClient) CreateReverseIndex(ctx context.Context, index *ReverseIndex) error {
	if !validateIndex(index) {
		return errors.New("the number of scores and places should be the same")
	}
	redisKey := reverseIndexKey(index.Metadata)
	for idx, id := range index.PlaceIDs {
		err := r.Get().ZAdd(ctx, redisKey, redis.Z{Score: index.Scores[idx], Member: id}).Err()
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *RedisClient) RetrieveReverseIndex(ctx context.Context, metadata *IndexMetadata, count int64) (*ReverseIndex, error) {
	redisKey := reverseIndexKey(metadata)
	// only limit the number of places
	res := r.Get().ZRevRangeByScore(ctx, redisKey, &redis.ZRangeBy{
		Max:   "+inf",
		Min:   "-inf",
		Count: count,
	})
	if res.Err() != nil {
		return nil, res.Err()
	}
	return &ReverseIndex{
		Metadata: metadata,
		Scores:   nil,
		PlaceIDs: res.Val(),
	}, nil
}

func validateIndex(index *ReverseIndex) bool {
	return len(index.Scores) == len(index.PlaceIDs)
}
