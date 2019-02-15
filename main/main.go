package main

import (
	"Vacation-planner/graph"
	"fmt"
)

/*
	The main function opens locations.csv file (used as a database) and close it after it finishes.
	If new locations are added to locations pool, they are written to csv as well.
	Currently no location deletion is supported.
*/
func main() {
	fmt.Println("Welcome to use the Vacation Planner!")

	// locations data
	SanFrancisco := graph.Point{Lat: 37.773972, Long: -122.431297}
	SanDiego := graph.Point{Lat: 32.715736, Long: -117.161087}
	LosAngeles := graph.Point{Lat: 34.052235, Long: -118.243683}
	LasVagas := graph.Point{Lat: 36.169941, Long: -115.139832}
	BuenosAires := graph.Point{Lat: -34.603683, Long: -58.381557}
	nyc := graph.Point{Lat:40.712776, Long:-74.005974}
	Boston := graph.Point{Lat:42.360081, Long:-71.058884}
	Pittsburg := graph.Point{Lat:40.440624, Long: -79.995888}
	MET := graph.Point{Lat: 40.779079, Long: -73.962578}

	locations := make(map[string]graph.Point)

	// fill data structure
	locations["New York City"] = nyc
	locations["Boston"] = Boston
	locations["San Francisco"] = SanFrancisco
	locations["San Diego"] = SanDiego
	locations["Los Angeles"] = LosAngeles
	locations["Las Vagas"] = LasVagas
	locations["Brenos Aires"] = BuenosAires
	locations["Pittsburg"] = Pittsburg

	locations["metropolitan museum of art"] = MET

	// write to csv file, which serves as a basic data store
	//city.WriteLocationsToCsv("locations.csv", locations)
	//
	//fmt.Println("Printing some major cities and their locations...")
	//for name, point := range city.GetLocationsFromCsv("locations.csv"){
	//	fmt.Println(name, " ", point)
	//}

	pitt := graph.Vertex{Location:Pittsburg, Name:"Pitt"}
	sd := graph.Vertex{Location:SanDiego, Name:"SD"}
	lv := graph.Vertex{Location:LasVagas, Name: "Las Vagas"}
	Nyc := graph.Vertex{Location:nyc, Name: "New York City"}
	//vertexes := []*graph.Vertex{&pitt, &sd, &lv, &Nyc}
	//testTreeTraversal(&sd, vertexes)

	// test priority queue interface
	nodes := []graph.Vertex{pitt, sd, lv, Nyc}
	pq := graph.MinPriorityQueue{}

	testPriorityQueueInterface(&pq, nodes)
}

func testTreeTraversal(root *graph.Vertex, nodes []*graph.Vertex){
	mst := graph.MinSpanningTree{Root: root}

	graph.GenerateGraph(nodes, false)

	m := mst.Construct(nodes)

	fmt.Println(mst.PreOrderTraversal(m))
}

func testMinSpanningTree(nodes []*graph.Vertex, limited bool) {
	graph.GenerateGraph(nodes, limited)

	tree := graph.MinSpanningTree{Root: nodes[0]}
	res := tree.Construct(nodes)
	for k, p := range res {
		fmt.Println(k, p.Parent)
	}
}

func testMinPriorityQueue(nodes []graph.Vertex) {
	q := graph.MinPriorityQueue{}
	for _, node := range nodes {
		q.Insert(node)
	}

	for i := 0; i < len(nodes); i++ {
		cur := q.ExtractTop() // node name
		fmt.Println(cur)
	}
}

func testPriorityQueueInterface(pq graph.PriorityQueue, nodes []graph.Vertex){
	for _, node := range nodes{
		pq.Insert(node)
	}

	fmt.Println("root is: ", pq.GetRoot().Name)

	for i:=0; i < len(nodes); i++{
		cur := pq.ExtractTop()
		fmt.Println(cur)
	}
}
