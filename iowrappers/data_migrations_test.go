package iowrappers

import (
	"context"
	"net/url"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/weihesdlegend/Vacation-planner/POI"
)

func TestRemovePlaces(t *testing.T) {
	// set up
	RedisMockSvr, _ := miniredis.Run()
	defer RedisMockSvr.Close()

	redisUrl := "redis://" + RedisMockSvr.Addr()
	redisURL, _ := url.Parse(redisUrl)
	redisClient := CreateRedisClient(redisURL)
	ctx := context.WithValue(context.Background(), ContextRequestIdKey, "r-33521-345")
	_ = CreateLogger()

	// create entities in the mock database
	// a place without URL
	placeA := POI.Place{
		ID:           "33511",
		Name:         "Rocky mountains",
		LocationType: POI.LocationTypePark,
		Location: POI.Location{
			Latitude:  12.5635,
			Longitude: 14.7834,
		},
		URL: "",
		Photo: POI.PlacePhoto{
			Reference: "www.rocky-mountains.com/photos",
		},
	}

	// a place without photo
	placeB := POI.Place{
		ID:           "33512",
		Name:         "Contemporary museum",
		LocationType: POI.LocationTypeMuseum,
		Location: POI.Location{
			Latitude:  12.5734,
			Longitude: 14.7912,
		},
		URL: "www.moma.com",
		Photo: POI.PlacePhoto{
			Reference: "",
		},
	}

	// a place with zero user ratings count
	placeC := POI.Place{
		ID:           "33513",
		Name:         "FT Cafe",
		LocationType: POI.LocationTypeCafe,
		PriceLevel:   POI.PriceLevelOne,
		Location: POI.Location{
			Latitude:  12.5734,
			Longitude: 14.7912,
		},
		URL: "www.ftcafe.net",
		Photo: POI.PlacePhoto{
			Reference: "www.ftcafe.net/photos",
		},
		UserRatingsTotal: 0,
	}

	var places []POI.Place
	var err error
	redisClient.SetPlacesAddGeoLocations(ctx, []POI.Place{placeA, placeB, placeC})
	places, _ = redisClient.NearbySearch(ctx, &PlaceSearchRequest{
		PlaceCat:            POI.PlaceCategoryVisit,
		IncludeClosedPlaces: true, // migration test reads raw bucket contents; fixtures carry no Status
		Location: POI.Location{
			Latitude:  12.5636,
			Longitude: 14.7813,
		},
		Radius: 10000,
	})

	if len(places) != 2 {
		t.Errorf("expected number of places equals 2, got %d", len(places))
		return
	}

	err = redisClient.RemovePlaces(ctx, []PlaceDetailsFields{PlaceDetailsFieldURL, PlaceDetailsFieldPhoto, PlaceDetailsFieldUserRatingsCount})
	if err != nil {
		t.Error(err)
		return
	}

	places, _ = redisClient.NearbySearch(ctx, &PlaceSearchRequest{
		PlaceCat:            POI.PlaceCategoryVisit,
		IncludeClosedPlaces: true, // migration test reads raw bucket contents; fixtures carry no Status
		Location: POI.Location{
			Latitude:  12.5636,
			Longitude: 14.7813,
		},
		Radius: 10000,
	})

	if len(places) != 0 {
		t.Errorf("expected number of %s places after removal equals 0, got %d", POI.PlaceCategoryVisit, len(places))
		return
	}

	places, _ = redisClient.NearbySearch(ctx, &PlaceSearchRequest{PlaceCat: POI.PlaceCategoryEatery,
		IncludeClosedPlaces: true, // migration test reads raw bucket contents; fixtures carry no Status
		Location: POI.Location{
			Latitude:  12.5636,
			Longitude: 14.7813,
		},
		Radius: 10000,
	},
	)

	if len(places) != 0 {
		t.Errorf("expected number of %s places after removal equals 0, got %d", POI.PlaceCategoryEatery, len(places))
		return
	}
}

// TestRemovePlacesClearsEveryCategoryBucket covers the orphan bug: removePlace used to ZREM a
// hardcoded "placeIDs:visit" plus "placeIDs:eatery:"+priceLevel — which produced
// "placeIDs:eatery:2" while writes used "placeIDs:eatery:level2" — and never touched the
// Shopping, Lodging, or Wellness buckets. The record was deleted while its geo members stayed
// behind, leaving members that count toward the radius gate and then resolve to nothing.
func TestRemovePlacesClearsEveryCategoryBucket(t *testing.T) {
	RedisMockSvr, _ := miniredis.Run()
	defer RedisMockSvr.Close()

	redisURL, _ := url.Parse("redis://" + RedisMockSvr.Addr())
	redisClient := CreateRedisClient(redisURL)
	ctx := context.Background()
	_ = CreateLogger()

	// One place filed under every category's bucket, as a place matching several searches would
	// be. It has no URL, so the URL requirement below marks it for removal.
	place := POI.Place{
		ID:           "multi-bucket-1",
		Name:         "Everything Emporium",
		LocationType: POI.LocationTypeStore,
		Location:     POI.Location{Latitude: 12.5635, Longitude: 14.7834},
		Photo:        POI.PlacePhoto{Reference: "photo-ref"},
	}
	if err := redisClient.SetPlace(ctx, place); err != nil {
		t.Fatalf("SetPlace: %v", err)
	}
	for _, cat := range POI.AllPlaceCategories {
		key := POI.EncodeNearbySearchRedisKey(cat)
		if err := redisClient.AddGeoLocation(ctx, key, place); err != nil {
			t.Fatalf("AddGeoLocation(%s): %v", key, err)
		}
	}

	if err := redisClient.RemovePlaces(ctx, []PlaceDetailsFields{PlaceDetailsFieldURL}); err != nil {
		t.Fatalf("RemovePlaces: %v", err)
	}

	for _, cat := range POI.AllPlaceCategories {
		key := POI.EncodeNearbySearchRedisKey(cat)
		members, err := redisClient.Get().ZRange(ctx, key, 0, -1).Result()
		if err != nil {
			t.Fatalf("ZRange(%s): %v", key, err)
		}
		for _, member := range members {
			if member == place.ID {
				t.Errorf("%s left behind in bucket %s as an orphan", place.ID, key)
			}
		}
	}
}
