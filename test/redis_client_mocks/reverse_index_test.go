package redis_client_mocks

import (
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"reflect"
	"testing"
)

var TestReverseIndex = &iowrappers.ReverseIndex{
	Metadata: &iowrappers.IndexMetadata{
		Location: POI.Location{City: "Beijing", Country: "China"},
		Weekday:  POI.DateSaturday,
		Interval: POI.TimeInterval{Start: 10, End: 13},
	},
	Scores:   []float64{7.2, 5.6, 6.8},
	PlaceIDs: []string{"Summer Palace", "Art Museum", "Peking Restaurant"},
}

func TestCreateAndRetrieveReverseIndex_ShouldRetrieveCorrectPlaces(t *testing.T) {
	var err error
	if err = RedisClient.CreateReverseIndex(RedisContext, TestReverseIndex); err != nil {
		t.Error(err)
	}

	var res *iowrappers.ReverseIndex
	if res, err = RedisClient.RetrieveReverseIndex(RedisContext, TestReverseIndex.Metadata, 3); err != nil {
		t.Error(err)
	}

	expectedPlaceIDs := []string{"Summer Palace", "Peking Restaurant", "Art Museum"}
	if !reflect.DeepEqual(res.PlaceIDs, expectedPlaceIDs) {
		t.Errorf("Expected place IDs are %+v, got %+v", expectedPlaceIDs, res.PlaceIDs)
	}

	t.Logf("Retrieved place IDs are: %+v", res.PlaceIDs)
}
