package pipeline

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/config"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// Service provides pipeline orchestration functionality
type Service interface {
	// Lifecycle management
	Start() error
	Stop() error
	IsRunning() bool
	GetStatus() Status

	// Configuration management
	GetConfig() *PipelineConfig
	UpdateConfig(config *PipelineConfig) error

	// Worker management
	RegisterWorker(worker BackgroundWorker) error
	UnregisterWorker(name string) error
	RestartWorker(name string) error
	RestartAllWorkers() error
	GetWorkerNames() []string

	// Health monitoring
	GetWorkerHealth(name string) (WorkerHealth, error)
	GetAllWorkerHealth() map[string]WorkerHealth
	GetWorkerStatus(name string) (WorkerStatus, error)
	GetAllWorkerStatuses() map[string]WorkerStatus
	IsHealthy() bool

	// Job management
	SubmitJob(job *Job) error
	GetJob(jobID string) (*Job, bool)
	ListJobs(status JobStatus) []*Job
	CancelJob(jobID string) error

	// Statistics
	GetManagerStats() ManagerStats
	GetOrchestratorStats() OrchestratorStats
}

// ServiceImpl implements the Service interface
type ServiceImpl struct {
	workerManager   *BackgroundWorkerManager
	jobOrchestrator *JobOrchestrator
	config          *PipelineConfig
	logger          *slog.Logger
	ctx             context.Context
	cancel          context.CancelFunc
	mu              sync.RWMutex
	isRunning       bool
}

// NewPipelineService creates a new pipeline service
func NewPipelineService(logger *slog.Logger) Service {
	ctx, cancel := context.WithCancel(context.Background())

	pipelineConfig := DefaultPipelineConfig()

	workerConfig := &config.WorkerConfig{
		ShutdownTimeout:   30 * time.Second,
		HealthCheckPeriod: pipelineConfig.HealthCheckInterval,
		MaxRetries:        3,
	}

	workerManager := NewBackgroundWorkerManager(workerConfig)
	jobOrchestrator := NewJobOrchestrator(workerManager, pipelineConfig)

	return &ServiceImpl{
		workerManager:   workerManager,
		jobOrchestrator: jobOrchestrator,
		config:          pipelineConfig,
		logger:          logger.With("component", "pipeline_service"),
		ctx:             ctx,
		cancel:          cancel,
		isRunning:       false,
	}
}

// Start starts the pipeline service
func (s *ServiceImpl) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isRunning {
		return errors.NewError().Messagef("pipeline service is already running").WithLocation().Build()
	}

	s.logger.Info("Starting pipeline service")

	if err := s.workerManager.StartAll(); err != nil {
		return errors.NewError().Message("failed to start worker manager").Cause(err).WithLocation().Build()
	}

	if err := s.jobOrchestrator.Start(); err != nil {
		return errors.NewError().Message("failed to start job orchestrator").Cause(err).WithLocation().Build()
	}

	s.isRunning = true
	s.logger.Info("Pipeline service started successfully")

	return nil
}

// Stop stops the pipeline service
func (s *ServiceImpl) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isRunning {
		return nil
	}

	s.logger.Info("Stopping pipeline service")

	if err := s.jobOrchestrator.Stop(); err != nil {
		s.logger.Error("Error stopping job orchestrator", "error", err)
	}

	if err := s.workerManager.StopAll(); err != nil {
		s.logger.Error("Error stopping worker manager", "error", err)
	}

	s.cancel()

	s.isRunning = false
	s.logger.Info("Pipeline service stopped")

	return nil
}

// IsRunning returns whether the pipeline service is running
func (s *ServiceImpl) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isRunning
}

// GetConfig returns the pipeline configuration
func (s *ServiceImpl) GetConfig() *PipelineConfig {
	return s.config
}

// UpdateConfig updates the pipeline configuration
func (s *ServiceImpl) UpdateConfig(config *PipelineConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isRunning {
		return errors.NewError().Messagef("cannot update configuration while pipeline is running").WithLocation().Build()
	}

	s.config = config
	return nil
}

// RegisterWorker registers a new background worker
func (s *ServiceImpl) RegisterWorker(worker BackgroundWorker) error {
	return s.workerManager.RegisterWorker(worker)
}

// UnregisterWorker removes a background worker
func (s *ServiceImpl) UnregisterWorker(name string) error {
	return s.workerManager.UnregisterWorker(name)
}

// RestartWorker stops and starts a worker
func (s *ServiceImpl) RestartWorker(name string) error {
	return s.workerManager.RestartWorker(name)
}

// RestartAllWorkers stops and starts all workers
func (s *ServiceImpl) RestartAllWorkers() error {
	return s.workerManager.RestartAllWorkers()
}

// GetWorkerNames returns a list of all registered worker names
func (s *ServiceImpl) GetWorkerNames() []string {
	return s.workerManager.GetWorkerNames()
}

// GetWorkerHealth returns health status for a specific worker
func (s *ServiceImpl) GetWorkerHealth(name string) (WorkerHealth, error) {
	return s.workerManager.GetWorkerHealth(name)
}

// GetAllWorkerHealth returns health status for all workers
func (s *ServiceImpl) GetAllWorkerHealth() map[string]WorkerHealth {
	return s.workerManager.GetAllWorkerHealth()
}

// GetWorkerStatus returns the current status of a worker
func (s *ServiceImpl) GetWorkerStatus(name string) (WorkerStatus, error) {
	return s.workerManager.GetWorkerStatus(name)
}

// GetAllWorkerStatuses returns status for all workers
func (s *ServiceImpl) GetAllWorkerStatuses() map[string]WorkerStatus {
	return s.workerManager.GetAllWorkerStatuses()
}

// IsHealthy returns true if all workers are healthy
func (s *ServiceImpl) IsHealthy() bool {
	return s.workerManager.IsHealthy()
}

// SubmitJob submits a new job for execution
func (s *ServiceImpl) SubmitJob(job *Job) error {
	return s.jobOrchestrator.SubmitJob(job)
}

// GetJob retrieves a job by ID
func (s *ServiceImpl) GetJob(jobID string) (*Job, bool) {
	return s.jobOrchestrator.GetJob(jobID)
}

// ListJobs returns all jobs with optional status filter
func (s *ServiceImpl) ListJobs(status JobStatus) []*Job {
	return s.jobOrchestrator.ListJobs(status)
}

// CancelJob cancels a pending or running job
func (s *ServiceImpl) CancelJob(jobID string) error {
	return s.jobOrchestrator.CancelJob(jobID)
}

// GetManagerStats returns statistics about the worker manager
func (s *ServiceImpl) GetManagerStats() ManagerStats {
	return s.workerManager.GetManagerStats()
}

// GetOrchestratorStats returns statistics about the job orchestrator
func (s *ServiceImpl) GetOrchestratorStats() OrchestratorStats {
	return s.jobOrchestrator.GetStats()
}

// GetStatus returns the overall status of the pipeline service
func (s *ServiceImpl) GetStatus() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()

	workerStats := s.workerManager.GetManagerStats()
	orchestratorStats := s.jobOrchestrator.GetStats()

	return Status{
		IsRunning:     s.isRunning,
		WorkerStats:   workerStats,
		JobStats:      orchestratorStats,
		IsHealthy:     s.workerManager.IsHealthy(),
		WorkerCount:   workerStats.TotalWorkers,
		ActiveJobs:    orchestratorStats.RunningJobs,
		PendingJobs:   orchestratorStats.PendingJobs,
		CompletedJobs: orchestratorStats.CompletedJobs,
		FailedJobs:    orchestratorStats.FailedJobs,
	}
}
