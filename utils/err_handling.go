package utils

import 	log "github.com/sirupsen/logrus"

func CheckErr(err error){
	if err != nil{
		log.Fatal(err)
	}
}
