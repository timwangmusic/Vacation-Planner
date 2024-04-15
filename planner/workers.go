package planner

import (
	"context"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
)

type Worker interface {
	handleJob(context.Context, *iowrappers.Job) error
}

type PlanningSolutionsWorker struct {
	idx      int
	s        *Solver
	c        *iowrappers.RedisClient
	jobQueue chan *iowrappers.Job
}

func (w *PlanningSolutionsWorker) handleJob(ctx context.Context, job *iowrappers.Job) error {
	req := job.Parameters.(*PlanningRequest)

	job.Status = iowrappers.JobStatusRunning
	err := w.c.UpdateJob(ctx, job)
	if err != nil {
		return err
	}

	resp := w.s.Solve(ctx, req)
	if resp.Err != nil {
		job.Status = iowrappers.JobStatusFailed
		err = w.c.UpdateJob(ctx, job)
		if err != nil {
			return err
		}
		return resp.Err
	}

	job.Status = iowrappers.JobStatusCompleted
	return w.c.UpdateJob(ctx, job)
}

func (w *PlanningSolutionsWorker) Run(ctx context.Context) {
	go func() {
		logger := iowrappers.Logger

		for {
			select {
			case job, ok := <-w.jobQueue:
				if !ok {
					logger.Debugf("worker %d is shutting down", w.idx)
					return
				}
				err := w.handleJob(ctx, job)
				if err != nil {
					logger.Error(err)
					w.jobQueue <- job
					continue
				}
				logger.Debugf("worker successfully handled job %s", job.ID)
			}
		}
	}()
}
