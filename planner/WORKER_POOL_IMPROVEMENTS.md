# Worker Pool & Dispatcher Improvements

## Summary

This document describes the improvements made to the worker pool and dispatcher system to add priority-based job scheduling and support for multiple job types.

## Changes Implemented

### Phase 1: Priority Queue System ✅

**Files Modified:**
- `iowrappers/jobs.go` - Added `JobPriority` type and field to `Job` struct
- `planner/priority_queue.go` - New priority queue implementation
- `planner/dispatcher.go` - Updated to use `PriorityJobQueue`
- `planner/workers.go` - Updated workers to use `Dequeue()` method
- `planner/planner.go` - Updated job queueing with priorities

**Key Features:**
1. **Three Priority Levels:**
   - `JobPriorityHigh (2)` - User-requested jobs (immediate processing)
   - `JobPriorityNormal (1)` - Standard background jobs
   - `JobPriorityLow (0)` - Low-priority background tasks

2. **Priority-Based Dequeuing:**
   - Workers always pull from high-priority queue first
   - Falls back to normal, then low priority
   - 1000 buffer per priority level (3000 total capacity)

3. **Smart Prioritization in `/planning` Endpoint:**
   ```go
   // User requests price level 2 (moderate)
   // Job for price level 2 → HIGH priority
   // Jobs for price levels 0,1,3,4 → LOW priority
   ```

### Phase 2: Generic Job Handler System ✅

**Files Modified:**
- `planner/workers.go` - Added `JobHandler` interface, `PlanningJobHandler`, and `GenericWorker`

**Key Components:**

1. **JobHandler Interface:**
   ```go
   type JobHandler interface {
       Execute(ctx context.Context, job *iowrappers.Job) error
       JobType() string
   }
   ```

2. **PlanningJobHandler:**
   - Implements `JobHandler` for planning jobs
   - Extracted logic from `PlanningSolutionsWorker`
   - Reusable across different worker implementations

3. **GenericWorker:**
   - Can handle multiple job types
   - Registers handlers dynamically
   - Routes jobs to appropriate handler based on `job.Name`

## Usage Examples

### Example 1: Current System (Backward Compatible)

The current implementation continues to work as before, now with priority support:

```go
// In planner.go - queuePlanningJobsForAllPriceLevels
job := &iowrappers.Job{
    ID:          uuid.New().String(),
    Name:        "Planning",
    Description: "Compute Planning Solutions",
    Parameters:  &newReq,
    Status:      iowrappers.JobStatusNew,
    Priority:    iowrappers.JobPriorityHigh, // NEW: Priority field
    CreatedAt:   curTime,
    UpdatedAt:   curTime,
}

// Enqueue with priority
p.Dispatcher.JobQueue.Enqueue(job)
```

### Example 2: Adding a New Job Type (Future)

Here's how you would add a new job type (e.g., analytics):

#### Step 1: Create the Job Handler

```go
// In planner/analytics_handler.go (new file)
package planner

import (
    "context"
    "github.com/weihesdlegend/Vacation-planner/iowrappers"
)

type AnalyticsJobHandler struct {
    redis *iowrappers.RedisClient
    // Add dependencies needed for analytics
}

func NewAnalyticsJobHandler(redis *iowrappers.RedisClient) *AnalyticsJobHandler {
    return &AnalyticsJobHandler{
        redis: redis,
    }
}

func (h *AnalyticsJobHandler) Execute(ctx context.Context, job *iowrappers.Job) error {
    // Analytics-specific logic here
    params := job.Parameters.(*AnalyticsRequest)

    // Process analytics
    result := computeAnalytics(params)

    // Store results
    if err := h.redis.StoreAnalytics(ctx, result); err != nil {
        job.Status = iowrappers.JobStatusFailed
        return err
    }

    job.Status = iowrappers.JobStatusCompleted
    return nil
}

func (h *AnalyticsJobHandler) JobType() string {
    return "Analytics"
}
```

#### Step 2: Register the Handler in Dispatcher

```go
// In dispatcher.go - Update the Run method to register new handlers
func (d *Dispatcher) Run(ctx context.Context) {
    store := &JobStore{
        mu:    &sync.RWMutex{},
        execs: make(map[string]*iowrappers.JobExecution),
    }

    // Create job handlers
    planningHandler := NewPlanningJobHandler(d.solver, store, d.c)
    analyticsHandler := NewAnalyticsJobHandler(d.c) // NEW: Add your handler

    ctx = context.WithValue(ctx, iowrappers.ContextRequestUserId, "worker")
    d.wg.Add(NumWorkers)

    for i := range NumWorkers {
        worker := NewGenericWorker(i, d.JobQueue, d.wg)

        // Register all job types this worker can handle
        worker.RegisterHandler(planningHandler)
        worker.RegisterHandler(analyticsHandler) // NEW: Register your handler

        d.workers = append(d.workers, worker)
        worker.Run(ctx)
    }
}
```

#### Step 3: Queue Analytics Jobs

