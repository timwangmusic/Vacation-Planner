package iowrappers

import (
	"context"
	"github.com/alicebob/miniredis/v2"
	"github.com/weihesdlegend/Vacation-planner/POI"
	"net/url"
	"testing"
)

func TestRemovePlaces(t *testing.T) {
	// set up
	RedisMockSvr, _ := miniredis.Run()

	redisUrl := "redis://" + RedisMockSvr.Addr()
	redisURL, _ := url.Parse(redisUrl)
	redisClient := CreateRedisClient(redisURL)
	ctx := context.WithValue(context.Background(), "request_id", "r-33521-345")
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

	var places []POI.Place
	var err error
	redisClient.SetPlacesOnCategory(ctx, []POI.Place{placeA, placeB})
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

	err = redisClient.RemovePlaces(ctx, []PlaceDetailsFields{PlaceDetailsFieldURL, PlaceDetailsFieldPhoto})
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
		t.Errorf("expected number of places after removal equals 0, got %d", len(places))
		return
	}
}
