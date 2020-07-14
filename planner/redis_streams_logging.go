package planner

import (
	log "github.com/sirupsen/logrus"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"strings"
	"sync"
)

func (planner MyPlanner) PlanningEventLogging(event iowrappers.PlanningEvent) {
	eventData := map[string]string{
		"user":      event.User,
		"city":      event.City,
		"country":   event.Country,
		"timestamp": event.Timestamp,
	}
	planner.RedisClient.StreamsLogging(planner.RedisStreamName, eventData)
}

func (planner MyPlanner) ProcessPlanningEvent(worker int, wg *sync.WaitGroup) {
	for event := range planner.PlanningEvents {
		log.Debugf("worker %d processing event for %s", worker, strings.Title(event.City)+", "+strings.ToUpper(event.Country))
		planner.RedisClient.CollectPlanningAPIStats(event)
	}
	wg.Done()
}
