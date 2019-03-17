package matching

import (
	"Vacation-planner/POI"
	"Vacation-planner/graph"
	"Vacation-planner/utils"
)

//location-based clustering, can be extended to matcher for multi-city planning
type PlacePair struct{
	VisitName string
	EateryName string
	PairPrice int
}

type LocationMatcher struct {
	EateryClusterMgr *graph.ClustersManager
	VisitClusterMgr  *graph.ClustersManager
}

func (m *LocationMatcher) Matching (location string, radius uint) (placePairs []PlacePair){
	//m.createClusterManager()

	/*
		1) get geo location data
		2) clustering
		3) calculate cluster centers
		4) generate place pairs
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

func (m *LocationMatcher) genPlacePairs(clusterPair *clusterCenterPair) (placePairs []PlacePair){
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

func (m *LocationMatcher) createClusterManager(){
	m.EateryClusterMgr = &graph.ClustersManager{PlaceCat: POI.PlaceCategoryEatery}
	m.VisitClusterMgr = &graph.ClustersManager{PlaceCat: POI.PlaceCategoryVisit}
}

func (m *LocationMatcher) clustering(location string, searchRadius uint, numClusters int){
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
