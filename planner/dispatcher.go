package planner

import (
	"context"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
)

const NumWorkers = 10

type Dispatcher struct {
	JobQueue    chan *iowrappers.Job
	workers     []*PlanningSolutionsWorker
	done        chan bool
	solver      *Solver
	redisClient *iowrappers.RedisClient
}

func NewDispatcher(s *Solver, c *iowrappers.RedisClient) *Dispatcher {
	return &Dispatcher{
		JobQueue:    make(chan *iowrappers.Job),
		workers:     make([]*PlanningSolutionsWorker, 0),
		done:        make(chan bool),
		solver:      s,
		redisClient: c,
	}
}

func (d *Dispatcher) Run(ctx context.Context) {
	for i := 0; i < NumWorkers; i++ {
		w := &PlanningSolutionsWorker{
			idx:      i,
			s:        d.solver,
			c:        d.redisClient,
			jobQueue: d.JobQueue,
		}
		d.workers = append(d.workers, w)
		w.Run(ctx)
	}
}

func (d *Dispatcher) Wait() {
	go func() {
		for {
			select {
			case <-d.done:
				close(d.JobQueue)
			}
		}
	}()
}

func (d *Dispatcher) Stop() {
	go func() {
		d.done <- true
	}()
}
