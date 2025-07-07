package pipeline

import (
	"context"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/types/config"
)

// Job represents a unit of work in the pipeline
type Job struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Status      JobStatus              `json:"status"`
	Parameters  map[string]interface{} `json:"parameters"`
	Result      map[string]interface{} `json:"result,omitempty"`
	Error       string                 `json:"error,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
}

// JobStatus represents the status of a job
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCancelled JobStatus = "cancelled"
)

// JobOrchestrator manages job execution across workers
type JobOrchestrator struct {
	jobs       map[string]*Job
	mutex      sync.RWMutex
	workers    *BackgroundWorkerManager
	config     *PipelineConfig
	jobChannel chan *Job
	ctx        context.Context
	cancel     context.CancelFunc
}

// PipelineConfig represents pipeline-specific configuration
type PipelineConfig struct {
	WorkerPoolSize      int           `yaml:"worker_pool_size" json:"worker_pool_size"`
	MaxConcurrentJobs   int           `yaml:"max_concurrent_jobs" json:"max_concurrent_jobs"`
	JobTimeout          time.Duration `yaml:"job_timeout" json:"job_timeout"`
	HealthCheckInterval time.Duration `yaml:"health_check_interval" json:"health_check_interval"`
}

// DefaultPipelineConfig returns default configuration using ZETA constants
func DefaultPipelineConfig() *PipelineConfig {
	return &PipelineConfig{
		WorkerPoolSize:      config.DefaultWorkerPoolSize,
		MaxConcurrentJobs:   config.MaxGoroutines,
		JobTimeout:          config.DefaultTimeout,
		HealthCheckInterval: 30 * time.Second,
	}
}

// NewJobOrchestrator creates a new job orchestrator
func NewJobOrchestrator(workers *BackgroundWorkerManager, config *PipelineConfig) *JobOrchestrator {
	ctx, cancel := context.WithCancel(context.Background())

	return &JobOrchestrator{
		jobs:       make(map[string]*Job),
		workers:    workers,
		config:     config,
		jobChannel: make(chan *Job, config.MaxConcurrentJobs),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// SubmitJob submits a new job for execution
func (jo *JobOrchestrator) SubmitJob(job *Job) error {
	jo.mutex.Lock()
	defer jo.mutex.Unlock()

	job.Status = JobStatusPending
	job.CreatedAt = time.Now()
	jo.jobs[job.ID] = job

	select {
	case jo.jobChannel <- job:
		return nil
	default:
		job.Status = JobStatusFailed
		job.Error = "job queue is full"
		return nil
	}
}

// GetJob retrieves a job by ID
func (jo *JobOrchestrator) GetJob(jobID string) (*Job, bool) {
	jo.mutex.RLock()
	defer jo.mutex.RUnlock()

	job, exists := jo.jobs[jobID]
	return job, exists
}

// ListJobs returns all jobs with optional status filter
func (jo *JobOrchestrator) ListJobs(status JobStatus) []*Job {
	jo.mutex.RLock()
	defer jo.mutex.RUnlock()

	var result []*Job
	for _, job := range jo.jobs {
		if status == "" || job.Status == status {
			result = append(result, job)
		}
	}
	return result
}

// CancelJob cancels a pending or running job
func (jo *JobOrchestrator) CancelJob(jobID string) error {
	jo.mutex.Lock()
	defer jo.mutex.Unlock()

	job, exists := jo.jobs[jobID]
	if !exists {
		return nil
	}

	if job.Status == JobStatusPending || job.Status == JobStatusRunning {
		job.Status = JobStatusCancelled
		now := time.Now()
		job.CompletedAt = &now
	}

	return nil
}

// Start starts the job orchestrator
func (jo *JobOrchestrator) Start() error {
	for i := 0; i < jo.config.WorkerPoolSize; i++ {
		go jo.jobProcessor()
	}

	return nil
}

// Stop stops the job orchestrator
func (jo *JobOrchestrator) Stop() error {
	jo.cancel()
	close(jo.jobChannel)
	return nil
}

// jobProcessor processes jobs from the job channel
func (jo *JobOrchestrator) jobProcessor() {
	for {
		select {
		case <-jo.ctx.Done():
			return
		case job, ok := <-jo.jobChannel:
			if !ok {
				return
			}
			jo.processJob(job)
		}
	}
}

// processJob processes a single job
func (jo *JobOrchestrator) processJob(job *Job) {
	jo.mutex.Lock()
	job.Status = JobStatusRunning
	now := time.Now()
	job.StartedAt = &now
	jo.mutex.Unlock()

	ctx, cancel := context.WithTimeout(jo.ctx, jo.config.JobTimeout)
	defer cancel()

	result, err := jo.executeJob(ctx, job)

	jo.mutex.Lock()
	defer jo.mutex.Unlock()

	completed := time.Now()
	job.CompletedAt = &completed

	if err != nil {
		job.Status = JobStatusFailed
		job.Error = err.Error()
	} else {
		job.Status = JobStatusCompleted
		job.Result = result
	}
}

// executeJob executes a job based on its type
func (jo *JobOrchestrator) executeJob(ctx context.Context, job *Job) (map[string]interface{}, error) {

	switch job.Type {
	case "analysis":
		return jo.executeAnalysisJob(ctx, job)
	case "build":
		return jo.executeBuildJob(ctx, job)
	case "deploy":
		return jo.executeDeployJob(ctx, job)
	default:
		return nil, nil
	}
}

// executeAnalysisJob executes an analysis job
func (jo *JobOrchestrator) executeAnalysisJob(ctx context.Context, job *Job) (map[string]interface{}, error) {
	return map[string]interface{}{
		"analyzed": true,
		"files":    10,
	}, nil
}

// executeBuildJob executes a build job
func (jo *JobOrchestrator) executeBuildJob(ctx context.Context, job *Job) (map[string]interface{}, error) {
	return map[string]interface{}{
		"image_id": "sha256:abc123",
		"size":     "100MB",
	}, nil
}

// executeDeployJob executes a deploy job
func (jo *JobOrchestrator) executeDeployJob(ctx context.Context, job *Job) (map[string]interface{}, error) {
	return map[string]interface{}{
		"deployed":  true,
		"namespace": "default",
	}, nil
}

// GetStats returns orchestrator statistics
func (jo *JobOrchestrator) GetStats() OrchestratorStats {
	jo.mutex.RLock()
	defer jo.mutex.RUnlock()

	stats := OrchestratorStats{
		TotalJobs:     len(jo.jobs),
		PendingJobs:   0,
		RunningJobs:   0,
		CompletedJobs: 0,
		FailedJobs:    0,
	}

	for _, job := range jo.jobs {
		switch job.Status {
		case JobStatusPending:
			stats.PendingJobs++
		case JobStatusRunning:
			stats.RunningJobs++
		case JobStatusCompleted:
			stats.CompletedJobs++
		case JobStatusFailed:
			stats.FailedJobs++
		}
	}

	return stats
}

// OrchestratorStats contains statistics about the job orchestrator
type OrchestratorStats struct {
	TotalJobs     int `json:"total_jobs"`
	PendingJobs   int `json:"pending_jobs"`
	RunningJobs   int `json:"running_jobs"`
	CompletedJobs int `json:"completed_jobs"`
	FailedJobs    int `json:"failed_jobs"`
}
