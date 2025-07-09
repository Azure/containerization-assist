package workflow

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	sessiontypes "github.com/Azure/container-kit/pkg/mcp/domain/session"
)

// JobType represents different types of jobs
type JobType string

const (
	JobTypeBuild      JobType = "build"
	JobTypeValidation JobType = "validation"
	JobTypePush       JobType = "push"
)

// AsyncJobInfo contains extended information about an async job
type AsyncJobInfo struct {
	JobID       string                 `json:"job_id"`
	Type        JobType                `json:"type"`
	Status      sessiontypes.JobStatus `json:"status"`
	SessionID   string                 `json:"session_id"`
	CreatedAt   time.Time              `json:"created_at"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Duration    *time.Duration         `json:"duration,omitempty"`
	Progress    float64                `json:"progress"` // 0.0 to 1.0
	Message     string                 `json:"message,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Result      map[string]interface{} `json:"result,omitempty"`
	Logs        []string               `json:"logs,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// JobManagerStats contains statistics about the job manager
type JobManagerStats struct {
	TotalJobs        int `json:"total_jobs"`
	PendingJobs      int `json:"pending_jobs"`
	RunningJobs      int `json:"running_jobs"`
	QueuedJobs       int `json:"queued_jobs"`
	CompletedJobs    int `json:"completed_jobs"`
	FailedJobs       int `json:"failed_jobs"`
	CancelledJobs    int `json:"cancelled_jobs"`
	MaxWorkers       int `json:"max_workers"`
	AvailableWorkers int `json:"available_workers"`
}

// JobExecutionService manages async job execution
type JobExecutionService interface {
	// CreateJob creates a new job and returns its ID
	CreateJob(jobType JobType, sessionID string, metadata map[string]interface{}) string

	// GetJob retrieves a job by ID
	GetJob(jobID string) (*AsyncJobInfo, error)

	// UpdateJob updates a job's status and information
	UpdateJob(jobID string, updater func(*AsyncJobInfo)) error

	// StartJob queues a job for execution
	StartJob(jobID string, executor func(context.Context, *AsyncJobInfo) error) error

	// ListJobs returns all jobs for a session
	ListJobs(sessionID string) []*AsyncJobInfo

	// CancelJob cancels a running job
	CancelJob(jobID string) error

	// GetStats returns job execution statistics
	GetStats() *JobManagerStats

	// Stop gracefully stops the service
	Stop()
}

// jobExecutionService implements JobExecutionService
type jobExecutionService struct {
	jobs   map[string]*AsyncJobInfo
	mutex  sync.RWMutex
	logger *slog.Logger

	// Worker pool
	workerPool chan struct{}
	maxWorkers int

	// Cleanup
	jobTTL     time.Duration
	shutdownCh chan struct{}
}

// JobExecutionConfig contains configuration for the job execution service
type JobExecutionConfig struct {
	MaxWorkers int           `json:"max_workers"`
	JobTTL     time.Duration `json:"job_ttl"`
	Logger     *slog.Logger
}

// NewJobExecutionService creates a new job execution service
func NewJobExecutionService(config JobExecutionConfig) JobExecutionService {
	if config.MaxWorkers <= 0 {
		config.MaxWorkers = 5
	}
	if config.JobTTL <= 0 {
		config.JobTTL = 1 * time.Hour
	}

	svc := &jobExecutionService{
		jobs:       make(map[string]*AsyncJobInfo),
		logger:     config.Logger,
		workerPool: make(chan struct{}, config.MaxWorkers),
		maxWorkers: config.MaxWorkers,
		jobTTL:     config.JobTTL,
		shutdownCh: make(chan struct{}),
	}

	// Start cleanup routine
	go svc.cleanupRoutine()

	return svc
}

func (j *jobExecutionService) CreateJob(jobType JobType, sessionID string, metadata map[string]interface{}) string {
	j.mutex.Lock()
	defer j.mutex.Unlock()

	jobID := generateJobID()
	job := &AsyncJobInfo{
		JobID:     jobID,
		Type:      jobType,
		Status:    sessiontypes.JobStatusPending,
		SessionID: sessionID,
		CreatedAt: time.Now(),
		Progress:  0.0,
		Metadata:  metadata,
		Logs:      make([]string, 0),
	}

	j.jobs[jobID] = job

	j.logger.Info("Created new job",
		"job_id", jobID,
		"type", string(jobType),
		"session_id", sessionID)

	return jobID
}

func (j *jobExecutionService) GetJob(jobID string) (*AsyncJobInfo, error) {
	j.mutex.RLock()
	defer j.mutex.RUnlock()

	job, exists := j.jobs[jobID]
	if !exists {
		return nil, errors.NewError().
			Messagef("job not found: %s", jobID).
			Code(errors.CodeResourceNotFound).
			Type(errors.ErrTypeNotFound).
			Context("job_id", jobID).
			WithLocation().
			Build()
	}

	// Return a copy
	jobCopy := *job
	if job.Logs != nil {
		jobCopy.Logs = make([]string, len(job.Logs))
		copy(jobCopy.Logs, job.Logs)
	}
	if job.Result != nil {
		jobCopy.Result = make(map[string]interface{})
		for k, v := range job.Result {
			jobCopy.Result[k] = v
		}
	}
	if job.Metadata != nil {
		jobCopy.Metadata = make(map[string]interface{})
		for k, v := range job.Metadata {
			jobCopy.Metadata[k] = v
		}
	}

	return &jobCopy, nil
}

func (j *jobExecutionService) UpdateJob(jobID string, updater func(*AsyncJobInfo)) error {
	j.mutex.Lock()
	defer j.mutex.Unlock()

	job, exists := j.jobs[jobID]
	if !exists {
		return errors.NewError().
			Messagef("job not found: %s", jobID).
			Code(errors.CodeResourceNotFound).
			Type(errors.ErrTypeNotFound).
			Context("job_id", jobID).
			WithLocation().
			Build()
	}

	updater(job)

	if job.Status == sessiontypes.JobStatusCompleted || job.Status == sessiontypes.JobStatusFailed {
		if job.StartedAt != nil && job.CompletedAt != nil {
			duration := job.CompletedAt.Sub(*job.StartedAt)
			job.Duration = &duration
		}
	}

	j.logger.Debug("Updated job",
		"job_id", jobID,
		"status", string(job.Status),
		"progress", job.Progress)

	return nil
}

func (j *jobExecutionService) StartJob(jobID string, executor func(context.Context, *AsyncJobInfo) error) error {
	// Queue the job for execution (always succeeds)
	go func() {
		// Wait for a worker slot to become available
		j.workerPool <- struct{}{}
		defer func() {
			<-j.workerPool // Release worker slot
		}()

		// Update job status to running
		err := j.UpdateJob(jobID, func(job *AsyncJobInfo) {
			job.Status = sessiontypes.JobStatusRunning
			now := time.Now()
			job.StartedAt = &now
			job.Message = "Job started"
		})
		if err != nil {
			j.logger.Error("Failed to update job status to running", "error", err, "job_id", jobID)
			return
		}

		// Create context with timeout for job execution
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		job, err := j.GetJob(jobID)
		if err != nil {
			j.logger.Error("Failed to get job for execution", "error", err, "job_id", jobID)
			return
		}

		// Execute the job
		execErr := executor(ctx, job)

		// Update job with result
		if err := j.UpdateJob(jobID, func(job *AsyncJobInfo) {
			now := time.Now()
			job.CompletedAt = &now
			job.Progress = 1.0

			if execErr != nil {
				job.Status = sessiontypes.JobStatusFailed
				job.Error = execErr.Error()
				job.Message = "Job failed"
			} else {
				job.Status = sessiontypes.JobStatusCompleted
				job.Message = "Job completed successfully"
			}
		}); err != nil {
			j.logger.Error("Failed to update job status after execution", "error", err, "job_id", jobID)
		}

		j.logger.Info("Job execution finished",
			"job_id", jobID,
			"status", string(job.Status),
			"error", execErr)
	}()

	return nil
}

func (j *jobExecutionService) ListJobs(sessionID string) []*AsyncJobInfo {
	j.mutex.RLock()
	defer j.mutex.RUnlock()

	var jobs []*AsyncJobInfo
	for _, job := range j.jobs {
		// If sessionID is empty, return all jobs; otherwise filter by sessionID
		if sessionID == "" || job.SessionID == sessionID {
			// Return a copy
			jobCopy := *job
			jobs = append(jobs, &jobCopy)
		}
	}

	return jobs
}

func (j *jobExecutionService) CancelJob(jobID string) error {
	return j.UpdateJob(jobID, func(job *AsyncJobInfo) {
		if job.Status == sessiontypes.JobStatusPending || job.Status == sessiontypes.JobStatusRunning {
			job.Status = sessiontypes.JobStatusCancelled
			now := time.Now()
			job.CompletedAt = &now
			job.Message = "Job cancelled"
		}
	})
}

func (j *jobExecutionService) GetStats() *JobManagerStats {
	j.mutex.RLock()
	defer j.mutex.RUnlock()

	stats := &JobManagerStats{
		TotalJobs:     len(j.jobs),
		PendingJobs:   0,
		RunningJobs:   0,
		CompletedJobs: 0,
		FailedJobs:    0,
		CancelledJobs: 0,
		MaxWorkers:    j.maxWorkers,
	}

	for _, job := range j.jobs {
		switch job.Status {
		case sessiontypes.JobStatusPending:
			stats.PendingJobs++
		case sessiontypes.JobStatusRunning:
			stats.RunningJobs++
		case sessiontypes.JobStatusCompleted:
			stats.CompletedJobs++
		case sessiontypes.JobStatusFailed:
			stats.FailedJobs++
		case sessiontypes.JobStatusCancelled:
			stats.CancelledJobs++
		}
	}

	// Available workers = max workers - currently running jobs
	stats.AvailableWorkers = j.maxWorkers - stats.RunningJobs

	return stats
}

// generateJobID generates a unique job ID
func generateJobID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// fallback to timestamp-based ID if crypto random fails
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func (j *jobExecutionService) Stop() {
	// Signal shutdown to cleanup routine
	close(j.shutdownCh)

	// Cancel all pending jobs
	j.mutex.Lock()
	defer j.mutex.Unlock()

	for jobID, job := range j.jobs {
		if job.Status == sessiontypes.JobStatusPending {
			job.Status = sessiontypes.JobStatusCancelled
			now := time.Now()
			job.CompletedAt = &now
			job.Message = "Job cancelled due to server shutdown"
			j.logger.Info("Cancelled pending job due to shutdown", "job_id", jobID)
		}
	}

	j.logger.Info("Job execution service stopped")
}

// cleanupRoutine periodically removes old completed jobs
func (j *jobExecutionService) cleanupRoutine() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			j.cleanup()
		case <-j.shutdownCh:
			return
		}
	}
}

// cleanup removes old completed jobs
func (j *jobExecutionService) cleanup() {
	j.mutex.Lock()
	defer j.mutex.Unlock()

	now := time.Now()
	var toDelete []string

	for jobID, job := range j.jobs {
		// Only cleanup completed/failed/cancelled jobs
		if job.Status == sessiontypes.JobStatusCompleted ||
			job.Status == sessiontypes.JobStatusFailed ||
			job.Status == sessiontypes.JobStatusCancelled {
			if job.CompletedAt != nil && now.Sub(*job.CompletedAt) > j.jobTTL {
				toDelete = append(toDelete, jobID)
			}
		}
	}

	for _, jobID := range toDelete {
		delete(j.jobs, jobID)
	}

	if len(toDelete) > 0 {
		j.logger.Info("Cleaned up old jobs",
			"cleaned_jobs", len(toDelete))
	}
}

// Backward compatibility

// JobManager is a type alias for backward compatibility
// DEPRECATED: Use JobExecutionService instead
type JobManager = jobExecutionService

// JobManagerConfig is a type alias for backward compatibility
// DEPRECATED: Use JobExecutionConfig instead
type JobManagerConfig = JobExecutionConfig

// NewJobManager creates a new job manager (for backward compatibility)
// DEPRECATED: Use NewJobExecutionService instead
func NewJobManager(config JobManagerConfig) JobExecutionService {
	return NewJobExecutionService(config)
}

// NewJobManagerWithServices creates a new job manager (for backward compatibility)
// DEPRECATED: Use NewJobExecutionService instead
func NewJobManagerWithServices(logger *slog.Logger) (JobExecutionService, error) {
	config := JobExecutionConfig{
		MaxWorkers: 5,
		JobTTL:     1 * time.Hour,
		Logger:     logger.With("component", "job_execution_service"),
	}
	return NewJobExecutionService(config), nil
}
