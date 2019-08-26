package main

import (
	"Vacation-planner/planner"
	"Vacation-planner/utils"
	"gopkg.in/yaml.v2"
	"io/ioutil"
)

type DevelopmentConfig struct {
	Conf Config `yaml:"development_config"`
}

type Config struct {
	MongoDBUrl       string `yaml:"mongo_url"`
	RedisUrl         string `yaml:"redis_url"`
	MapsClientApiKey string `yaml:"maps_client_api_key"`
	RedisStreamName  string `yaml:"redis_stream_name"`
}

func RunDevServer() {
	conf := DevelopmentConfig{}
	yml_file, err := ioutil.ReadFile("Config/server_config.yml")
	utils.CheckErr(err)

	err = yaml.Unmarshal(yml_file, &conf)
	utils.CheckErr(err)

	myPlanner := planner.MyPlanner{}
	conf_ := conf.Conf
	myPlanner.Init(conf_.MapsClientApiKey, conf_.MongoDBUrl, conf_.RedisUrl, conf_.RedisStreamName)
	myPlanner.HandlingRequests()
}

func main() {
	RunDevServer()
}
