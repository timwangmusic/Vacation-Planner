package main

import (
	"Vacation-planner/utils"
	"Vacation-planner/POI"
	"Vacation-planner/graph"
	"Vacation-planner/iowrappers"
	"fmt"
	"log"
)

func main() {
	fmt.Println("Welcome to use the Vacation Planner!")

	// locations data
	SanFrancisco := graph.Point{Lat: 37.773972, Lng: -122.431297}
	SanDiego := graph.Point{Lat: 32.715736, Lng: -117.161087}
	LosAngeles := graph.Point{Lat: 34.052235, Lng: -118.243683}
	LasVagas := graph.Point{Lat: 36.169941, Lng: -115.139832}
	nyc := graph.Point{Lat:40.712776, Lng:-74.005974}
	Boston := graph.Point{Lat:42.360081, Lng:-71.058884}
	Pittsburg := graph.Point{Lat:40.440624, Lng: -79.995888}
	MET := graph.Point{Lat: 40.779079, Lng: -73.962578}
	//BuenosAires := graph.Point{Lat: -34.603683, Lng: -58.381557}

	pitt := graph.Vertex{Location:Pittsburg, Name:"Pittsburgh"}
	sd := graph.Vertex{Location:SanDiego, Name:"San Diego"}
	lv := graph.Vertex{Location:LasVagas, Name: "Las Vagas"}
	Nyc := graph.Vertex{Location:nyc, Name: "New York City"}
	boston := graph.Vertex{Location:Boston, Name: "Boston"}
	la := graph.Vertex{Location:LosAngeles, Name: "Los Angeles"}
	sf := graph.Vertex{Location:SanFrancisco, Name: "San Francisco"}
	met := graph.Vertex{Location:MET, Name: "Metropolitan museum"}

	sd.Key = sd.Dist(pitt)
	lv.Key = lv.Dist(pitt)
	Nyc.Key = Nyc.Dist(pitt)
	boston.Key = boston.Dist(pitt)
	la.Key = la.Dist(pitt)
	sf.Key = sf.Dist(pitt)
	met.Key = met.Dist(pitt)

	nodes := []*graph.Vertex{&pitt, &la, &sd, &sf, &lv, &Nyc, &boston, &met}

	//test priority queue interface
	pq := graph.MinPriorityQueue{}
	testPriorityQueueInterface(&pq, nodes)

	// test clustering
	//testClustering("AIzaSyDRkZOKwe521MXspQZnZvR8pwJsh1d5tEY", "visit")
	/*
	Test of Json functions
	 */

	var placeData utils.PlaceInfo
	err := utils.ReadFromFile("", &placeData)
	if err != nil {
		log.Fatal("Unable to read Json file for this test case.")
	}
	if placeData.Name == "" {
		log.Fatal("Name not read at all.")
	}
	place := POI.CreatePlace(placeData.Name, placeData.Location, placeData.Addr,
		placeData.LocationType, placeData.PlaceId, placeData.PriceLevel)
	fmt.Println(place.GetAddress())
	//mapclient := iowrappers.MapsClient{}
	//mapclient.CreateClient("AIzaSyDRkZOKwe521MXspQZnZvR8pwJsh1d5tEY")
	//places := mapclient.ExtensiveNearbySearch("34.052235,-118.243683", "visit", 10000,
	//	"", 200, 10)
	//
	//fmt.Printf("number of places obtained is %d \n", len(places))
	//for _, place := range places{
	//	fmt.Println(place.GetName())
	//}

	//pitt := graph.Vertex{Location:Pittsburg, Name:"Pitt"}
	//sd := graph.Vertex{Location:SanDiego, Name:"SD"}
	//vertexes := []*graph.Vertex{&pitt, &sd}
	//testTreeTraversal(&sd, vertexes)


	//Test MX

	sw1:=graph.SimpleWeight{}
	sw2:=graph.SimpleWeight{}
	sw1.SetWeight(5)
	sw2.SetWeight(6)
	if sw1.Compare(sw2){
		fmt.Println("Simple: Left is NMT Right.")
	} else {
		fmt.Print("Simple: Right is NMR Left")
	}
	//FIXME: graph function mismatch, version control problems
	sbw1 := graph.SimpleBaseWeight{}
	sbw2 := graph.SimpleBaseWeight{}
	sbw1.SetWeight(50, 100.25)
	sbw2.SetWeight(60, 99)
	sbw1.Setcmpflag(graph.PRIORITY_TIME)
	sbw2.Setcmpflag(graph.PRIORITY_BUDGET)
	if sw1.Compare(sw2){
		fmt.Println("Base_Time: Left is NMT Right.")
	} else {
		fmt.Print("Base_Time: Right is NMR Left")
	}

	if sw2.Compare(sw1){
		fmt.Println("Base_Budget: Right is NMT Left.")
	} else {
		fmt.Print("Base_Budget: Left is NMR Right")
	}
}

func testPriorityQueueInterface(pq graph.PriorityQueue, nodes []*graph.Vertex){
	for _, node := range nodes{
		pq.Insert(*node)
	}

	fmt.Println("root is: ", pq.GetRoot().Name)

	for i:=0; i < len(nodes); i++{
		cur := pq.ExtractTop()
		fmt.Println(cur)
	}
}

func testClustering(apiKey string, placeCat POI.PlaceCategory){
	mapClient := iowrappers.MapsClient{}
	mapClient.CreateClient(apiKey)

	clusterManager := graph.ClustersManager{PlaceCat:placeCat, Client: &mapClient}

	locationData := clusterManager.GetGeoLocationData("40.779079,-73.962578", 500, "")

	clusterManager.Clustering(&locationData, 3)

	for k, cluster := range clusterManager.PlaceClusters.Clusters{
		fmt.Printf("The size of cluster %d is %d \n", k, cluster.Size())
	}
}