package main

import (
	"github.com/kelseyhightower/envconfig"
	log "github.com/sirupsen/logrus"
	"github.com/weihesdlegend/Vacation-planner/planner"
	"net/url"
)

type Config struct {
	Server struct {
		ServerPort string `envconfig:"PORT" default:":10000"`
	}
	Database struct {
		MongoDBName string `envconfig:"MONGO_DB_NAME" default:"VacationPlanner"`
		MongoDBUrl  string `envconfig:"MONGODB_URI" required:"true"`
	}
	Redis struct {
		RedisUrl        string `envconfig:"REDISCLOUD_URL" required:"true"`
		RedisStreamName string `default:"stream:planning_api_usage"`
	}
	MapsClientApiKey string `envconfig:"MAPS_CLIENT_API_KEY" required:"true"`
}

func RunDevServer() {
	conf := Config{}
	err := envconfig.Process("", &conf)
	if err != nil {
		log.Fatal(err)
	}

	log.Info(conf.Redis.RedisUrl)
	redisURL, err := url.Parse(conf.Redis.RedisUrl)
	if err != nil {
		log.Fatal(err)
	}

	myPlanner := planner.MyPlanner{}
	myPlanner.Init(conf.MapsClientApiKey, conf.Database.MongoDBName, conf.Database.MongoDBUrl, redisURL,
		conf.Redis.RedisStreamName)
	myPlanner.HandlingRequests(conf.Server.ServerPort)
}

func main() {
	RunDevServer()
}
