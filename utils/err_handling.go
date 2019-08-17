package utils

import (
	"errors"
	log "github.com/sirupsen/logrus"
	"runtime/debug"
)

func CheckErr(err error){
	if err != nil{
		log.Error(err)
	}
}
func GenerateErr(errstring string) (err error){
	err = errors.New(errstring)
	debug.PrintStack()
	return
}
