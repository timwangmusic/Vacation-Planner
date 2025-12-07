package planner

import (
	"testing"
	"time"

	"github.com/weihesdlegend/Vacation-planner/iowrappers"
)

func TestPriorityJobQueue_BasicEnqueueDequeue(t *testing.T) {
	pq := NewPriorityJobQueue(10)
	defer pq.Close()

	job := &iowrappers.Job{
		ID:       "test-1",
		Name:     "TestJob",
		Priority: iowrappers.JobPriorityNormal,
	}

	// Enqueue job
	if !pq.Enqueue(job) {
		t.Fatal("Failed to enqueue job")
	}

	// Dequeue job
	dequeuedJob := pq.Dequeue()
	if dequeuedJob == nil {
		t.Fatal("Dequeued job is nil")
	}

	if dequeuedJob.ID != job.ID {
		t.Errorf("Expected job ID %s, got %s", job.ID, dequeuedJob.ID)
	}
}

func TestPriorityJobQueue_PriorityOrdering(t *testing.T) {
	pq := NewPriorityJobQueue(10)
	defer pq.Close()

	// Create jobs with different priorities
	lowPriorityJob := &iowrappers.Job{
		ID:       "low-1",
		Name:     "LowPriorityJob",
		Priority: iowrappers.JobPriorityLow,
	}

	normalPriorityJob := &iowrappers.Job{
		ID:       "normal-1",
		Name:     "NormalPriorityJob",
		Priority: iowrappers.JobPriorityNormal,
	}

	highPriorityJob := &iowrappers.Job{
		ID:       "high-1",
		Name:     "HighPriorityJob",
		Priority: iowrappers.JobPriorityHigh,
	}

	// Enqueue in reverse order: low -> normal -> high
	pq.Enqueue(lowPriorityJob)
	pq.Enqueue(normalPriorityJob)
	pq.Enqueue(highPriorityJob)

	// Dequeue should return high priority first
	job1 := pq.Dequeue()
	if job1.Priority != iowrappers.JobPriorityHigh {
		t.Errorf("Expected HIGH priority job first, got priority %d", job1.Priority)
	}

	// Then normal priority
	job2 := pq.Dequeue()
	if job2.Priority != iowrappers.JobPriorityNormal {
		t.Errorf("Expected NORMAL priority job second, got priority %d", job2.Priority)
	}

	// Finally low priority
	job3 := pq.Dequeue()
	if job3.Priority != iowrappers.JobPriorityLow {
		t.Errorf("Expected LOW priority job third, got priority %d", job3.Priority)
	}
}

func TestPriorityJobQueue_MultipleSamePriority(t *testing.T) {
	pq := NewPriorityJobQueue(10)
	defer pq.Close()

	// Enqueue multiple high priority jobs
	for i := 1; i <= 3; i++ {
		job := &iowrappers.Job{
			ID:       string(rune('A' + i - 1)),
			Name:     "HighPriorityJob",
			Priority: iowrappers.JobPriorityHigh,
		}
		pq.Enqueue(job)
	}

	// All dequeued jobs should be high priority
	for i := 0; i < 3; i++ {
		job := pq.Dequeue()
		if job == nil {
			t.Fatalf("Expected job at position %d, got nil", i)
		}
		if job.Priority != iowrappers.JobPriorityHigh {
			t.Errorf("Expected HIGH priority at position %d, got %d", i, job.Priority)
		}
	}
}

func TestPriorityJobQueue_Stats(t *testing.T) {
	pq := NewPriorityJobQueue(10)
	defer pq.Close()

	// Initially empty
	if pq.Len() != 0 {
		t.Errorf("Expected queue length 0, got %d", pq.Len())
	}

	high, normal, low := pq.Stats()
	if high != 0 || normal != 0 || low != 0 {
		t.Errorf("Expected all stats to be 0, got high=%d, normal=%d, low=%d", high, normal, low)
	}

	// Add jobs to different priorities
	pq.Enqueue(&iowrappers.Job{ID: "h1", Priority: iowrappers.JobPriorityHigh})
	pq.Enqueue(&iowrappers.Job{ID: "h2", Priority: iowrappers.JobPriorityHigh})
	pq.Enqueue(&iowrappers.Job{ID: "n1", Priority: iowrappers.JobPriorityNormal})
	pq.Enqueue(&iowrappers.Job{ID: "l1", Priority: iowrappers.JobPriorityLow})

	// Check total length
	if pq.Len() != 4 {
		t.Errorf("Expected queue length 4, got %d", pq.Len())
	}

	// Check individual queue stats
	high, normal, low = pq.Stats()
	if high != 2 {
		t.Errorf("Expected 2 high priority jobs, got %d", high)
	}
	if normal != 1 {
		t.Errorf("Expected 1 normal priority job, got %d", normal)
	}
	if low != 1 {
		t.Errorf("Expected 1 low priority job, got %d", low)
	}
}

