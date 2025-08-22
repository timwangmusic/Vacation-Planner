package planner

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/weihesdlegend/Vacation-planner/iowrappers"
)

// deduplicate job executions
var jobExecutions = make(map[string]*iowrappers.JobExecution)

type Worker interface {
	handleJob(context.Context, *iowrappers.Job) error
}

type PlanningSolutionsWorker struct {
	idx      int
	s        *Solver
	c        *iowrappers.RedisClient
	jobQueue chan *iowrappers.Job
	wg       *sync.WaitGroup
	store    *JobStore
}

func (w *PlanningSolutionsWorker) handleJob(ctx context.Context, job *iowrappers.Job) error {
	defer createJobRecord(ctx, job, w.c)
	req := job.Parameters.(*PlanningRequest)

	jobKey, err := toSolutionKey(req)
	if err != nil {
		return err
	}

	if w.store.shouldSkipJobExecution(jobKey) {
		job.Status = iowrappers.JobStatusDuplicated
		return nil
	}

	if err = w.createJobExecution(jobKey, job); err != nil {
		job.Status = iowrappers.JobStatusFailed
		return err
	}

	if err = w.store.updateJobExecutionStatus(jobKey, iowrappers.JobStatusRunning); err != nil {
		job.Status = iowrappers.JobStatusFailed
		return err
	}

	resp := w.s.Solve(ctx, req)
	if resp.Err != nil {
		job.Status = iowrappers.JobStatusFailed
		if err = w.store.updateJobExecutionStatus(jobKey, iowrappers.JobStatusFailed); err != nil {
			job.Status = iowrappers.JobStatusUnknown
			return err
		}
		return resp.Err
	}

	if err = w.store.updateJobExecutionStatus(jobKey, iowrappers.JobStatusCompleted); err != nil {
		job.Status = iowrappers.JobStatusUnknown
		return err
	}

	job.Status = iowrappers.JobStatusCompleted
	return nil
}

func createJobRecord(ctx context.Context, job *iowrappers.Job, c *iowrappers.RedisClient) {
	logger := iowrappers.Logger
	if job.Status == iowrappers.JobStatusDuplicated {
		logger.Debugf("job %s is duplicated, do not create a record for now.", job.ID)
		return
	}

	if err := c.UpdateJob(ctx, job); err != nil {
		logger.Error(err)
	}
}

func (w *PlanningSolutionsWorker) createJobExecution(jobKey string, job *iowrappers.Job) error {
	w.store.mu.Lock()
	defer w.store.mu.Unlock()
	if _, ok := w.store.execs[jobKey]; ok {
		return fmt.Errorf("job execution already exists: %v", jobKey)
	} else {
		w.store.execs[jobKey] = &iowrappers.JobExecution{
			JobID:     job.ID,
			Status:    iowrappers.JobStatusCreated,
			ExpiresAt: time.Now().Add(iowrappers.JobExpirationTime),
		}
	}
	return nil
}

func (s *JobStore) shouldSkipJobExecution(jobKey string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	curTime := time.Now()
	if execution, ok := s.execs[jobKey]; ok {
		if execution.Status == iowrappers.JobStatusCreated || execution.Status == iowrappers.JobStatusRunning || execution.Status == iowrappers.JobStatusCompleted {
			return execution.ExpiresAt.After(curTime)
		}
	}
	return false
}

func (s *JobStore) updateJobExecutionStatus(jobKey string, newStatus iowrappers.JobStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.execs[jobKey]; ok {
		s.execs[jobKey].Status = newStatus
	} else {
		return fmt.Errorf("job to be updated %s does not exist", jobKey)
	}
	return nil
}

func (w *PlanningSolutionsWorker) Run(ctx context.Context) {
	go func() {
		defer w.wg.Done()
		logger := iowrappers.Logger

		for job := range w.jobQueue {
			err := w.handleJob(ctx, job)
			if err != nil {
				logger.Error(err)
				continue
			}
			logger.Debugf("worker %d successfully handled job %s: %+v", w.idx, job.ID, job.Parameters)
		}

		logger.Debugf("worker %d is shutting down", w.idx)
	}()
}
