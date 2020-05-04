package main

import (
	"context"
	"github.com/kelseyhightower/envconfig"
	log "github.com/sirupsen/logrus"
	"github.com/weihesdlegend/Vacation-planner/planner"
	"github.com/weihesdlegend/Vacation-planner/utils"
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
	Database struct {
		MongoDBName string `envconfig:"MONGO_DB_NAME" default:"VacationPlanner"`
		MongoDBUrl  string `envconfig:"MONGODB_URI" required:"true"`
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
	myPlanner.Init(conf.MapsClientApiKey, conf.Database.MongoDBUrl, redisURL,
		conf.Redis.RedisStreamName, conf.Database.MongoDBName)
	svr := myPlanner.SetupRouter(conf.Server.ServerPort)

	wg := &sync.WaitGroup{}
	wg.Add(numWorkers)
	// dispatch workers
	for worker := 0; worker < numWorkers; worker++ {
		go myPlanner.ProcessPlanningEvent(worker, wg)
	}

	go func() {
		if err := svr.ListenAndServe(); err != nil {
			log.Fatal(err)
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	// block until receiving interrupting signal
	<-c

	// closing event channel after server shuts down
	close(myPlanner.PlanningEvents)
	wg.Wait()

	defer myPlanner.Destroy()

	// create a deadline for other connections to complete IO
	ctx, cancel := context.WithTimeout(context.Background(), planner.ServerTimeout)
	defer cancel()

	utils.CheckErrImmediate(svr.Shutdown(ctx), utils.LogError)

	log.Info("Server gracefully shut down")
	os.Exit(0)
}

func main() {
	RunServer()
}
