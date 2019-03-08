package matching

import (
	"Vacation-planner/POI"
	"Vacation-planner/graph"
	"Vacation-planner/utils"
)

/*
	The primary functionality of this package is using support from graph package
to find visit-eatery pairs from results obtained by nearby search.
	Matchers uses service of cluster managers to search nearby places.
Then it returns a list of PlacePair triples (one for each time slot) to planner.
*/

type PlacePair struct{
	VisitName string
	EateryName string
	PairPrice int
}

type Matcher struct {
	EateryClusterMgr *graph.ClustersManager
	VisitClusterMgr  *graph.ClustersManager
}

func (m *Matcher) Matching (location string, radius uint) (placePairs []PlacePair){
	//m.createClusterManager()

	/*
		1) get geo location data
		2) clustering
		3) calculate cluster centers
	*/
	m.clustering(location, radius, 3)

	// pair cluster centers from two categories
	clusterPairs, err := MatchClusterCenters(*m.VisitClusterMgr.ClusterCenters, *m.EateryClusterMgr.ClusterCenters)

	utils.CheckErr(err)

	for _, clusterPair := range clusterPairs{
		placePairs = append(placePairs, m.genPlacePairs(&clusterPair)...)
	}
	return
}

func (m *Matcher) genPlacePairs(clusterPair *clusterCenterPair) (placePairs []PlacePair){
	visitCluster := m.VisitClusterMgr.PlaceClusters.Clusters[clusterPair.VisitIdx]
	eateryCluster := m.EateryClusterMgr.PlaceClusters.Clusters[clusterPair.EateryIdx]

	for _, visitPlace := range visitCluster.Places{
		for _, eateryPlace := range eateryCluster.Places{
			price := checkPrice(visitPlace.GetPriceLevel()) +
				checkPrice(eateryPlace.GetPriceLevel())
			placePairs = append(placePairs,
				PlacePair{visitPlace.GetName(), eateryPlace.GetName(), price})
		}
	}
	return
}

func (m *Matcher) createClusterManager(){
	m.EateryClusterMgr = &graph.ClustersManager{PlaceCat: POI.PlaceCategoryEatery}
	m.VisitClusterMgr = &graph.ClustersManager{PlaceCat: POI.PlaceCategoryVisit}
}

func (m *Matcher) clustering(location string, searchRadius uint, numClusters int){
	// get geolocation data for visit places
	visitData := m.VisitClusterMgr.GetGeoLocationData(location, searchRadius, "simple")
	// clustering for visit places
	visitClusterResult, visitClusterSizes := m.VisitClusterMgr.Clustering(&visitData, numClusters)
	// calculate cluster centers
	m.VisitClusterMgr.FindClusterCenter(&visitData, &visitClusterResult, &visitClusterSizes)


	// get geolocation data for visit places
	eateryData := m.EateryClusterMgr.GetGeoLocationData(location, searchRadius, "simple")
	// clustering for eatery places
	eateryClusterResult, eateryClusterSizes := m.EateryClusterMgr.Clustering(&eateryData, numClusters)
	// calculate cluster centers
	m.EateryClusterMgr.FindClusterCenter(&eateryData, &eateryClusterResult, &eateryClusterSizes)
}
