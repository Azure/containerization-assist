package orchestration

import (
	"context"
	"fmt"
	"sync"
	"time"

	sessiontypes "github.com/Azure/container-copilot/pkg/mcp/internal/session"
	"github.com/rs/zerolog"
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

// JobManager manages async jobs
type JobManager struct {
	jobs   map[string]*AsyncJobInfo
	mutex  sync.RWMutex
	logger zerolog.Logger

	// Worker pool
	workerPool chan struct{}
	maxWorkers int

	// Cleanup
	jobTTL     time.Duration
	shutdownCh chan struct{}
}

// JobManagerConfig contains configuration for the job manager
type JobManagerConfig struct {
	MaxWorkers int           `json:"max_workers"`
	JobTTL     time.Duration `json:"job_ttl"`
	Logger     zerolog.Logger
}

// NewJobManager creates a new job manager
func NewJobManager(config JobManagerConfig) *JobManager {
	if config.MaxWorkers <= 0 {
		config.MaxWorkers = 5
	}
	if config.JobTTL <= 0 {
		config.JobTTL = 1 * time.Hour
	}

	jm := &JobManager{
		jobs:       make(map[string]*AsyncJobInfo),
		logger:     config.Logger,
		workerPool: make(chan struct{}, config.MaxWorkers),
		maxWorkers: config.MaxWorkers,
		jobTTL:     config.JobTTL,
		shutdownCh: make(chan struct{}),
	}

	// Start cleanup routine
	go jm.cleanupRoutine()

	return jm
}

// CreateJob creates a new job and returns its ID
func (jm *JobManager) CreateJob(jobType JobType, sessionID string, metadata map[string]interface{}) string {
	jm.mutex.Lock()
	defer jm.mutex.Unlock()

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

	jm.jobs[jobID] = job

	jm.logger.Info().
		Str("job_id", jobID).
		Str("type", string(jobType)).
		Str("session_id", sessionID).
		Msg("Created new job")

	return jobID
}

// GetJob retrieves a job by ID
func (jm *JobManager) GetJob(jobID string) (*AsyncJobInfo, error) {
	jm.mutex.RLock()
	defer jm.mutex.RUnlock()

	job, exists := jm.jobs[jobID]
	if !exists {
		return nil, fmt.Errorf("job not found: %s", jobID)
	}

	// Return a copy to avoid race conditions
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

// UpdateJob updates a job's status and information
func (jm *JobManager) UpdateJob(jobID string, updater func(*AsyncJobInfo)) error {
	jm.mutex.Lock()
	defer jm.mutex.Unlock()

	job, exists := jm.jobs[jobID]
	if !exists {
		return fmt.Errorf("job not found: %s", jobID)
	}

	updater(job)

	// Update duration if job is completed
	if job.Status == sessiontypes.JobStatusCompleted || job.Status == sessiontypes.JobStatusFailed {
		if job.StartedAt != nil && job.CompletedAt != nil {
			duration := job.CompletedAt.Sub(*job.StartedAt)
			job.Duration = &duration
		}
	}

	jm.logger.Debug().
		Str("job_id", jobID).
		Str("status", string(job.Status)).
		Float64("progress", job.Progress).
		Msg("Updated job")

	return nil
}

// StartJob queues a job for execution and executes it when a worker becomes available
func (jm *JobManager) StartJob(jobID string, executor func(context.Context, *AsyncJobInfo) error) error {
	// Queue the job for execution (always succeeds)
	go func() {
		// Wait for a worker slot to become available
		jm.workerPool <- struct{}{}
		defer func() {
			<-jm.workerPool // Release worker slot
		}()

		// Update job status to running
		err := jm.UpdateJob(jobID, func(job *AsyncJobInfo) {
			job.Status = sessiontypes.JobStatusRunning
			now := time.Now()
			job.StartedAt = &now
			job.Message = "Job started"
		})
		if err != nil {
			jm.logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to update job status to running")
			return
		}

		// Create context with timeout for job execution
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		job, err := jm.GetJob(jobID)
		if err != nil {
			jm.logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to get job for execution")
			return
		}

		// Execute the job
		execErr := executor(ctx, job)

		// Update job with result
		if err := jm.UpdateJob(jobID, func(job *AsyncJobInfo) {
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
			jm.logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to update job status after execution")
		}

		jm.logger.Info().
			Str("job_id", jobID).
			Str("status", string(job.Status)).
			Err(execErr).
			Msg("Job execution finished")
	}()

	return nil
}

// ListJobs returns all jobs for a session, or all jobs if sessionID is empty
func (jm *JobManager) ListJobs(sessionID string) []*AsyncJobInfo {
	jm.mutex.RLock()
	defer jm.mutex.RUnlock()

	var jobs []*AsyncJobInfo
	for _, job := range jm.jobs {
		// If sessionID is empty, return all jobs; otherwise filter by sessionID
		if sessionID == "" || job.SessionID == sessionID {
			// Return a copy
			jobCopy := *job
			jobs = append(jobs, &jobCopy)
		}
	}

	return jobs
}

// CancelJob cancels a running job
func (jm *JobManager) CancelJob(jobID string) error {
	return jm.UpdateJob(jobID, func(job *AsyncJobInfo) {
		if job.Status == sessiontypes.JobStatusPending || job.Status == sessiontypes.JobStatusRunning {
			job.Status = sessiontypes.JobStatusCancelled
			now := time.Now()
			job.CompletedAt = &now
			job.Message = "Job cancelled"
		}
	})
}

// GetStats returns job manager statistics
func (jm *JobManager) GetStats() *JobManagerStats {
	jm.mutex.RLock()
	defer jm.mutex.RUnlock()

	stats := &JobManagerStats{
		TotalJobs:     len(jm.jobs),
		PendingJobs:   0,
		RunningJobs:   0,
		CompletedJobs: 0,
		FailedJobs:    0,
		CancelledJobs: 0,
		MaxWorkers:    jm.maxWorkers,
	}

	for _, job := range jm.jobs {
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
	stats.AvailableWorkers = jm.maxWorkers - stats.RunningJobs

	return stats
}

// Stop gracefully stops the job manager
func (jm *JobManager) Stop() {
	// Signal shutdown to cleanup routine
	close(jm.shutdownCh)

	// Cancel all pending jobs
	jm.mutex.Lock()
	defer jm.mutex.Unlock()

	for jobID, job := range jm.jobs {
		if job.Status == sessiontypes.JobStatusPending {
			job.Status = sessiontypes.JobStatusCancelled
			now := time.Now()
			job.CompletedAt = &now
			job.Message = "Job cancelled due to server shutdown"
			jm.logger.Info().Str("job_id", jobID).Msg("Cancelled pending job due to shutdown")
		}
	}

	jm.logger.Info().Msg("Job manager stopped")
}

// cleanupRoutine periodically removes old completed jobs
func (jm *JobManager) cleanupRoutine() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			jm.cleanup()
		case <-jm.shutdownCh:
			return
		}
	}
}

// cleanup removes old completed jobs
func (jm *JobManager) cleanup() {
	jm.mutex.Lock()
	defer jm.mutex.Unlock()

	now := time.Now()
	var toDelete []string

	for jobID, job := range jm.jobs {
		// Only cleanup completed/failed/cancelled jobs
		if job.Status == sessiontypes.JobStatusCompleted || job.Status == sessiontypes.JobStatusFailed || job.Status == sessiontypes.JobStatusCancelled {
			if job.CompletedAt != nil && now.Sub(*job.CompletedAt) > jm.jobTTL {
				toDelete = append(toDelete, jobID)
			}
		}
	}

	for _, jobID := range toDelete {
		delete(jm.jobs, jobID)
	}

	if len(toDelete) > 0 {
		jm.logger.Info().
			Int("cleaned_jobs", len(toDelete)).
			Msg("Cleaned up old jobs")
	}
}

// generateJobID generates a unique job ID
func generateJobID() string {
	return fmt.Sprintf("job_%d", time.Now().UnixNano())
}

// JobManagerStats contains statistics about the job manager
type JobManagerStats struct {
	TotalJobs        int `json:"total_jobs"`
	PendingJobs      int `json:"pending_jobs"`
	RunningJobs      int `json:"running_jobs"`
	CompletedJobs    int `json:"completed_jobs"`
	FailedJobs       int `json:"failed_jobs"`
	CancelledJobs    int `json:"cancelled_jobs"`
	AvailableWorkers int `json:"available_workers"`
	MaxWorkers       int `json:"max_workers"`
}
