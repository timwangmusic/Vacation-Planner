package iowrappers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/modern-go/reflect2"
	"time"
)

type JobStatus string

const (
	JobStatusNew        JobStatus = "new"
	JobStatusCreated    JobStatus = "created"
	JobStatusDuplicated JobStatus = "duplicated"
	JobStatusRunning    JobStatus = "running"
	JobStatusFailed     JobStatus = "failed"
	JobStatusCompleted  JobStatus = "completed"
	JobStatusUnknown    JobStatus = "unknown"

	JobExpirationTime = 24 * time.Hour
)

type Job struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
	Status      JobStatus   `json:"status"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// JobExecution helps the service to determine if the job result is still valid
type JobExecution struct {
	JobID     string    `json:"job_id"`
	Status    JobStatus `json:"status"`
	ExpiresAt time.Time `json:"expires_at"`
}

const JobRedisKeyPrefix = "job:"

func (r *RedisClient) UpdateJob(ctx context.Context, job *Job) error {
	if reflect2.IsNil(job) {
		return errors.New("job cannot be nil")
	}

	key := JobRedisKeyPrefix + job.ID
	curTime := time.Now()
	if exists, err := r.Get().Exists(ctx, key).Result(); err != nil {
		return err
	} else if exists == 0 {
		job.CreatedAt = curTime
	} else {
		job.UpdatedAt = curTime
	}

	data, err := json.Marshal(job)
	if err != nil {
		return err
	}

	return r.Get().Set(ctx, key, string(data), JobExpirationTime).Err()
}

func (r *RedisClient) GetJob(ctx context.Context, id string) (*Job, error) {
	key := JobRedisKeyPrefix + id
	if exists, err := r.Get().Exists(ctx, key).Result(); err != nil {
		return nil, err
	} else if exists == 0 {
		return nil, fmt.Errorf("job %s does not exist", id)
	}

	result, err := r.Get().Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	job := new(Job)
	if err = json.Unmarshal([]byte(result), job); err != nil {
		return nil, err
	}
	return job, nil
}

func (r *RedisClient) DeleteJob(ctx context.Context, id string) error {
	key := JobRedisKeyPrefix + id
	return r.Get().Del(ctx, key).Err()
}
