package planner

import (
	"context"
	"fmt"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"sync"
	"time"
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
}

func (w *PlanningSolutionsWorker) handleJob(ctx context.Context, job *iowrappers.Job, mutex *sync.RWMutex) error {
	defer createJobRecord(ctx, job, w.c)
	req := job.Parameters.(*PlanningRequest)

	jobKey, err := toSolutionKey(req)
	if err != nil {
		return err
	}

	if shouldSkipJobExecution(jobKey, mutex) {
		job.Status = iowrappers.JobStatusDuplicated
		return nil
	}

	if err = createJobExecution(jobKey, job, mutex); err != nil {
		job.Status = iowrappers.JobStatusFailed
		return err
	}

	if err = updateJobExecutionStatus(jobKey, mutex, iowrappers.JobStatusRunning); err != nil {
		job.Status = iowrappers.JobStatusFailed
		return err
	}

	resp := w.s.Solve(ctx, req)
	if resp.Err != nil {
		job.Status = iowrappers.JobStatusFailed
		if err = updateJobExecutionStatus(jobKey, mutex, iowrappers.JobStatusFailed); err != nil {
			job.Status = iowrappers.JobStatusUnknown
			return err
		}
		return resp.Err
	}

	if err = updateJobExecutionStatus(jobKey, mutex, iowrappers.JobStatusCompleted); err != nil {
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

func createJobExecution(jobKey string, job *iowrappers.Job, mutex *sync.RWMutex) error {
	defer mutex.Unlock()
	mutex.Lock()
	if _, ok := jobExecutions[jobKey]; ok {
		return fmt.Errorf("job execution already exists: %v", jobKey)
	} else {
		jobExecutions[jobKey] = &iowrappers.JobExecution{
			JobID:     job.ID,
			Status:    iowrappers.JobStatusCreated,
			ExpiresAt: time.Now().Add(iowrappers.JobExpirationTime),
		}
	}
	return nil
}

func shouldSkipJobExecution(jobKey string, mu *sync.RWMutex) bool {
	defer mu.RUnlock()
	curTime := time.Now()
	mu.RLock()
	if execution, ok := jobExecutions[jobKey]; ok {
		if execution.Status == iowrappers.JobStatusCreated || execution.Status == iowrappers.JobStatusRunning || execution.Status == iowrappers.JobStatusCompleted {
			return execution.ExpiresAt.After(curTime)
		}
	}
	return false
}

func updateJobExecutionStatus(jobKey string, mu *sync.RWMutex, newStatus iowrappers.JobStatus) error {
	defer mu.Unlock()
	mu.Lock()
	if _, ok := jobExecutions[jobKey]; ok {
		jobExecutions[jobKey].Status = newStatus
	} else {
		return fmt.Errorf("job to be updated %s does not exist", jobKey)
	}
	return nil
}

func (w *PlanningSolutionsWorker) Run(ctx context.Context, mu *sync.RWMutex) {
	go func() {
		defer w.wg.Done()
		logger := iowrappers.Logger

		for job := range w.jobQueue {
			err := w.handleJob(ctx, job, mu)
			if err != nil {
				logger.Error(err)
				continue
			}
			logger.Debugf("worker %d successfully handled job %s: %+v", w.idx, job.ID, job.Parameters)
		}

		logger.Debugf("worker %d is shutting down", w.idx)
	}()
}
