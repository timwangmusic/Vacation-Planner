// This file defines TimeClusters architecture.
// One TimeClusters consists of multiple TimeCluster and time intervals.
// One TimeCluster consists of a set of places and a time interval.
// All TimeClusters are controlled by TimeClustersManager.
// One TimeClustersManager manages one TimeClusters.
// TimeClustersManager representation contains a set of places for ease of data containment.
package graph

import (
	"Vacation-planner/POI"
	"Vacation-planner/iowrappers"
)

// TimeCluster consists of a set of places and a time interval
type TimeCluster struct {
	places []POI.Place
	timeInterval POI.TimeInterval
}

// returns number of places in a time cluster
func (cls *TimeCluster) Size() int{
	return len(cls.places)
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
//To use a TimeClustersManager, fetch places data using GetPlaces and then time clustering with TimeClustering method.
type TimeClustersManager struct{
	Client        *iowrappers.MapsClient
	TimeClusters  *TimeClusters
	places        []POI.Place
	PlaceCat      POI.PlaceCategory
}

// TimeClusterManager initialization
func (placeManager *TimeClustersManager) Init(client *iowrappers.MapsClient, placeCat POI.PlaceCategory,
	timeIntervals []POI.TimeInterval){
	placeManager.Client = client
	placeManager.PlaceCat = placeCat
	placeManager.TimeClusters = &TimeClusters{Clusters: make(map[string]*TimeCluster, 0)}
	placeManager.TimeClusters.TimeIntervals = POI.GoogleMapsTimeIntervals{}
	for _, interval := range timeIntervals{
		clusterKey := interval.Serialize()
		placeManager.TimeClusters.TimeIntervals.InsertTimeInterval(interval)
		placeManager.TimeClusters.Clusters[clusterKey] = &TimeCluster{timeInterval: interval}
		placeManager.TimeClusters.Clusters[clusterKey].places = make([]POI.Place, 0)
	}
}

// call Google API to obtain nearby Places and extract location data
func (placeManager *TimeClustersManager) GetPlaces(location string, searchRadius uint, searchType string){
	var places []POI.Place
	if searchType == ""{
		places = placeManager.Client.SimpleNearbySearch(location, placeManager.PlaceCat, searchRadius, "")
	} else{
		places = placeManager.Client.ExtensiveNearbySearch(location, placeManager.PlaceCat,
			searchRadius, "", 100, 3)
	}
	placeManager.places = places
}

// assign places to time Clusters using their time interval info
func (placeManager *TimeClustersManager) TimeClustering(){
	for _, place := range placeManager.places{
		placeManager.assign(&place)
	}
}

func (placeManager *TimeClustersManager) assign(place *POI.Place){
	openingHour := place.GetHour(POI.DATE_THURSAY)		// get Thursday opening hour
	openingInterval, err := POI.ParseTimeInterval(openingHour)
	if err != nil{
		return
	}
	for _, interval := range *placeManager.TimeClusters.TimeIntervals.GetAllIntervals(){
		if openingInterval.Intersect(&interval){
			clusterKey := interval.Serialize()
			clusterPlaces := &placeManager.TimeClusters.Clusters[clusterKey].places
			*clusterPlaces = append(*clusterPlaces, *place)
		}
	}
}
