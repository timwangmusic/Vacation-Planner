package test

import (
	"github.com/stretchr/testify/assert"
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/graph"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"testing"
)

const MockURL = "www.maps.google.com/my-favorite"

type SearchClientMock struct {
}

func (mocker SearchClientMock) GetGeocode(*iowrappers.GeocodeQuery) (float64, float64, error) {
	return 0.0, 0.0, nil
}

func (mocker SearchClientMock) NearbySearch(*iowrappers.PlaceSearchRequest) ([]POI.Place, error) {
	return nil, nil
}

func (mocker SearchClientMock) PlaceDetailsSearch(string) (place POI.Place, err error) {
	place.URL = MockURL
	return
}

func TestUpdatePlaceDetails(t *testing.T) {
	searcher := SearchClientMock{}

	places := make([]POI.Place, 1)
	places[0] = POI.Place{URL: ""}
	assert.True(t, places[0].GetURL()=="")

	placesNeedUpdate := graph.UpdatePlacesDetails(searcher, places)
	if len(placesNeedUpdate) != 1 {
		t.Fatalf("expected number of places need update to be 1, got %d", len(placesNeedUpdate))
	}
	if placesNeedUpdate[0].GetURL() != MockURL {
		t.Errorf("expected updated URL to be %s, got %s", MockURL, placesNeedUpdate[0].GetURL())
	}
}
