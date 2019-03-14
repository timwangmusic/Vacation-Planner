package graph

import (
	"Vacation-planner/POI"
	"Vacation-planner/iowrappers"
)

type TimeCluster struct {
	places []POI.Place
	timeInterval POI.TimeInterval
}

// returns number of places in a time cluster
func (cls *TimeCluster) Size() int{
	return len(cls.places)
}

type TimeClusters struct {
	clusters map[string]*TimeCluster					// maps interval to time cluster
	timeIntervals POI.GoogleMapsTimeIntervals			// all time intervals
}

// returns number of time intervals in a time-based cluster
func (cls *TimeClusters) Size() int{
	return len(cls.clusters)
}

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
	placeManager.TimeClusters = &TimeClusters{clusters: make(map[string]*TimeCluster, 0)}
	placeManager.TimeClusters.timeIntervals = POI.GoogleMapsTimeIntervals{}
	for _, interval := range timeIntervals{
		clusterKey := interval.Serialize()
		placeManager.TimeClusters.timeIntervals.InsertTimeInterval(interval)
		placeManager.TimeClusters.clusters[clusterKey] = &TimeCluster{timeInterval: interval}
		placeManager.TimeClusters.clusters[clusterKey].places = make([]POI.Place, 0)
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

// assign places to time clusters using their time interval info
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
	for _, interval := range *placeManager.TimeClusters.timeIntervals.GetAllIntervals(){
		if openingInterval.Intersect(&interval){
			clusterKey := interval.Serialize()
			clusterPlaces := &placeManager.TimeClusters.clusters[clusterKey].places
			*clusterPlaces = append(*clusterPlaces, *place)
		}
	}
}
