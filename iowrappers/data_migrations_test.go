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
		PlaceCat: POI.PlaceCategoryVisit,
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
		PlaceCat: POI.PlaceCategoryVisit,
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