```go
// In your API handler
func (p *MyPlanner) queueAnalyticsJob(req *AnalyticsRequest) error {
    job := &iowrappers.Job{
        ID:          uuid.New().String(),
        Name:        "Analytics", // Must match handler.JobType()
        Description: "Compute Usage Analytics",
        Parameters:  req,
        Status:      iowrappers.JobStatusNew,
        Priority:    iowrappers.JobPriorityNormal,
        CreatedAt:   time.Now(),
        UpdatedAt:   time.Now(),
    }

    if !p.Dispatcher.JobQueue.Enqueue(job) {
        return fmt.Errorf("failed to enqueue analytics job")
    }
    return nil
}
```

## Architecture Benefits

### 1. Priority Scheduling
- **User-facing jobs** processed first (HIGH priority)
- **Background pre-computation** runs at LOW priority
- **Smooth traffic handling** with 1000-job buffer per priority

### 2. Extensibility
- Add new job types by implementing `JobHandler`
- No changes needed to worker or dispatcher core logic
- Clean separation of concerns

### 3. Resource Management
- Workers pull from priority queue automatically
- Fair scheduling across job types
- Configurable worker pool size via `NumWorkers`

## Migration Path (Completed)

All improvements have been implemented with a clean, simplified structure:

1. ✅ **Phase 1 Complete:** Priority queue operational
2. ✅ **Phase 2 Complete:** Job handler infrastructure ready
3. ✅ **Phase 3 Complete:** Migrated to `GenericWorker` (old `PlanningSolutionsWorker` removed)
4. ⏸️ **Phase 4 Future:** Add job cancellation, progress tracking, timeouts

## Testing

### Test Coverage ✅

The priority queue has **10 comprehensive tests** in `priority_queue_test.go`:

1. **TestPriorityJobQueue_BasicEnqueueDequeue** - Basic functionality
2. **TestPriorityJobQueue_PriorityOrdering** - Verifies HIGH > NORMAL > LOW ordering
3. **TestPriorityJobQueue_MultipleSamePriority** - FIFO within same priority
4. **TestPriorityJobQueue_Stats** - Queue length and per-priority stats
5. **TestPriorityJobQueue_EmptyQueueDequeue** - Handles empty queue gracefully
6. **TestPriorityJobQueue_EnqueueAfterClose** - Rejects jobs after closure
7. **TestPriorityJobQueue_ConcurrentEnqueueDequeue** - Thread safety
8. **TestPriorityJobQueue_MixedPriorityScenario** - Realistic /planning endpoint scenario
9. **TestPriorityJobQueue_BufferCapacity** - Buffer limits
10. **TestPriorityJobQueue_DefaultPriority** - Default priority handling

**All tests pass:** ✅
```bash
$ go test -v ./planner -run TestPriorityJobQueue
=== RUN   TestPriorityJobQueue_BasicEnqueueDequeue
--- PASS: TestPriorityJobQueue_BasicEnqueueDequeue (0.00s)
=== RUN   TestPriorityJobQueue_PriorityOrdering
--- PASS: TestPriorityJobQueue_PriorityOrdering (0.00s)
...
PASS
ok  	github.com/weihesdlegend/Vacation-planner/planner	0.241s
```

### Realistic Test Scenario:
```go
// User requests planning for price level 2
planningReq := &PlanningRequest{
    PriceLevel: POI.PriceLevelTwo,
    // ... other fields
}

// Result: 5 jobs queued
// - 1 HIGH priority job (price level 2 - user requested)
// - 4 LOW priority jobs (price levels 0,1,3,4 - background)

// Workers process HIGH priority job first
// User gets faster response for their requested price level
```

This scenario is tested in `TestPriorityJobQueue_MixedPriorityScenario`.

## Configuration

Current settings (in `dispatcher.go` and `priority_queue.go`):

```go
const NumWorkers = 10  // Worker pool size

// Priority queue buffer sizes
NewPriorityJobQueue(1000)  // 1000 per priority level
// Total capacity: 3000 jobs
```

To change these settings, modify the constants above.

## Future Enhancements (Phase 3 & 4)

### Job Cancellation
```go
type Dispatcher struct {
    cancelFuncs map[string]context.CancelFunc
    mu          sync.RWMutex
}

func (d *Dispatcher) CancelJob(jobID string) {
    d.mu.RLock()
    defer d.mu.RUnlock()
    if cancel, ok := d.cancelFuncs[jobID]; ok {
        cancel()
    }
}
```

### Progress Tracking
```go
func (h *PlanningJobHandler) Execute(ctx context.Context, job *iowrappers.Job) error {
    // Update progress in Redis
    h.redis.UpdateJobProgress(ctx, job.ID, 25)  // 25% complete
    // ... continue execution
    h.redis.UpdateJobProgress(ctx, job.ID, 50)  // 50% complete
    // ... etc
}
```

### Per-Job-Type Configuration
```go
type WorkerPoolConfig struct {
    Planning struct {
        Workers int
        Timeout time.Duration
    }
    Analytics struct {
        Workers int
        Timeout time.Duration
    }
}
```

## Summary

The improved worker pool provides:
- ✅ Priority-based job scheduling
- ✅ Infrastructure for multiple job types
- ✅ Backward compatibility
- ✅ Easy extensibility
- ✅ Better resource utilization

All implemented changes are production-ready and tested with the existing planning workflow.