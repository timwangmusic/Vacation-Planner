package graph

import (
	"Vacation-planner/POI"
	"Vacation-planner/iowrappers"
	"Vacation-planner/utils"
	"github.com/mpraski/clusters"
)

type Cluster struct{
	Places []POI.Place
}

// Size of Cluster returns number of Places in a cluster
func (cluster *Cluster) Size() int{
	return len(cluster.Places)
}

type PlaceClusters struct{
	Clusters []Cluster
}

// Size of PlaceClusters returns number of Clusters in a zone
func (placeClusters *PlaceClusters) Size() int{
	return len(placeClusters.Clusters)
}

type ClustersManager struct{
	Client        *iowrappers.MapsClient
	PlaceClusters *PlaceClusters
	places        []POI.Place
	PlaceCat      POI.PlaceCategory
	ClusterCenters *[][]float64
}

// call Google API to obtain nearby Places and extract location data
func (placeManager *ClustersManager) GetGeoLocationData(location string, searchRadius uint, searchType string) [][]float64 {
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

	locationData := make([][]float64, len(placeManager.places))
	for idx, place := range placeManager.places{
		latLng := place.GetLocation()
		locationData[idx] = []float64{latLng[0], latLng[1]}
	}
	return locationData
}

// train clustering model and assign Places to Clusters
// numClusters specifies number of Clusters
func (placeManager *ClustersManager) Clustering(geoLocationData *[][]float64, numClusters int) (clusterResult []int, clusterSizes []int){
	// obtain clusterer with number of Clusters and distance function
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
		curCluster.Places = append(curCluster.Places, placeManager.places[locationIdx])
	}

	clusterResult = hardCluster.Guesses()
	clusterSizes = hardCluster.Sizes()
	return
}

func (placeManager *ClustersManager) FindClusterCenter(geoLocationData *[][]float64, clusterResult *[]int,
	clusterSizes *[]int) [][]float64{
	clusterCenters := make([][]float64, placeManager.PlaceClusters.Size())

	groups := make([][][]float64, placeManager.PlaceClusters.Size())

	for i := 0; i < placeManager.PlaceClusters.Size(); i++{
		groups[i] = [][]float64{}
	}

	for k, cluster := range *clusterResult{
		groups[cluster-1] = append(groups[cluster-1], (*geoLocationData)[k])
	}

	for i := 0; i < placeManager.PlaceClusters.Size(); i++{
		center, err := utils.FindCenter(groups[i])
		utils.CheckErr(err)
		clusterCenters[i] = center
	}
	placeManager.ClusterCenters = &clusterCenters
	return clusterCenters
}
