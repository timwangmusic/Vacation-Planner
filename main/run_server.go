package main

import (
	log "github.com/sirupsen/logrus"
	"github.com/weihesdlegend/Vacation-planner/planner"
	"github.com/weihesdlegend/Vacation-planner/utils"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
)

type DevelopmentConfig struct {
	Conf Config `yaml:"development_config"`
}

type Config struct {
	MongoDBUrl       string `yaml:"mongo_url"`
	RedisUrl         string `yaml:"redis_url"`
	MapsClientApiKey string `yaml:"maps_client_api_key"`
	RedisStreamName  string `yaml:"redis_stream_name"`
	ServerPort		 string `yaml:"server_port"`
}

func RunDevServer() {
	conf := DevelopmentConfig{}
  
	ymlFile, err := ioutil.ReadFile("config/server_config.yml")
	utils.CheckErrImmediate(err, utils.LogFatal)
  
	err = yaml.Unmarshal(ymlFile, &conf)
	utils.CheckErrImmediate(err, utils.LogFatal)

	conf.Conf.MapsClientApiKey = os.Getenv("MAPS_CLIENT_API_KEY")
	if conf.Conf.MapsClientApiKey == "" {
		log.Fatal("environment variable maps client API key is not set")
	}
	myPlanner := planner.MyPlanner{}
	conf_ := conf.Conf
	myPlanner.Init(conf_.MapsClientApiKey, conf_.MongoDBUrl, conf_.RedisUrl, conf_.RedisStreamName)
	myPlanner.HandlingRequests(conf_.ServerPort)
}

func main() {
	RunDevServer()
}
