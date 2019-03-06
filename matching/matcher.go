package matching

import (
	"Vacation-planner/POI"
	"Vacation-planner/graph"
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
	eateryClusterMgr *graph.ClustersManager
	visitClusterMgr *graph.ClustersManager
}

func (m *Matcher) Matching (location string, radius uint) []PlacePair{
	m.createClusterManager()

	m.clustering(location, radius, 3)
}

func (m *Matcher) createClusterManager(){
	m.eateryClusterMgr = &graph.ClustersManager{PlaceCat:POI.PlaceCategoryEatery}
	m.visitClusterMgr = &graph.ClustersManager{PlaceCat: POI.PlaceCategoryVisit}
}

func (m *Matcher) clustering(location string, searchRadius uint, numClusters int){
	// get geolocation data for visit places
	visitData := m.visitClusterMgr.GetGeoLocationData(location, searchRadius, "simple")
	// clustering for visit places
	m.visitClusterMgr.Clustering(&visitData, numClusters)

	// get geolocation data for visit places
	eateryData := m.eateryClusterMgr.GetGeoLocationData(location, searchRadius, "simple")
	// clustering for eatery places
	m.eateryClusterMgr.Clustering(&eateryData, numClusters)
}

