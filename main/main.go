package main

import (
	"Vacation-planner/graph"
	"fmt"
)

func main() {
	fmt.Println("Welcome to use the Vacation Planner!")

	// locations data
	SanFrancisco := graph.Point{Lat: 37.773972, Long: -122.431297}
	SanDiego := graph.Point{Lat: 32.715736, Long: -117.161087}
	LosAngeles := graph.Point{Lat: 34.052235, Long: -118.243683}
	LasVagas := graph.Point{Lat: 36.169941, Long: -115.139832}
	nyc := graph.Point{Lat:40.712776, Long:-74.005974}
	Boston := graph.Point{Lat:42.360081, Long:-71.058884}
	Pittsburg := graph.Point{Lat:40.440624, Long: -79.995888}
	MET := graph.Point{Lat: 40.779079, Long: -73.962578}
	//BuenosAires := graph.Point{Lat: -34.603683, Long: -58.381557}

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
