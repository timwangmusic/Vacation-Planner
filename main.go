package main

import (
	"net/url"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/braintree/manners"
	"github.com/kelseyhightower/envconfig"
	log "github.com/sirupsen/logrus"
	"github.com/weihesdlegend/Vacation-planner/planner"
	"gopkg.in/yaml.v3"
)

const numWorkers = 5

type Config struct {
	Server struct {
		Domain     string `envconfig:"DOMAIN"`
		ServerPort string `envconfig:"PORT" default:"10000"`
	}
	Redis struct {
		RedisUrl        string `envconfig:"REDIS_URL" default:"redis://localhost:6379"`
		RedisStreamName string `default:"stream:planning_api_usage"`
	}
	MapsClientApiKey        string `default:"YOUR_GOOGLE_API_KEY" split_words:"true"`
	GoogleOAuthClientID     string `envconfig:"GOOGLE_OAUTH_CLIENT_ID"`
	GoogleOAuthClientSecret string `envconfig:"GOOGLE_OAUTH_CLIENT_SECRET"`
	GeonamesApiKey          string `envconfig:"GEONAMES_API_KEY"`
}

type Configurations struct {
	Server struct {
		GoogleMaps struct {
			DetailedSearchFields []string `yaml:"detailed_search_fields"`
		} `yaml:"google_maps"`
	} `yaml:"server"`
}

// flatten configs as a key-value map
func flattenConfig(configs *Configurations) map[string]interface{} {
	flattenedConfigs := make(map[string]interface{})
	flattenedConfigs["server:google_maps:detailed_search_fields"] = configs.Server.GoogleMaps.DetailedSearchFields
	return flattenedConfigs
}

func RunServer() {
	conf := Config{}
	err := envconfig.Process("", &conf)
	if err != nil {
		log.Fatal(err)
	}

	redisURL, err := url.Parse(conf.Redis.RedisUrl)
	if err != nil {
		log.Fatal(err)
	}

	configFile, configFileReadErr := os.Open("config/config.yml")
	if configFileReadErr != nil {
		log.Fatalf("configs read failure: %v", configFileReadErr)
	}

	configs := &Configurations{}
	configFileDecoder := yaml.NewDecoder(configFile)
	if configFileDecodeErr := configFileDecoder.Decode(configs); configFileDecodeErr != nil {
		log.Fatal(configFileDecodeErr)
	}

	myPlanner := planner.MyPlanner{}

	myPlanner.Init(conf.MapsClientApiKey, redisURL, conf.Redis.RedisStreamName, flattenConfig(configs), conf.GoogleOAuthClientID, conf.GoogleOAuthClientSecret, conf.Server.Domain, conf.GeonamesApiKey)
	svr := myPlanner.SetupRouter(conf.Server.ServerPort)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	graceSvr := manners.NewWithServer(svr)

	go listenForShutDownServer(c, graceSvr, &myPlanner)

	err = graceSvr.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}

	log.Info("Server gracefully shut down.")
}

func main() {
	RunServer()
}

func listenForShutDownServer(ch <-chan os.Signal, svr *manners.GracefulServer, myPlanner *planner.MyPlanner) {
	// destroy zap logger
	defer myPlanner.Destroy()

	wg := &sync.WaitGroup{}
	wg.Add(numWorkers)
	// dispatch workers
	for worker := 0; worker < numWorkers; worker++ {
		go myPlanner.ProcessPlanningEvent(worker, wg)
	}

	go func() {
		// wait for shut-down signal
		<-ch

		// close worker channels
		close(myPlanner.PlanningEvents)
	}()

	wg.Wait()
	svr.Close()
}
