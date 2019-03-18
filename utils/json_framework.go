package utils

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"src/github.com/sirupsen/logrus"
)

/*
Base interface, can be extended to arbitrary structs/Json
Objects.
 */

type PlaceInfo struct{
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
		/*TODO: Need directory management fucntions*/
		fname = "create_visit_location_test_001.json"
	}
	jsonFile, err := os.Open(fname)
	if err != nil {
		logrus.Error(err.Error())
		return err
	}
	/*TODO: How to handle this error*/
	defer jsonFile.Close()

	byteValue, _ := ioutil.ReadAll(jsonFile)
	err = json.Unmarshal(byteValue, ptr)
	if err != nil {
		logrus.Error(err.Error())
		return err
	}
	return nil
}

func WriteJsonToFile(fname string, ptr interface{}) error{
	if fname == "" {
		err := errors.New("file name can't be empty")
		logrus.Error(err.Error())
		return err
	}
	//check directory usable?
	if _, err := os.Stat(fname); err == nil {
		err := errors.New("json target file already exists")
		logrus.Error(err.Error())
		return err
	}
	byteValue, err := json.Marshal(ptr)
	if err != nil {
		logrus.Error(err.Error())
		return err
	}
	err = ioutil.WriteFile(fname, byteValue, 0644)
	if err != nil {
		logrus.Error(err.Error())
		return err
	}
	return nil
}
