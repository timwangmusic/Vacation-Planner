package knapsack

import (
	"github.com/stretchr/testify/assert"
	"github.com/weihesdlegend/Vacation-planner/matching"
	"github.com/weihesdlegend/Vacation-planner/utils"
	"testing"
)

func TestKnapsack(t *testing.T) {
	var priceAllZero bool
	priceAllZero = false
	places := make([]matching.Place, 20, 20)
	err := utils.ReadFromFile("../../test_visit_random_gen.json", &places)
	if err != nil || len(places) == 0 {
		t.Error("Json file read error")
	}
	t.Log(len(places))
	t.Log(cap(places))
	for _, p := range places {
		if p.Price != 0.0 {
			priceAllZero = true
		}
	}
	if !priceAllZero {
		t.Error("All price is Zero.")
	}
	result := matching.Knapsack(places, 35, 1500)
	if len(result) == 0 {
		t.Error("No result is returned.")
	}
	result2 := matching.Knapsackv2(places, 35, 1500)
	if len(result) == 0 {
		t.Error("No result is returned by v2")
	}
	for _, p := range result {
		t.Logf("Placename: %s, ID: %s", p.Name, p.PlaceId)
	}
	t.Logf("Knapsack V1 result size: %d", len(result))
	for _, p := range result2 {
		t.Logf("Placename: %s, ID: %s", p.Name, p.PlaceId)
	}
	t.Logf("Knapsack V2 result size: %d", len(result2))
	if len(result) != len(result2) {
		t.Error("v2 result doesn't match")
	}
	for i := range result {
		if result[i].PlaceId != result2[i].PlaceId {
			t.Error("v2 result is not the same")
		}
	}
	assert.Equal(t,result2[0].PlaceId, "ChIJkwQn2FnxNIgRXbZ_Wu4cdL0", "Assert result[0] is expected")
	assert.Equal(t,result2[2].PlaceId, "ChIJr7aBzePzNIgRi2Dp2ZCFmUY", "Assert result[2] is expected")
	assert.Equal(t,result2[4].PlaceId, "ChIJTfpr7Qj0NIgRdO4BjOIRB0c", "Assert result[4] is expected")
	print(result)
}
