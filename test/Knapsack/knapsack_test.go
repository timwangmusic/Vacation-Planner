package test

import (
	"Vacation-planner/POI"
	"Vacation-planner/matching"
	"Vacation-planner/utils"
	"testing"
)

func TestKnapsack(t *testing.T){
	var priceAllZero bool
	priceAllZero = false
	places := make([]matching.Place, 20, 20)
	err := utils.ReadFromFile("../../test_visit_random_gen.json", &places)
	if err != nil || len(places)==0 {
		t.Error("Json file read error")
	}
	t.Log(len(places))
	t.Log(cap(places))
	for _, p := range places {
		if p.Price != 0.0 {
			priceAllZero = true
		} else {
			//places[j].Price = math.Round(rand.ExpFloat64()*10+10)
		}
		t.Logf("stay time %d", POI.GetStayingTimeForLocationType(p.PlaceType))
	}
	//utils.WriteJsonToFile("../../test_visit_random_gen.json", &places)
	if priceAllZero == false {
		t.Log("All price is Zero.")
	}
	//t.Log(places)
	result := matching.Knapsack(places, 35, 1500)
	if len(result)==0 {
		t.Error("No result is returned.")
	}
	t.Log(result)
}
