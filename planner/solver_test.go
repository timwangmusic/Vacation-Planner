package planner

import (
	"errors"
	"reflect"
	"testing"

	"github.com/bobg/go-generics/slices"
	"github.com/modern-go/reflect2"
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"github.com/weihesdlegend/Vacation-planner/matching"
	"github.com/weihesdlegend/Vacation-planner/utils"
)

func init() {
	utils.LogErrorWithLevel(iowrappers.CreateLogger(), utils.LogFatal)
}

func TestSolver_filterPlaces1(t *testing.T) {
	type fields struct {
		Searcher               *iowrappers.PoiSearcher
		placeMatcher           *PlaceMatcher
		placeDedupeCountLimit  int
		nearbyCitiesCountLimit int
	}
	type args struct {
		places []matching.Place
		params map[matching.FilterCriteria]interface{}
		c      POI.PlaceCategory
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []string
		wantErr bool
	}{
		{
			name: "test filter places should return correct results",
			fields: fields{
				Searcher:     &iowrappers.PoiSearcher{},
				placeMatcher: &PlaceMatcher{},
			},
			args: args{
				c: POI.PlaceCategoryEatery,
				places: []matching.Place{
					{
						Place: &POI.Place{
							ID:               "33521",
							UserRatingsTotal: 100,
							Hours: [7]string{
								"Monday: 6:00AM-10:00PM",
								"Tuesday: 6:00AM-10:00PM",
								"Wednesday: 6:00AM-10:00PM",
								"Thursday: 6:00AM-10:00PM",
								"Friday: 6:00AM-10:00PM",
								"Saturday: 6:00AM-10:00PM",
								"Sunday: 6:00AM-10:00PM",
							},
							PriceLevel: POI.PriceLevelTwo,
						},
						Category: POI.PlaceCategoryEatery,
					},
					{
						Place: &POI.Place{
							ID:               "33522",
							UserRatingsTotal: 2000,
							Hours: [7]string{
								"Monday: 10:00AM-10:00PM",
								"Tuesday: 10:00AM-10:00PM",
								"Wednesday: 10:00AM-10:00PM",
								"Thursday: 11:00AM-10:00PM",
								"Friday: 12:00PM-11:00PM",
								"Saturday: 10:00AM-10:00PM",
								"Sunday: 11:00AM-10:00PM",
							},
							PriceLevel: POI.PriceLevelTwo,
						},
						Category: POI.PlaceCategoryEatery,
					},
				},
				params: map[matching.FilterCriteria]interface{}{
					matching.FilterByUserRating: matching.UserRatingFilterParams{
						MinUserRatings: 1,
					},
					matching.FilterByTimePeriod: matching.TimeFilterParams{
						Day: POI.DateFriday,
						TimeInterval: POI.TimeInterval{
							Start: 11,
							End:   16,
						},
					},
					matching.FilterByPriceRange: matching.PriceRangeFilterParams{
						Category:   POI.PlaceCategoryEatery,
						PriceLevel: POI.PriceLevelTwo,
					},
				},
			},
			want:    []string{"33521"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Solver{}
			s.Init(tt.fields.Searcher, tt.fields.placeDedupeCountLimit, tt.fields.nearbyCitiesCountLimit)
			got, err := s.filterPlaces(tt.args.places, tt.args.params, tt.args.c)
			if (err != nil) != tt.wantErr {
				t.Errorf("filterPlaces() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			placeIDs, err := extractPlaceIDs(got)
			if err != nil {
				t.Error(err)
				return
			}
			if !reflect.DeepEqual(placeIDs, tt.want) {
				t.Errorf("filterPlaces() got = %v, want %v", placeIDs, tt.want)
			}
		})
	}
}

func extractPlaceIDs(places []matching.Place) ([]string, error) {
	placeIDs, err := slices.Map(places, func(idx int, p matching.Place) (string, error) {
		if reflect2.IsNil(p.Place) {
			return "", errors.New("place field is empty")
		}
		return p.Place.GetID(), nil
	})
	if err != nil {
		return nil, err
	}
	return placeIDs, nil
}
