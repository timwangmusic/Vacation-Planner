package planner

import (
	"sync"

	"github.com/weihesdlegend/Vacation-planner/iowrappers"
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
		p.RedisClient.CollectPlanningAPIStats(event, worker)
	}
	iowrappers.Logger.Debugf("recollected resources for worker %d", worker)
}
