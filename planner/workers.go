package planner

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/weihesdlegend/Vacation-planner/iowrappers"
)

// JobHandler interface allows different job types to be executed by workers
// Implementing this interface enables adding new job types without modifying worker code
type JobHandler interface {
	// Execute processes a job and returns an error if execution fails
	Execute(ctx context.Context, job *iowrappers.Job) error
	// JobType returns the type of job this handler processes
	JobType() string
}

// Helper functions for JobStore

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

// PlanningJobHandler implements JobHandler for planning solution jobs
type PlanningJobHandler struct {
	solver *Solver
	store  *JobStore
	redis  *iowrappers.RedisClient
}

// NewPlanningJobHandler creates a new planning job handler
func NewPlanningJobHandler(solver *Solver, store *JobStore, redis *iowrappers.RedisClient) *PlanningJobHandler {
	return &PlanningJobHandler{
		solver: solver,
		store:  store,
		redis:  redis,
	}
}

// Execute processes a planning job
func (h *PlanningJobHandler) Execute(ctx context.Context, job *iowrappers.Job) error {
	defer createJobRecord(ctx, job, h.redis)

	req, ok := job.Parameters.(*PlanningRequest)
	if !ok {
		return fmt.Errorf("invalid job parameters type for planning job")
	}

	jobKey, err := toSolutionKey(req)
	if err != nil {
		return err
	}

	// Check for duplicate execution
	if h.store.shouldSkipJobExecution(jobKey) {
		job.Status = iowrappers.JobStatusDuplicated
		return nil
	}

	// Create execution record
	if err = h.createJobExecution(jobKey, job); err != nil {
		job.Status = iowrappers.JobStatusFailed
		return err
	}

	// Update status to running
	if err = h.store.updateJobExecutionStatus(jobKey, iowrappers.JobStatusRunning); err != nil {
		job.Status = iowrappers.JobStatusFailed
		return err
	}

	// Execute solver
	resp := h.solver.Solve(ctx, req)
	if resp.Err != nil {
		job.Status = iowrappers.JobStatusFailed
		if err = h.store.updateJobExecutionStatus(jobKey, iowrappers.JobStatusFailed); err != nil {
			job.Status = iowrappers.JobStatusUnknown
			return err
		}
		return resp.Err
	}

	// Mark completed
	if err = h.store.updateJobExecutionStatus(jobKey, iowrappers.JobStatusCompleted); err != nil {
		job.Status = iowrappers.JobStatusUnknown
		return err
	}

	job.Status = iowrappers.JobStatusCompleted
	return nil
}

// JobType returns the job type identifier
func (h *PlanningJobHandler) JobType() string {
	return "Planning"
}

// createJobExecution creates a new job execution record
func (h *PlanningJobHandler) createJobExecution(jobKey string, job *iowrappers.Job) error {
	h.store.mu.Lock()
	defer h.store.mu.Unlock()

	if _, ok := h.store.execs[jobKey]; ok {
		return fmt.Errorf("job execution already exists: %v", jobKey)
	}

	h.store.execs[jobKey] = &iowrappers.JobExecution{
		JobID:     job.ID,
		Status:    iowrappers.JobStatusCreated,
		ExpiresAt: time.Now().Add(iowrappers.JobExpirationTime),
	}

	return nil
}

// GenericWorker is a flexible worker that can handle jobs using registered JobHandlers
type GenericWorker struct {
	idx      int
	jobQueue *PriorityJobQueue
	wg       *sync.WaitGroup
	handlers map[string]JobHandler // Map of job type to handler
}

// NewGenericWorker creates a new generic worker
func NewGenericWorker(idx int, jobQueue *PriorityJobQueue, wg *sync.WaitGroup) *GenericWorker {
	return &GenericWorker{
		idx:      idx,
		jobQueue: jobQueue,
		wg:       wg,
		handlers: make(map[string]JobHandler),
	}
}

// RegisterHandler registers a job handler for a specific job type
func (w *GenericWorker) RegisterHandler(handler JobHandler) {
	w.handlers[handler.JobType()] = handler
}

// Run starts the worker's processing loop
func (w *GenericWorker) Run(ctx context.Context) {
	go func() {
		defer w.wg.Done()
		logger := iowrappers.Logger

		for {
			job := w.jobQueue.Dequeue()
			if job == nil {
				// Queue is closed and empty
				break
			}

			// Find the appropriate handler for this job
			handler, ok := w.handlers[job.Name]
			if !ok {
				logger.Errorf("worker %d: no handler registered for job type %s", w.idx, job.Name)
				job.Status = iowrappers.JobStatusFailed
				continue
			}

			// Execute the job using the handler
			err := handler.Execute(ctx, job)
			if err != nil {
				logger.Error(err)
				continue
			}

			logger.Debugf("worker %d successfully handled %s job %s", w.idx, job.Name, job.ID)
		}

		logger.Debugf("worker %d is shutting down", w.idx)
	}()
}
