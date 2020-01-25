//Time-based matching
//Matches request from planner for a particular day
package matching

import (
	log "github.com/sirupsen/logrus"
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/graph"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"sort"
)

type Matcher interface {
	Matching(req *TimeMatchingRequest) (clusters []PlaceCluster)
}

type TimeMatcher struct {
	PoiSearcher *iowrappers.PoiSearcher
	CateringMgr *graph.TimeClustersManager
	TouringMgr  *graph.TimeClustersManager
}

type TimeSlot struct {
	Slot POI.TimeInterval
}

type TimeMatchingRequest struct {
	Location  string      // city,country
	Radius    uint        // search Radius
	TimeSlots []TimeSlot  // division of day
	Weekday   POI.Weekday // Weekday
}

type Place struct {
	PlaceId   string            `json:"id"`
	Name      string            `json:"name"`
	PlaceType POI.LocationType  `json:"place_type"`
	CatTag    POI.PlaceCategory `json:"category"`
	Address   string            `json:"address"`
	Price     float64           `json:"price"`
	Rating    float32           `json:"rating"`
	Location  [2]float64        `json:"geolocation"`
}

type PlaceCluster struct {
	Places []Place  `json:"places"`
	Slot   TimeSlot `json:"time slot"`
}

func (matcher *TimeMatcher) Init(poiSearcher *iowrappers.PoiSearcher) {
	if poiSearcher == nil {
		log.Fatal("PoiSearcher does not exist")
	}
	matcher.PoiSearcher = poiSearcher
	matcher.CateringMgr = &graph.TimeClustersManager{PlaceCat: POI.PlaceCategoryEatery}
	matcher.TouringMgr = &graph.TimeClustersManager{PlaceCat: POI.PlaceCategoryVisit}
}

func (matcher *TimeMatcher) Matching(req *TimeMatchingRequest) (clusters []PlaceCluster) {
	// place search and time clustering
	matcher.placeSearch(req, POI.PlaceCategoryEatery) // search catering
	matcher.placeSearch(req, POI.PlaceCategoryVisit)  // search visit locations

	clusterMap := make(map[string]*PlaceCluster)

	matcher.timeClustering(POI.PlaceCategoryEatery, clusterMap)
	matcher.timeClustering(POI.PlaceCategoryVisit, clusterMap)

	clusters = make([]PlaceCluster, len(clusterMap))
	timeIntervals := make([]POI.TimeInterval, 0)
	for _, cluster := range clusterMap { // clusters and timeIntervals are of same length
		timeIntervals = append(timeIntervals, cluster.Slot.Slot)
	}
	// sort time intervals in place by start time
	sort.Sort(POI.ByStartTime(timeIntervals))

	for idx, interval := range timeIntervals {
		intervalKey := interval.Serialize()
		clusters[idx] = *clusterMap[intervalKey]
	}

	return
}

func (matcher *TimeMatcher) timeClustering(placeCat POI.PlaceCategory, clusterMap map[string]*PlaceCluster) {
	var mgr *graph.TimeClustersManager

	switch placeCat {
	case POI.PlaceCategoryVisit:
		mgr = matcher.TouringMgr
	case POI.PlaceCategoryEatery:
		mgr = matcher.CateringMgr
	default:
		mgr = &graph.TimeClustersManager{PlaceCat: POI.PlaceCategoryVisit}
	}

	for _, timeInterval := range *mgr.TimeClusters.TimeIntervals.GetAllIntervals() {
		clusterKey := timeInterval.Serialize()
		if _, exist := clusterMap[clusterKey]; !exist {
			clusterMap[clusterKey] = &PlaceCluster{Places: make([]Place, 0), Slot: TimeSlot{timeInterval}}
		}
		cluster := mgr.TimeClusters.Clusters[clusterKey]
		for _, place := range cluster.Places {
			(*clusterMap[clusterKey]).Places = append((*clusterMap[clusterKey]).Places, matcher.createPlace(place, placeCat))
		}
	}

}

func (matcher *TimeMatcher) placeSearch(req *TimeMatchingRequest, placeCat POI.PlaceCategory) {
	var mgr *graph.TimeClustersManager

	switch placeCat {
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
	mgr.Init(matcher.PoiSearcher, placeCat, intervals, req.Weekday)
	mgr.PlaceSearch(req.Location, req.Radius)
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
	Place_.PlaceType = place.LocationType
	return Place_
}
