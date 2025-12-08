package planner

import (
	"context"
	"sync"

	"github.com/weihesdlegend/Vacation-planner/iowrappers"
)

const NumWorkers = 10

// JobStore tracks and deduplicates concurrent job executions
type JobStore struct {
	mu    *sync.RWMutex
	execs map[string]*iowrappers.JobExecution
}

type Dispatcher struct {
	JobQueue *PriorityJobQueue
	workers  []*GenericWorker
	solver   *Solver
	c        *iowrappers.RedisClient
	wg       *sync.WaitGroup
}

func NewDispatcher(s *Solver, c *iowrappers.RedisClient) *Dispatcher {
	return &Dispatcher{
		JobQueue: NewPriorityJobQueue(1000), // Buffer of 1000 per priority level
		workers:  make([]*GenericWorker, 0),
		solver:   s,
		c:        c,
		wg:       &sync.WaitGroup{},
	}
}

func (d *Dispatcher) Run(ctx context.Context) {
	store := &JobStore{
		mu:    &sync.RWMutex{},
		execs: make(map[string]*iowrappers.JobExecution),
	}

	// Create job handlers
	planningHandler := NewPlanningJobHandler(d.solver, store, d.c)

	ctx = context.WithValue(ctx, iowrappers.ContextRequestUserId, "worker")
	d.wg.Add(NumWorkers)
	for i := range NumWorkers {
		worker := NewGenericWorker(i, d.JobQueue, d.wg)

		// Register job handlers
		worker.RegisterHandler(planningHandler)

		d.workers = append(d.workers, worker)
		worker.Run(ctx)
	}
}

func (d *Dispatcher) Wait() {
	d.wg.Wait()
}

func (d *Dispatcher) Stop() {
	go func() {
		d.JobQueue.Close()
	}()
}
