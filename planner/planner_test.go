package planner

import (
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"github.com/weihesdlegend/Vacation-planner/matching"
	"reflect"
	"testing"
)

func TestCopyRequests(t *testing.T) {
	type args struct {
		req       PlanningRequest
		numCopies int
	}
	tests := []struct {
		name    string
		args    args
		want    []PlanningRequest
		wantErr bool
	}{
		{
			name: "regular copy request",
			args: args{
				req: PlanningRequest{
					Location: POI.Location{Country: "China", City: "Beijing"},
					Slots: []SlotRequest{
						{
							Weekday: POI.DateWednesday,
							TimeSlot: matching.TimeSlot{Slot: POI.TimeInterval{
								Start: 10,
								End:   13,
							}},
							Category: POI.PlaceCategoryVisit,
						},
					},
					TravelDate:       "1-21-2050",
					WithNearbyCities: true,
				},
				numCopies: 2,
			},
			want: []PlanningRequest{
				{
					Location: POI.Location{Country: "China", City: "Beijing"},
					Slots: []SlotRequest{
						{
							Weekday: POI.DateWednesday,
							TimeSlot: matching.TimeSlot{Slot: POI.TimeInterval{
								Start: 10,
								End:   13,
							}},
							Category: POI.PlaceCategoryVisit,
						},
					},
					TravelDate:       "1-21-2050",
					WithNearbyCities: true,
				},
				{
					Location: POI.Location{Country: "China", City: "Beijing"},
					Slots: []SlotRequest{
						{
							Weekday: POI.DateWednesday,
							TimeSlot: matching.TimeSlot{Slot: POI.TimeInterval{
								Start: 10,
								End:   13,
							}},
							Category: POI.PlaceCategoryVisit,
						},
					},
					TravelDate:       "1-21-2050",
					WithNearbyCities: true,
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := deepCopyAnything(tt.args.req, tt.args.numCopies)
			if (err != nil) != tt.wantErr {
				t.Errorf("CopyRequests() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CopyRequests() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_planDetails(t *testing.T) {
	type args struct {
		r *iowrappers.PlanningSolutionRecord
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "planning solution record",
			args: args{&iowrappers.PlanningSolutionRecord{
				PlaceNames:  []string{"Tian Tan Park", "The Celestial Palace"},
				TimeSlots:   []string{"From 10 to 12", "From 15 to 17"},
				Destination: POI.Location{Country: "China", City: "Beijing", AdminAreaLevelOne: "Beijing"},
			}},
			want:    "Visiting Beijing, BEIJING, CHINA. From 10 to 12 at: Tian Tan Park; From 15 to 17 at: The Celestial Palace",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := planDetails(tt.args.r)
			if (err != nil) != tt.wantErr {
				t.Errorf("planDetails() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("planDetails() got = %v, want %v", got, tt.want)
			}
		})
	}
}
