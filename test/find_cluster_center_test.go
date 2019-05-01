package test

import (
	"Vacation-planner/graph"
	"fmt"
	"testing"
)

func TestFindClusterCenter (t *testing.T){
	geolocations := [][]float64{
		{40.440624, -79.995888},
		{37.773972, -122.431297},
		{32.715736, -117.161087},
		{40.712776, -74.005974},
		{34.052235, -118.243683},
		{36.169941, -115.139832},
		{40.779079, -73.962578},
	}

	mgr := graph.ClustersManager{}
	clusterResult := []int{1, 2, 3, 1, 3, 2, 1}
	clusterSizes := []int{3, 2, 2}

	placeClusters := graph.PlaceClusters{}
	mgr.PlaceClusters = &placeClusters
	mgr.PlaceClusters.Clusters = make([]graph.BasicCluster, 3)

	centers := mgr.FindClusterCenter(&geolocations, &clusterResult, &clusterSizes)
	fmt.Println(centers)
}
