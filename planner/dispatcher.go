package planner

import (
	"context"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"sync"
)

const NumWorkers = 10

type Dispatcher struct {
	JobQueue chan *iowrappers.Job
	workers  []*PlanningSolutionsWorker
	solver   *Solver
	wg       *sync.WaitGroup
}

func NewDispatcher(s *Solver) *Dispatcher {
	return &Dispatcher{
		JobQueue: make(chan *iowrappers.Job),
		workers:  make([]*PlanningSolutionsWorker, 0),
		solver:   s,
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
