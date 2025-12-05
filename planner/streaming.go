package planner

import (
	"context"
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
	p.RedisClient.StreamsLogging(context.Background(), p.RedisStreamName, eventData)
}

func (p *MyPlanner) ProcessPlanningEvent(worker int, wg *sync.WaitGroup) {
	defer wg.Done()
	ctx := context.Background()
	for event := range p.PlanningEvents {
		p.RedisClient.CollectPlanningAPIStats(ctx, event, worker)
	}
	iowrappers.Logger.Debugf("recollected resources for worker %d", worker)
}
