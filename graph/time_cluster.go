// This file defines TimeClusters architecture.
// One TimeClusters consists of multiple TimeCluster and time intervals.
// One TimeCluster consists of a set of Places and a time interval.
// All TimeClusters are controlled by TimeClustersManager.
// One TimeClustersManager manages one TimeClusters.
// TimeClustersManager representation contains a set of Places for ease of data containment.
package graph

import (
	"Vacation-planner/POI"
	"Vacation-planner/iowrappers"
)

// TimeCluster consists of a set of Places and a time interval
type TimeCluster struct {
	Places       []POI.Place
	TimeInterval POI.TimeInterval
}

// returns number of Places in a time cluster
func (cls *TimeCluster) Size() int{
	return len(cls.Places)
}

// TimeClusters consists of multiple TimeCluster
type TimeClusters struct {
	Clusters      map[string]*TimeCluster     // maps interval to time cluster
	TimeIntervals POI.GoogleMapsTimeIntervals // all time intervals
}

// returns number of time intervals in a time-based cluster
func (cls *TimeClusters) Size() int{
	return len(cls.Clusters)
}

//TimeClustersManager
//To use a TimeClustersManager, fetch Places data using GetPlaces and then time clustering with TimeClustering method.
type TimeClustersManager struct{
	Client        *iowrappers.MapsClient
	TimeClusters  *TimeClusters
	places        []POI.Place
	PlaceCat      POI.PlaceCategory
	Weekday		  POI.Weekday
}

// TimeClusterManager initialization
func (placeManager *TimeClustersManager) Init(client *iowrappers.MapsClient, placeCat POI.PlaceCategory,
	timeIntervals []POI.TimeInterval, day POI.Weekday){
	placeManager.Client = client
	placeManager.PlaceCat = placeCat
	placeManager.Weekday = day
	placeManager.TimeClusters = &TimeClusters{Clusters: make(map[string]*TimeCluster, 0)}
	placeManager.TimeClusters.TimeIntervals = POI.GoogleMapsTimeIntervals{}
	for _, interval := range timeIntervals{
		clusterKey := interval.Serialize()
		placeManager.TimeClusters.TimeIntervals.InsertTimeInterval(interval)
		placeManager.TimeClusters.Clusters[clusterKey] = &TimeCluster{TimeInterval: interval}
		placeManager.TimeClusters.Clusters[clusterKey].Places = make([]POI.Place, 0)
	}
}

// call Google API to obtain nearby Places and extract location data
func (placeManager *TimeClustersManager) GetPlaces(location string, searchRadius uint, searchType string){
	request := iowrappers.PlaceSearchRequest{
		Location: location,
		PlaceCat: placeManager.PlaceCat,
		Radius: searchRadius,
		RankBy: "prominence",
	}
	if searchType == ""{
		request.MaxNumResults = 20
	} else{
		request.MaxNumResults = 100
	}
	placeManager.places = placeManager.Client.NearbySearch(&request)
}

// assign Places to time Clusters using their time interval info
func (placeManager *TimeClustersManager) TimeClustering(day POI.Weekday) {
	for _, place := range placeManager.places{
		placeManager.assign(&place, day)
	}
}

func (placeManager *TimeClustersManager) assign(place *POI.Place, day POI.Weekday) {
	openingHour := place.GetHour(day)		// get opening hour for a weekday
	openingInterval, err := POI.ParseTimeInterval(openingHour)
	if err != nil{
		return
	}
	for _, interval := range *placeManager.TimeClusters.TimeIntervals.GetAllIntervals(){
		if openingInterval.Intersect(&interval){
			clusterKey := interval.Serialize()
			clusterPlaces := &placeManager.TimeClusters.Clusters[clusterKey].Places
			*clusterPlaces = append(*clusterPlaces, *place)
		}
	}
}
