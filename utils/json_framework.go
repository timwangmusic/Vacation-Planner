package utils

import (
	"encoding/json"
	"io"
	"os"

	"github.com/sirupsen/logrus"
)

/*
Base interface, can be extended to arbitrary structs/Json
Objects.
*/

func ReadFromFile(fileName string, ptr interface{}) error {
	if fileName == "" {
		/*TODO: Need directory management functions*/
		fileName = "create_visit_location_test_001.json"
	}
	jsonFile, err := os.Open(fileName)
	if err != nil {
		logrus.Error(err.Error())
		return err
	}
	/*TODO: How to handle this error*/
	defer func(jsonFile *os.File) {
		err := jsonFile.Close()
		if err != nil {
			LogErrorWithLevel(err, LogError)
		}
	}(jsonFile)

	byteValue, _ := io.ReadAll(jsonFile)
	err = json.Unmarshal(byteValue, ptr)
	if err != nil {
		logrus.Error(err.Error())
		return err
	}
	return nil
}
