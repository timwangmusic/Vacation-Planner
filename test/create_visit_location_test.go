package test

import (
	"Vacation-planner/POI"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"testing"
)
type Place struct{
	location string `json:location`
	name string `json:name`
	addr string	`json: addr`
	locationType string `json:locationtype`
	placeId	string `json:placeid`
	priceLevel int	`json:pricelevel`
}

func (_ Place) readFromFile(fname string, placeptr *Place) error{
	if fname == "" {
		fname = "create_visit_location_test_001.json"
	}
	jsonFile, err := os.Open(fname)
	if err != nil {
		/*FIXME: Need to be integrated to native log functions*/
		fmt.Println(err)
		return err
	}

	defer jsonFile.Close()

	byteValue, _ := ioutil.ReadAll(jsonFile)
	err = json.Unmarshal(byteValue, placeptr)
	if err != nil {
		/*FIXME: Need to be integrated to native log functions*/
		fmt.Println(err)
		return err
	}
	return nil
}


func TestCreatePlace(t *testing.T){
	//location := "32.715736,-117.161087"
	name := "lincoln park"
	addr := "450 National Ave, Mountain View, USA, 94043"
	//place := POI.CreatePlace(name, location, addr, "stay", "lincolnpark_mtv", 3)
	var placeData Place
	err := placeData.readFromFile("", &placeData)
	if err != nil {
		log.Fatal("Unable to read Json file for this test case.")
	}
	if placeData.name == "" {
		t.Error("Name not read at all.")
	}
	place := POI.CreatePlace(placeData.name, placeData.location, placeData.addr,
		placeData.locationType, placeData.placeId, placeData.priceLevel)
	if place.GetName() != name{
		t.Errorf("Name setting is not correct. \n Expected: %s, got: %s",
			name, place.GetName())
	}
	if place.GetLocation() != [2]float64{32.715736,-117.161087}{
		t.Errorf("Location setting is not correct.")
	}
	if place.GetType() != "stay" {
		t.Errorf("Type setting is not correct.")
	}
	if place.GetAddress() != addr{
		t.Errorf("Address setting is not correct. \n Expected: %s \n Got: %s",
			addr, place.GetAddress())
	}
	if place.GetPriceLevel() != 3{
		t.Errorf("Price level setting is not correct. \n Expected: %d \n Got: %d",
			3, place.GetPriceLevel())
	}
}
