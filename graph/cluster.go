package graph

import (
	"Vacation-planner/POI"
	"Vacation-planner/iowrappers"
	"Vacation-planner/utils"
	"github.com/mpraski/clusters"
)

type Cluster struct{
	places []POI.Place
}

// Size of Cluster returns number of places in a cluster
func (cluster *Cluster) Size() int{
	return len(cluster.places)
}

type PlaceClusters struct{
	Clusters []Cluster
}

// Size of PlaceClusters returns number of clusters in a zone
func (placeClusters *PlaceClusters) Size() int{
	return len(placeClusters.Clusters)
}

type ClustersManager struct{
	Client        *iowrappers.MapsClient
	PlaceClusters *PlaceClusters
	places        []POI.Place
	PlaceCat      POI.PlaceCategory
}

// call Google API to obtain nearby places and extract location data
func (placeManager *ClustersManager) GetGeoLocationData(location string, searchRadius uint) [][]float64{
	places := placeManager.Client.SimpleNearbySearch(location, placeManager.PlaceCat, searchRadius, "")
	placeManager.places = places
	locationData := make([][]float64, len(places))
	for idx, place := range places{
		latlng := place.GetLocation()
		locationData[idx] = []float64{latlng[0], latlng[1]}
	}
	return locationData
}

// train clustering model and assign places to clusters
func (placeManager *ClustersManager) Clustering(geoLocationData *[][]float64, numClusters int){
	// obtain clusterer with number of clusters and distance function
	hardCluster, err := clusters.KMeans(1000, numClusters, utils.HaversineDist)
	utils.CheckErr(err)

	// training
	err = hardCluster.Learn(*geoLocationData)
	utils.CheckErr(err)

	placeClusters := PlaceClusters{}
	placeManager.PlaceClusters = &placeClusters
	placeManager.PlaceClusters.Clusters = make([]Cluster, numClusters)

	// save membership info
	for locationIdx, clusterIdx := range hardCluster.Guesses(){
		curCluster := &placeManager.PlaceClusters.Clusters[clusterIdx-1]
		curCluster.places = append(curCluster.places, placeManager.places[locationIdx])
	}
}
