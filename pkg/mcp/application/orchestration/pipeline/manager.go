package pipeline

import (
	"context"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/core/config"
	"github.com/Azure/container-kit/pkg/mcp/errors"
	"github.com/rs/zerolog"
)

// Manager represents the main pipeline manager that coordinates workers and jobs
type Manager struct {
	workerManager   *BackgroundWorkerManager
	jobOrchestrator *JobOrchestrator
	config          *PipelineConfig
	logger          zerolog.Logger
	ctx             context.Context
	cancel          context.CancelFunc
	mu              sync.RWMutex
	isRunning       bool
}

// NewManager creates a new pipeline manager
func NewManager(logger zerolog.Logger) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	pipelineConfig := DefaultPipelineConfig()

	workerConfig := &config.WorkerConfig{
		ShutdownTimeout:   30 * time.Second,
		HealthCheckPeriod: pipelineConfig.HealthCheckInterval,
		MaxRetries:        3,
	}

	workerManager := NewBackgroundWorkerManager(workerConfig)

	jobOrchestrator := NewJobOrchestrator(workerManager, pipelineConfig)

	return &Manager{
		workerManager:   workerManager,
		jobOrchestrator: jobOrchestrator,
		config:          pipelineConfig,
		logger:          logger,
		ctx:             ctx,
		cancel:          cancel,
		isRunning:       false,
	}
}

// Start starts the pipeline manager
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.isRunning {
		return errors.NewError().Messagef("pipeline manager is already running").WithLocation().Build()
	}

	m.logger.Info().Msg("Starting pipeline manager")

	if err := m.workerManager.StartAll(); err != nil {
		return errors.NewError().Message("failed to start worker manager").Cause(err).WithLocation().Build()
	}

	if err := m.jobOrchestrator.Start(); err != nil {
		return errors.NewError().Message("failed to start job orchestrator").Cause(err).WithLocation().Build()
	}

	m.isRunning = true
	m.logger.Info().Msg("Pipeline manager started successfully")

	return nil
}

// Stop stops the pipeline manager
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.isRunning {
		return nil
	}

	m.logger.Info().Msg("Stopping pipeline manager")

	if err := m.jobOrchestrator.Stop(); err != nil {
		m.logger.Error().Err(err).Msg("Error stopping job orchestrator")
	}

	if err := m.workerManager.StopAll(); err != nil {
		m.logger.Error().Err(err).Msg("Error stopping worker manager")
	}

	m.cancel()

	m.isRunning = false
	m.logger.Info().Msg("Pipeline manager stopped")

	return nil
}

// IsRunning returns whether the pipeline manager is running
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isRunning
}

// RegisterWorker registers a new background worker
func (m *Manager) RegisterWorker(worker BackgroundWorker) error {
	return m.workerManager.RegisterWorker(worker)
}

// UnregisterWorker removes a background worker
func (m *Manager) UnregisterWorker(name string) error {
	return m.workerManager.UnregisterWorker(name)
}

// SubmitJob submits a new job for execution
func (m *Manager) SubmitJob(job *Job) error {
	return m.jobOrchestrator.SubmitJob(job)
}

// GetJob retrieves a job by ID
func (m *Manager) GetJob(jobID string) (*Job, bool) {
	return m.jobOrchestrator.GetJob(jobID)
}

// ListJobs returns all jobs with optional status filter
func (m *Manager) ListJobs(status JobStatus) []*Job {
	return m.jobOrchestrator.ListJobs(status)
}

// CancelJob cancels a pending or running job
func (m *Manager) CancelJob(jobID string) error {
	return m.jobOrchestrator.CancelJob(jobID)
}

// GetWorkerHealth returns health status for a specific worker
func (m *Manager) GetWorkerHealth(name string) (WorkerHealth, error) {
	return m.workerManager.GetWorkerHealth(name)
}

// GetAllWorkerHealth returns health status for all workers
func (m *Manager) GetAllWorkerHealth() map[string]WorkerHealth {
	return m.workerManager.GetAllWorkerHealth()
}

// GetWorkerStatus returns the current status of a worker
func (m *Manager) GetWorkerStatus(name string) (WorkerStatus, error) {
	return m.workerManager.GetWorkerStatus(name)
}

// GetAllWorkerStatuses returns status for all workers
func (m *Manager) GetAllWorkerStatuses() map[string]WorkerStatus {
	return m.workerManager.GetAllWorkerStatuses()
}

// IsHealthy returns true if all workers are healthy
func (m *Manager) IsHealthy() bool {
	return m.workerManager.IsHealthy()
}

// GetManagerStats returns statistics about the worker manager
func (m *Manager) GetManagerStats() ManagerStats {
	return m.workerManager.GetManagerStats()
}

// GetOrchestratorStats returns statistics about the job orchestrator
func (m *Manager) GetOrchestratorStats() OrchestratorStats {
	return m.jobOrchestrator.GetStats()
}

// RestartWorker stops and starts a worker
func (m *Manager) RestartWorker(name string) error {
	return m.workerManager.RestartWorker(name)
}

// RestartAllWorkers stops and starts all workers
func (m *Manager) RestartAllWorkers() error {
	return m.workerManager.RestartAllWorkers()
}

// GetWorkerNames returns a list of all registered worker names
func (m *Manager) GetWorkerNames() []string {
	return m.workerManager.GetWorkerNames()
}

// GetConfig returns the pipeline configuration
func (m *Manager) GetConfig() *PipelineConfig {
	return m.config
}

// UpdateConfig updates the pipeline configuration
func (m *Manager) UpdateConfig(config *PipelineConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.isRunning {
		return errors.NewError().Messagef("cannot update configuration while pipeline is running").WithLocation().Build()
	}

	m.config = config
	return nil
}

// GetStatus returns the overall status of the pipeline manager
func (m *Manager) GetStatus() PipelineStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	workerStats := m.workerManager.GetManagerStats()
	orchestratorStats := m.jobOrchestrator.GetStats()

	return PipelineStatus{
		IsRunning:     m.isRunning,
		WorkerStats:   workerStats,
		JobStats:      orchestratorStats,
		IsHealthy:     m.workerManager.IsHealthy(),
		WorkerCount:   workerStats.TotalWorkers,
		ActiveJobs:    orchestratorStats.RunningJobs,
		PendingJobs:   orchestratorStats.PendingJobs,
		CompletedJobs: orchestratorStats.CompletedJobs,
		FailedJobs:    orchestratorStats.FailedJobs,
	}
}

// PipelineStatus represents the overall status of the pipeline
type PipelineStatus struct {
	IsRunning     bool              `json:"is_running"`
	WorkerStats   ManagerStats      `json:"worker_stats"`
	JobStats      OrchestratorStats `json:"job_stats"`
	IsHealthy     bool              `json:"is_healthy"`
	WorkerCount   int               `json:"worker_count"`
	ActiveJobs    int               `json:"active_jobs"`
	PendingJobs   int               `json:"pending_jobs"`
	CompletedJobs int               `json:"completed_jobs"`
	FailedJobs    int               `json:"failed_jobs"`
}

// CreateExampleWorker creates an example worker for testing
func CreateExampleWorker(name string, taskFunc func(ctx context.Context) error, interval time.Duration) BackgroundWorker {
	return NewSimpleBackgroundWorker(name, taskFunc, interval)
}
