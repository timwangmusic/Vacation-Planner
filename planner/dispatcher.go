package planner

import (
	"context"
	"sync"

	"github.com/weihesdlegend/Vacation-planner/iowrappers"
)

const NumWorkers = 10

type Dispatcher struct {
	JobQueue chan *iowrappers.Job
	workers  []*PlanningSolutionsWorker
	solver   *Solver
	c        *iowrappers.RedisClient
	wg       *sync.WaitGroup
}

func NewDispatcher(s *Solver, c *iowrappers.RedisClient) *Dispatcher {
	return &Dispatcher{
		JobQueue: make(chan *iowrappers.Job),
		workers:  make([]*PlanningSolutionsWorker, 0),
		solver:   s,
		c:        c,
		wg:       &sync.WaitGroup{},
	}
}

func (d *Dispatcher) Run(ctx context.Context) {
	mu := &sync.RWMutex{}
	ctx = context.WithValue(ctx, iowrappers.ContextRequestUserId, "worker")
	d.wg.Add(NumWorkers)
	for i := 0; i < NumWorkers; i++ {
		w := &PlanningSolutionsWorker{
			idx:      i,
			s:        d.solver,
			c:        d.c,
			jobQueue: d.JobQueue,
			wg:       d.wg,
		}
		d.workers = append(d.workers, w)
		w.Run(ctx, mu)
	}
}

func (d *Dispatcher) Wait() {
	d.wg.Wait()
}

func (d *Dispatcher) Stop() {
	go func() {
		close(d.JobQueue)
	}()
}
