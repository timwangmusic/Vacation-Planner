package planner

import (
	"strings"
	"sync"

	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func (p *MyPlanner) planningEventLogging(event iowrappers.PlanningEvent) {
	eventData := map[string]string{
		"user":      event.User,
		"city":      event.City,
		"country":   event.Country,
		"timestamp": event.Timestamp,
	}
	p.RedisClient.StreamsLogging(p.RedisStreamName, eventData)
}

func (p *MyPlanner) ProcessPlanningEvent(worker int, wg *sync.WaitGroup) {
	defer wg.Done()
	for event := range p.PlanningEvents {
		c := cases.Title(language.English)
		iowrappers.Logger.Debugf("worker %d processing event for %s", worker, c.String(event.City)+", "+strings.ToUpper(event.Country))
		p.RedisClient.CollectPlanningAPIStats(event)
	}
	iowrappers.Logger.Debugf("recollected resources for worker %d", worker)
}
