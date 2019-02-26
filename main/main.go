package main

import (
	"Vacation-planner/graph"
	"Vacation-planner/utils"
	"fmt"
	"github.com/mmcloughlin/geohash"
	"github.com/mpraski/clusters"
	"googlemaps.github.io/maps"
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
	//
	testPriorityQueueInterface(&pq, nodes)

	location := "40.779079,-73.962578"
	parsed_location, _ := maps.ParseLatLng(location)
	code1 := geohash.Encode(parsed_location.Lat, parsed_location.Lng)
	fmt.Println(code1)
	fmt.Println(geohash.Neighbors(code1))

	location2 := "32.715736,-117.161087"
	parsed_location_2, _ := maps.ParseLatLng(location2)
	code2 := geohash.Encode(parsed_location_2.Lat, parsed_location_2.Lng)
	fmt.Println(code2)
	fmt.Println(geohash.Neighbors(code2))

	data := [][]float64{{37.773972, -122.431297},
		{32.715736, -117.161087},
		{36.169941, -115.139832},
		{40.779079, -73.962578},
	}

	//observation := []float64{34.052235, -118.243683}
	observation := []float64{42.360081, -71.058884}
	c, _ := clusters.KMeans(100,2, utils.HaversineDist)

	c.Learn(data)

	fmt.Println(c.Predict(observation))
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
