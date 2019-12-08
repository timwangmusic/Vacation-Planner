package planner

type PlanningEvent struct {
	City    string `json:"city"`
	Country string `json:"country"`
}

func (planner MyPlanner) PlanningEventLogging(event PlanningEvent) {
	eventData := map[string]string{
		"city":    event.City,
		"country": event.Country,
	}
	planner.RedisClient.StreamsLogging(planner.RedisStreamName, eventData)
}