func TestPriorityJobQueue_EmptyQueueDequeue(t *testing.T) {
	pq := NewPriorityJobQueue(10)

	// Close queue immediately
	pq.Close()

	// Dequeue from closed empty queue should return nil
	job := pq.Dequeue()
	if job != nil {
		t.Errorf("Expected nil from empty closed queue, got job %v", job)
	}
}

func TestPriorityJobQueue_EnqueueAfterClose(t *testing.T) {
	pq := NewPriorityJobQueue(10)
	pq.Close()

	// Try to enqueue after closing
	job := &iowrappers.Job{
		ID:       "test-1",
		Priority: iowrappers.JobPriorityNormal,
	}

	success := pq.Enqueue(job)
	if success {
		t.Error("Expected Enqueue to return false after queue closure")
	}
}

func TestPriorityJobQueue_ConcurrentEnqueueDequeue(t *testing.T) {
	pq := NewPriorityJobQueue(100)
	defer pq.Close()

	numJobs := 50
	done := make(chan bool)

	// Producer goroutine
	go func() {
		for i := 0; i < numJobs; i++ {
			priority := iowrappers.JobPriorityLow
			if i%3 == 0 {
				priority = iowrappers.JobPriorityHigh
			} else if i%2 == 0 {
				priority = iowrappers.JobPriorityNormal
			}

			job := &iowrappers.Job{
				ID:       string(rune('A' + i)),
				Priority: priority,
			}
			pq.Enqueue(job)
		}
	}()

	// Consumer goroutine
	go func() {
		count := 0
		for count < numJobs {
			job := pq.Dequeue()
			if job != nil {
				count++
			}
		}
		done <- true
	}()

	// Wait for consumer to finish with timeout
	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for concurrent enqueue/dequeue")
	}
}

func TestPriorityJobQueue_MixedPriorityScenario(t *testing.T) {
	pq := NewPriorityJobQueue(20)
	defer pq.Close()

	// Simulate realistic scenario from /planning endpoint
	// User requests price level 2 (HIGH priority)
	// Background jobs for other price levels (LOW priority)

	userRequestedJob := &iowrappers.Job{
		ID:       "user-requested",
		Name:     "Planning",
		Priority: iowrappers.JobPriorityHigh,
	}

	backgroundJobs := []*iowrappers.Job{
		{ID: "bg-1", Name: "Planning", Priority: iowrappers.JobPriorityLow},
		{ID: "bg-2", Name: "Planning", Priority: iowrappers.JobPriorityLow},
		{ID: "bg-3", Name: "Planning", Priority: iowrappers.JobPriorityLow},
		{ID: "bg-4", Name: "Planning", Priority: iowrappers.JobPriorityLow},
	}

	// Enqueue background jobs first
	for _, job := range backgroundJobs {
		pq.Enqueue(job)
	}

	// Then enqueue user-requested job (should jump to front)
	pq.Enqueue(userRequestedJob)

	// First dequeued job should be user-requested (HIGH priority)
	firstJob := pq.Dequeue()
	if firstJob.ID != "user-requested" {
		t.Errorf("Expected user-requested job first, got %s", firstJob.ID)
	}

	// Remaining jobs should be background jobs
	for i := 0; i < 4; i++ {
		job := pq.Dequeue()
		if job.Priority != iowrappers.JobPriorityLow {
			t.Errorf("Expected LOW priority background job at position %d, got priority %d", i, job.Priority)
		}
	}

	// Queue should be empty now
	if pq.Len() != 0 {
		t.Errorf("Expected empty queue, got length %d", pq.Len())
	}
}

func TestPriorityJobQueue_BufferCapacity(t *testing.T) {
	bufferSize := 5
	pq := NewPriorityJobQueue(bufferSize)
	defer pq.Close()

	// Fill up the high priority queue to capacity
	for i := 0; i < bufferSize; i++ {
		job := &iowrappers.Job{
			ID:       string(rune('A' + i)),
			Priority: iowrappers.JobPriorityHigh,
		}
		if !pq.Enqueue(job) {
			t.Fatalf("Failed to enqueue job %d", i)
		}
	}

	// Verify all jobs are in the queue
	high, _, _ := pq.Stats()
	if high != bufferSize {
		t.Errorf("Expected %d high priority jobs, got %d", bufferSize, high)
	}
}

func TestPriorityJobQueue_DefaultPriority(t *testing.T) {
	pq := NewPriorityJobQueue(10)
	defer pq.Close()

	// Create job with no priority set (should default to 0/Low)
	job := &iowrappers.Job{
		ID:   "default-priority",
		Name: "TestJob",
		// Priority not set
	}

	pq.Enqueue(job)

	// Should be placed in normal queue per implementation
	dequeuedJob := pq.Dequeue()
	if dequeuedJob == nil {
		t.Fatal("Failed to dequeue job with default priority")
	}

	if dequeuedJob.ID != job.ID {
		t.Errorf("Expected job ID %s, got %s", job.ID, dequeuedJob.ID)
	}
}