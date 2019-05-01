//Time-based matching
//Matches request from planner for a particular day
package matching

import (
	"Vacation-planner/POI"
	"Vacation-planner/graph"
	"Vacation-planner/iowrappers"
)

type TimeMatcher struct{
	CateringMgr *graph.TimeClustersManager
	TouringMgr 	*graph.TimeClustersManager
}

type TimeSlot struct {
	Slot POI.TimeInterval
}

// Request from planner
type TimeMatchingRequest struct {
	Location  string      // lat,lng
	Radius    uint        // search Radius
	TimeSlots []TimeSlot  // division of day
	Weekday   POI.Weekday // Weekday
}

type Place struct{
	PlaceId 	string	`json:id`
	Name    	string	`json:name`
	CatTag		POI.PlaceCategory  `json:category`
	Address 	string	`json:address`
	Price   	float64 `json:price`
	Rating  	float32 `json:rating`
	Location    [2]float64 `json:geolocation`
}

type PlaceCluster struct{
	Places []Place	`json:places`
	Slot TimeSlot	`json:time slot`
}

// Matching takes requests from planner and a valid client, returns place clusters with time slot
func (matcher *TimeMatcher) Matching(req *TimeMatchingRequest, mapsClient *iowrappers.MapsClient) (PlaceClusters []PlaceCluster){
	// place search and time clustering
	matcher.placeSearch(req, POI.PlaceCategoryEatery, mapsClient)	// search catering
	matcher.placeSearch(req, POI.PlaceCategoryVisit, mapsClient)	// search visit locations

	clusterMap := make(map[string]*PlaceCluster)

	matcher.processCluster(POI.PlaceCategoryEatery, &clusterMap)
	matcher.processCluster(POI.PlaceCategoryVisit, &clusterMap)

	for _, cluster := range clusterMap{
		PlaceClusters = append(PlaceClusters, *cluster)
	}
	return
}

func (matcher *TimeMatcher) processCluster(placeCat POI.PlaceCategory, clusterMap *(map[string]*PlaceCluster)){
	var mgr *graph.TimeClustersManager

	switch placeCat{
	case POI.PlaceCategoryVisit:
		mgr = matcher.TouringMgr
	case POI.PlaceCategoryEatery:
		mgr = matcher.CateringMgr
	default:
		mgr = &graph.TimeClustersManager{PlaceCat: POI.PlaceCategoryVisit}
	}

	for _, timeInterval := range mgr.TimeClusters.TimeIntervals.Intervals{
		clusterKey := timeInterval.Serialize()
		if _, exist := (*clusterMap)[clusterKey]; !exist{
			(*clusterMap)[clusterKey] = &PlaceCluster{Places: make([]Place, 0), Slot: TimeSlot{timeInterval}}
		}
		cluster := mgr.TimeClusters.Clusters[clusterKey]
		for _, place := range cluster.Places{
			((*clusterMap)[clusterKey]).Places = append(((*clusterMap)[clusterKey]).Places, matcher.createPlace(place, placeCat))
		}
	}

}

func (matcher *TimeMatcher) placeSearch(req *TimeMatchingRequest, placeCat POI.PlaceCategory, mapsClient *iowrappers.MapsClient) (Places []Place){
	var mgr *graph.TimeClustersManager

	switch placeCat{
	case POI.PlaceCategoryVisit:
		mgr = matcher.TouringMgr
	case POI.PlaceCategoryEatery:
		mgr = matcher.CateringMgr
	default:
		mgr = &graph.TimeClustersManager{PlaceCat: POI.PlaceCategoryVisit}
	}

	intervals := make([]POI.TimeInterval, 0)
	for _, slot := range req.TimeSlots {
		intervals = append(intervals, slot.Slot)
	}

	// this is how to use TimeClustersManager
	mgr.Init(mapsClient, placeCat, intervals, req.Weekday)
	mgr.PlaceSearch(req.Location, req.Radius, "")
	mgr.Clustering(req.Weekday)

	return
}

func (matcher *TimeMatcher) createPlace(place POI.Place, catTag POI.PlaceCategory) Place {
	Place_ := Place{}
	Place_.PlaceId = place.GetID()
	Place_.Address = place.GetFormattedAddress()
	Place_.Name = place.GetName()
	Place_.Price = checkPrice(place.GetPriceLevel())
	Place_.Rating = place.GetRating()
	Place_.Location = place.GetLocation()
	Place_.CatTag = catTag
	return Place_
}
