package planner

import (
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func (planner *MyPlanner) planningEventLogging(event iowrappers.PlanningEvent) {
	eventData := map[string]string{
		"user":      event.User,
		"city":      event.City,
		"country":   event.Country,
		"timestamp": event.Timestamp,
	}
	planner.RedisClient.StreamsLogging(planner.RedisStreamName, eventData)
}

func (planner *MyPlanner) ProcessPlanningEvent(worker int, wg *sync.WaitGroup) {
	for event := range planner.PlanningEvents {
		c := cases.Title(language.English)
		log.Debugf("worker %d processing event for %s", worker, c.String(event.City)+", "+strings.ToUpper(event.Country))
		planner.RedisClient.CollectPlanningAPIStats(event)
	}
	wg.Done()
}
