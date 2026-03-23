package planner

import (
	"testing"

	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/matching"
)

func makePlace(id string, lat, lng float64) matching.Place {
	return matching.Place{
		Place: &POI.Place{
			ID:               id,
			Rating:           4.0,
			UserRatingsTotal: 100,
			Location:         POI.Location{Latitude: lat, Longitude: lng},
		},
		Price: 1,
	}
}

func TestGroupPlacesBySpatialClusters_SingleCluster(t *testing.T) {
	// all places within a small area should form one cluster
	places := []matching.Place{
		makePlace("a1", 40.7128, -74.0060),
		makePlace("a2", 40.7130, -74.0058),
	}
	clusters := [][]matching.Place{places, places}

	groups := groupPlacesBySpatialClusters(clusters, DefaultPlaceSearchRadius)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group for nearby places, got %d", len(groups))
	}
	for slotIdx, slot := range groups[0] {
		if len(slot) != 2 {
			t.Errorf("slot %d: expected 2 places, got %d", slotIdx, len(slot))
		}
	}
}

func TestGroupPlacesBySpatialClusters_MultipleClusters(t *testing.T) {
	// places far apart should form separate clusters
	// NYC and LA are ~3900km apart
	nycPlaces := []matching.Place{
		makePlace("nyc1", 40.7128, -74.0060),
		makePlace("nyc2", 40.7200, -74.0000),
	}
	laPlaces := []matching.Place{
		makePlace("la1", 34.0522, -118.2437),
		makePlace("la2", 34.0600, -118.2500),
	}

	slot0 := append(nycPlaces, laPlaces...)
	slot1 := append(nycPlaces, laPlaces...)

	clusters := [][]matching.Place{slot0, slot1}
	groups := groupPlacesBySpatialClusters(clusters, DefaultPlaceSearchRadius)

	if len(groups) < 2 {
		t.Fatalf("expected at least 2 groups for distant places, got %d", len(groups))
	}

	// each group should have places from both slots
	for gi, group := range groups {
		for si, slot := range group {
			if len(slot) == 0 {
				t.Errorf("group %d, slot %d: expected places but got none", gi, si)
			}
		}
	}
}

func TestGroupPlacesBySpatialClusters_EmptySlotFiltered(t *testing.T) {
	// if a spatial cluster has no places in a slot, that group is excluded
	nycPlace := makePlace("nyc1", 40.7128, -74.0060)
	laPlace := makePlace("la1", 34.0522, -118.2437)

	// slot 0 has both NYC and LA, slot 1 only has NYC
	slot0 := []matching.Place{nycPlace, laPlace}
	slot1 := []matching.Place{nycPlace}

	clusters := [][]matching.Place{slot0, slot1}
	groups := groupPlacesBySpatialClusters(clusters, DefaultPlaceSearchRadius)

	// LA group should be filtered out since slot1 has no LA places
	for gi, group := range groups {
		for si, slot := range group {
			if len(slot) == 0 {
				t.Errorf("group %d, slot %d: should not have empty slot in valid group", gi, si)
			}
		}
	}
}

func TestGroupPlacesBySpatialClusters_Empty(t *testing.T) {
	groups := groupPlacesBySpatialClusters(nil, DefaultPlaceSearchRadius)
	if groups != nil {
		t.Errorf("expected nil for empty input, got %v", groups)
	}
}

func TestMaxPlacesPerSlotTruncation(t *testing.T) {
	// verify the constant is reasonable
	if MaxPlacesPerSlot <= 0 {
		t.Fatal("MaxPlacesPerSlot must be positive")
	}
	if MaxPlacesPerSlot > MaxSolutionsToSaveCount {
		t.Error("MaxPlacesPerSlot should not exceed MaxSolutionsToSaveCount")
	}
}
