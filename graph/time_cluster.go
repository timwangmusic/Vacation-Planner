// This file defines TimeClusters architecture.
// One TimeClusters consists of multiple TimeCluster and time intervals.
// One TimeCluster consists of a set of Places and a time interval.
// All TimeClusters are controlled by TimeClustersManager.
// One TimeClustersManager manages one TimeClusters.
// TimeClustersManager representation contains a set of Places for ease of data containment.
package graph

import (
	"context"
	log "github.com/sirupsen/logrus"
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
)

const (
	TimeClusterMinResults = 20
)

type ClusterManager interface {
	PlaceSearch(context.Context, string, uint) // location and search radius
}

// TimeCluster consists of a set of Places and a time interval
type TimeCluster struct {
	Places       []POI.Place
	TimeInterval POI.TimeInterval
}

// an interface for handling interval-like data structure
type TimeIntervals interface {
	NumIntervals() int                         // get number of time intervals
	GetAllIntervals() *[]POI.TimeInterval      // get all time intervals as a list of Start and End time
	GetInterval(int) (error, POI.TimeInterval) // get an interval by specifying its index
	InsertTimeInterval(POI.TimeInterval)       // add an interval
}

// returns number of Places in a time cluster
func (cls *TimeCluster) Size() int {
	return len(cls.Places)
}

// TimeClusters consists of multiple TimeCluster
type TimeClusters struct {
	Clusters      map[string]*TimeCluster // maps interval to time cluster
	TimeIntervals TimeIntervals           // all time intervals
}

// returns number of time intervals in a time-based cluster
func (cls *TimeClusters) Size() int {
	return len(cls.Clusters)
}

//TimeClustersManager
//To use a TimeClustersManager, fetch Places data using PlaceSearch and then time clustering with Clustering method.
type TimeClustersManager struct {
	poiSearcher  *iowrappers.PoiSearcher
	TimeClusters *TimeClusters
	places       []POI.Place
	PlaceCat     POI.PlaceCategory
	Weekday      POI.Weekday
}

// TimeClusterManager initialization
func (placeManager *TimeClustersManager) Init(poiSearcher *iowrappers.PoiSearcher, placeCat POI.PlaceCategory,
	timeIntervals []POI.TimeInterval, day POI.Weekday) {
	placeManager.poiSearcher = poiSearcher
	placeManager.PlaceCat = placeCat
	placeManager.Weekday = day
	placeManager.TimeClusters = &TimeClusters{Clusters: make(map[string]*TimeCluster, 0)}
	placeManager.TimeClusters.TimeIntervals = &POI.GoogleMapsTimeIntervals{}
	for _, interval := range timeIntervals {
		clusterKey := interval.Serialize()
		placeManager.TimeClusters.TimeIntervals.InsertTimeInterval(interval)
		placeManager.TimeClusters.Clusters[clusterKey] = &TimeCluster{TimeInterval: interval}
		placeManager.TimeClusters.Clusters[clusterKey].Places = make([]POI.Place, 0)
	}
}

// searchType is a selector for MinNumResults in PlaceSearchRequest
func (placeManager *TimeClustersManager) PlaceSearch(context context.Context, location string, searchRadius uint) {
	request := iowrappers.PlaceSearchRequest{
		Location:      location,
		PlaceCat:      placeManager.PlaceCat,
		Radius:        searchRadius,
		MinNumResults: TimeClusterMinResults,
	}
	var err error
	placeManager.places, err = placeManager.poiSearcher.NearbySearch(context, &request)
	if err != nil {
		log.Error(err)
	}
	//placeManager.poiSearcher.UpdateRedis(nil, UpdatePlacesDetails(*placeManager.poiSearcher, placeManager.places))
}

// assign Places to time Clusters using their time interval info
func (placeManager *TimeClustersManager) Clustering(day POI.Weekday) {
	for _, place := range placeManager.places {
		placeManager.assign(&place, day)
	}
}

func (placeManager *TimeClustersManager) assign(place *POI.Place, day POI.Weekday) {
	openingHour := place.GetHour(day) // get opening hour for a weekday
	openingInterval, err := POI.ParseTimeInterval(openingHour)
	if err != nil {
		return
	}
	for _, interval := range *placeManager.TimeClusters.TimeIntervals.GetAllIntervals() {
		if openingInterval.Inclusive(&interval) {
			clusterKey := interval.Serialize()
			clusterPlaces := &placeManager.TimeClusters.Clusters[clusterKey].Places
			*clusterPlaces = append(*clusterPlaces, *place)
		}
	}
}
