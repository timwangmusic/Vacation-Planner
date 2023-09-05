package planner

import (
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/matching"
	"reflect"
	"testing"
)

func TestCopyRequests(t *testing.T) {
	type args struct {
		req       PlanningReq
		numCopies int
	}
	tests := []struct {
		name    string
		args    args
		want    []PlanningReq
		wantErr bool
	}{
		{
			name: "regular copy request",
			args: args{
				req: PlanningReq{
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
					TravelDate: "1-21-2050",
				},
				numCopies: 2,
			},
			want: []PlanningReq{
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
					TravelDate: "1-21-2050",
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
					TravelDate: "1-21-2050",
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
