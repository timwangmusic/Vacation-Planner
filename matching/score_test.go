package matching

import (
	"testing"

	"github.com/weihesdlegend/Vacation-planner/POI"
)

func TestPlaceScore(t *testing.T) {
	type args struct {
		place Place
	}
	tests := []struct {
		name string
		args args
		want float64
	}{
		{
			name: "test compute place score with zero price should return correct result",
			args: args{place: Place{
				Place:    &POI.Place{Name: "Local Park", UserRatingsTotal: 99, Rating: 4.0},
				Category: POI.PlaceCategoryVisit,
				Price:    0,
			}},
			want: 1.6,
		},
		{
			name: "test compute place score with price at level two should return correct result",
			args: args{place: Place{
				Place:    &POI.Place{Name: "Uncle Pizza", UserRatingsTotal: 999, Rating: 4.0},
				Category: POI.PlaceCategoryEatery,
				Price:    2,
			}},
			want: 6.0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PlaceScore(tt.args.place); got != tt.want {
				t.Errorf("PlaceScore() = %v, want %v", got, tt.want)
			}
		})
	}
}
