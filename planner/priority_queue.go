package planner

import (
	"sync"

	"github.com/weihesdlegend/Vacation-planner/iowrappers"
)

// PriorityJobQueue manages jobs across three priority levels
// Workers always pull from high-priority queue first, then normal, then low
type PriorityJobQueue struct {
	high   chan *iowrappers.Job
	normal chan *iowrappers.Job
	low    chan *iowrappers.Job
	closed bool
	mu     sync.RWMutex
}

// NewPriorityJobQueue creates a new priority-based job queue
// bufferSize determines the capacity of each priority channel
func NewPriorityJobQueue(bufferSize int) *PriorityJobQueue {
	return &PriorityJobQueue{
		high:   make(chan *iowrappers.Job, bufferSize),
		normal: make(chan *iowrappers.Job, bufferSize),
		low:    make(chan *iowrappers.Job, bufferSize),
		closed: false,
	}
}

// Enqueue adds a job to the appropriate priority queue
func (pq *PriorityJobQueue) Enqueue(job *iowrappers.Job) bool {
	pq.mu.RLock()
	defer pq.mu.RUnlock()

	if pq.closed {
		return false
	}

	switch job.Priority {
	case iowrappers.JobPriorityHigh:
		pq.high <- job
	case iowrappers.JobPriorityNormal:
		pq.normal <- job
	case iowrappers.JobPriorityLow:
		pq.low <- job
	default:
		// Default to normal priority if not specified
		pq.normal <- job
	}

	return true
}

// Dequeue retrieves the next job, prioritizing high > normal > low
// Returns nil when all queues are closed and empty
func (pq *PriorityJobQueue) Dequeue() *iowrappers.Job {
	for {
		// Try high priority first (non-blocking)
		select {
		case job, ok := <-pq.high:
			if ok {
				return job
			}
			// High queue closed, continue to check others
		default:
			// No high priority jobs available
		}

		// Try normal priority (non-blocking)
		select {
		case job, ok := <-pq.normal:
			if ok {
				return job
			}
			// Normal queue closed, continue to check low
		default:
			// No normal priority jobs available
		}

		// Try low priority (non-blocking)
		select {
		case job, ok := <-pq.low:
			if ok {
				return job
			}
			// Low queue closed, continue to final check
		default:
			// No low priority jobs available
		}

		// If we get here, all queues are empty
		// Block on all queues simultaneously, respecting priority
		select {
		case job, ok := <-pq.high:
			if !ok {
				// High queue closed, check if all closed
				if pq.allClosed() {
					return nil
				}
				continue
			}
			return job
		case job, ok := <-pq.normal:
			if !ok {
				if pq.allClosed() {
					return nil
				}
				continue
			}
			return job
		case job, ok := <-pq.low:
			if !ok {
				if pq.allClosed() {
					return nil
				}
				continue
			}
			return job
		}
	}
}

// Close closes all priority queues
func (pq *PriorityJobQueue) Close() {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if !pq.closed {
		pq.closed = true
		close(pq.high)
		close(pq.normal)
		close(pq.low)
	}
}

// allClosed checks if all channels are closed
func (pq *PriorityJobQueue) allClosed() bool {
	// Check if channels are closed by attempting non-blocking receive
	select {
	case _, ok := <-pq.high:
		if ok {
			return false
		}
	default:
	}

	select {
	case _, ok := <-pq.normal:
		if ok {
			return false
		}
	default:
	}

	select {
	case _, ok := <-pq.low:
		if ok {
			return false
		}
	default:
	}

	return true
}

// Len returns the approximate total number of jobs across all priorities
// Note: This is a snapshot and may not be accurate in concurrent scenarios
func (pq *PriorityJobQueue) Len() int {
	return len(pq.high) + len(pq.normal) + len(pq.low)
}

// Stats returns the number of jobs in each priority queue
func (pq *PriorityJobQueue) Stats() (high, normal, low int) {
	return len(pq.high), len(pq.normal), len(pq.low)
}
