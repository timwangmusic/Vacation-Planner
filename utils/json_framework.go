package utils

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

/*
Base interface, can be extended to arbitrary structs/Json
Objects.
 */

type Place struct{
	/*FIXME: Need to set private variables and did
	tedious set/get functions?
	*/
	Location string `json:location`
	Name string `json:name`
	Addr string	`json: addr`
	LocationType string `json:locationtype`
	PlaceId	string `json:placeid`
	PriceLevel int	`json:pricelevel`
}

func ReadFromFile(fname string, ptr interface{}) error{
	if fname == "" {
		/*FIXME: Need directory management fucntions*/
		fname = "create_visit_location_test_001.json"
	}
	jsonFile, err := os.Open(fname)
	if err != nil {
		/*FIXME: Need to be integrated to native log functions*/
		fmt.Println(err)
		return err
	}
	/*FIXME: How to handle this error*/
	defer jsonFile.Close()

	byteValue, _ := ioutil.ReadAll(jsonFile)
	err = json.Unmarshal(byteValue, ptr)
	if err != nil {
		/*FIXME: Need to be integrated to native log functions*/
		fmt.Println(err)
		return err
	}
	return nil
}



