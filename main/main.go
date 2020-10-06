package main

import (
	"github.com/braintree/manners"
	"github.com/kelseyhightower/envconfig"
	log "github.com/sirupsen/logrus"
	"github.com/weihesdlegend/Vacation-planner/planner"
	"net/url"
	"os"
	"os/signal"
	"sync"
)

const numWorkers = 5

type Config struct {
	Server struct {
		ServerPort string `envconfig:"PORT" default:"10000"`
	}
	Redis struct {
		RedisUrl        string `envconfig:"REDISCLOUD_URL" required:"true"`
		RedisStreamName string `default:"stream:planning_api_usage"`
	}
	MapsClientApiKey string `required:"true" split_words:"true"`
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

	myPlanner := planner.MyPlanner{}
	myPlanner.Init(conf.MapsClientApiKey, redisURL, conf.Redis.RedisStreamName)
	svr := myPlanner.SetupRouter(conf.Server.ServerPort)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)

	graceSvr := manners.NewWithServer(svr)

	go listenForShutDownServer(c, graceSvr, &myPlanner)

	err = graceSvr.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}

	log.Info("Server gracefully shut down")
}

func main() {
	RunServer()
}

func listenForShutDownServer(ch <-chan os.Signal, svr *manners.GracefulServer, myPlanner *planner.MyPlanner) {
	wg := &sync.WaitGroup{}
	wg.Add(numWorkers)
	// dispatch workers
	for worker := 0; worker < numWorkers; worker++ {
		go myPlanner.ProcessPlanningEvent(worker, wg)
	}

	// block and wait for shut-down signal
	<-ch

	// destroy zap logger
	defer myPlanner.Destroy()
	// close worker channels
	close(myPlanner.PlanningEvents)
	wg.Wait()

	svr.Close()
}
