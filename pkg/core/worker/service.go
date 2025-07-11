package worker

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"fmt"
	"github.com/Azure/container-kit/pkg/mcp/errors"
)

// Service provides a unified interface to worker management operations
type Service interface {
	// Worker lifecycle
	RegisterWorker(worker Worker) error
	UnregisterWorker(name string) error
	StartWorker(name string) error
	StopWorker(name string) error
	RestartWorker(name string) error

	// Worker management
	StartAll() error
	StopAll() error
	RestartAll() error

	// Worker status and monitoring
	GetWorkerStatus(name string) (WorkerStatus, error)
	GetAllWorkerStatuses() map[string]WorkerStatus
	GetWorkerHealth(name string) (WorkerHealth, error)
	GetAllWorkerHealth() map[string]WorkerHealth
	GetWorkerNames() []string
	IsHealthy() bool

	// Worker information
	GetWorkerInfo(name string) (*WorkerInfo, error)
	ListWorkers() ([]WorkerSummary, error)
	GetManagerStats() ManagerStats
}

// ServiceImpl implements the Worker Service interface
type ServiceImpl struct {
	logger       *slog.Logger
	workers      map[string]*workerInstance
	config       *Config
	ctx          context.Context
	cancel       context.CancelFunc
	mutex        sync.RWMutex
	isRunning    bool
	shutdownChan chan struct{}
}

// NewWorkerService creates a new Worker service
func NewWorkerService(logger *slog.Logger, config *Config) Service {
	ctx, cancel := context.WithCancel(context.Background())

	if config == nil {
		config = DefaultConfig()
	}

	return &ServiceImpl{
		logger:       logger.With("component", "worker_service"),
		workers:      make(map[string]*workerInstance),
		config:       config,
		ctx:          ctx,
		cancel:       cancel,
		shutdownChan: make(chan struct{}),
	}
}

// Worker interface that all workers must implement
type Worker interface {
	Name() string
	Execute(ctx context.Context) error
	Interval() time.Duration
	IsEnabled() bool
	HealthCheck() error
}

// Status represents the current status of a worker
type Status string

const (
	StatusStopped  Status = "stopped"
	StatusStarting Status = "starting"
	StatusRunning  Status = "running"
	StatusStopping Status = "stopping"
	StatusFailed   Status = "failed"
	StatusUnknown  Status = "unknown"

	// Backward compatibility constants
	WorkerStatusStopped  = StatusStopped
	WorkerStatusStarting = StatusStarting
	WorkerStatusRunning  = StatusRunning
	WorkerStatusStopping = StatusStopping
	WorkerStatusFailed   = StatusFailed
	WorkerStatusUnknown  = StatusUnknown
)

// Type aliases for backward compatibility
//
//nolint:revive // These aliases are needed for backward compatibility
type WorkerStatus = Status

//nolint:revive // These aliases are needed for backward compatibility
type WorkerHealth = Health

//nolint:revive // These aliases are needed for backward compatibility
type WorkerInfo = Info

//nolint:revive // These aliases are needed for backward compatibility
type WorkerSummary = Summary

// Health represents the health status of a worker
type Health struct {
	Healthy      bool
	Status       Status
	LastCheck    time.Time
	LastError    error
	ErrorCount   int
	SuccessCount int
	Uptime       time.Duration
}

// Info contains detailed information about a worker
type Info struct {
	Name           string
	Status         Status
	Health         Health
	Interval       time.Duration
	Enabled        bool
	ExecutionCount int64
	LastExecution  time.Time
	NextExecution  time.Time
	CreatedAt      time.Time
	StartedAt      *time.Time
	StoppedAt      *time.Time
}

// Summary contains summary information about a worker
type Summary struct {
	Name           string
	Status         Status
	Healthy        bool
	Enabled        bool
	ExecutionCount int64
	LastExecution  time.Time
	Uptime         time.Duration
}

// ManagerStats contains statistics about the worker manager
type ManagerStats struct {
	TotalWorkers     int
	RunningWorkers   int
	HealthyWorkers   int
	FailedWorkers    int
	TotalExecutions  int64
	AvgExecutionTime time.Duration
	Uptime           time.Duration
	LastUpdate       time.Time
}

// Config contains worker service configuration
type Config struct {
	ShutdownTimeout   time.Duration
	HealthCheckPeriod time.Duration
	MaxRetries        int
	RetryDelay        time.Duration
	MaxConcurrent     int
}

// DefaultConfig returns default worker service configuration
func DefaultConfig() *Config {
	return &Config{
		ShutdownTimeout:   30 * time.Second,
		HealthCheckPeriod: 10 * time.Second,
		MaxRetries:        3,
		RetryDelay:        1 * time.Second,
		MaxConcurrent:     10,
	}
}

// workerInstance represents an internal worker instance
type workerInstance struct {
	worker         Worker
	status         WorkerStatus
	health         WorkerHealth
	ctx            context.Context
	cancel         context.CancelFunc
	doneChan       chan struct{}
	executionCount int64
	lastExecution  time.Time
	nextExecution  time.Time
	createdAt      time.Time
	startedAt      *time.Time
	stoppedAt      *time.Time
	mutex          sync.RWMutex
}

// RegisterWorker registers a new worker
func (s *ServiceImpl) RegisterWorker(worker Worker) error {
	s.logger.Info("Registering worker", "name", worker.Name())

	s.mutex.Lock()
	defer s.mutex.Unlock()

	name := worker.Name()
	if _, exists := s.workers[name]; exists {
		return errors.New(errors.CodeIoError, "worker", fmt.Sprintf("worker already registered: %s", name), nil)
	}

	ctx, cancel := context.WithCancel(s.ctx)

	instance := &workerInstance{
		worker:    worker,
		status:    WorkerStatusStopped,
		ctx:       ctx,
		cancel:    cancel,
		doneChan:  make(chan struct{}),
		createdAt: time.Now(),
		health: WorkerHealth{
			Healthy:   true,
			Status:    WorkerStatusStopped,
			LastCheck: time.Now(),
		},
	}

	s.workers[name] = instance

	s.logger.Info("Successfully registered worker", "name", name)
	return nil
}

// UnregisterWorker removes a worker
func (s *ServiceImpl) UnregisterWorker(name string) error {
	s.logger.Info("Unregistering worker", "name", name)

	s.mutex.Lock()
	defer s.mutex.Unlock()

	instance, exists := s.workers[name]
	if !exists {
		return errors.New(errors.CodeNotFound, "worker", fmt.Sprintf("worker not found: %s", name), nil)
	}

	// Stop the worker if it's running
	if instance.status == WorkerStatusRunning {
		if err := s.stopWorkerInstance(instance); err != nil {
			s.logger.Error("Failed to stop worker instance", "name", name, "error", err)
		}
	}

	instance.cancel()
	delete(s.workers, name)

	s.logger.Info("Successfully unregistered worker", "name", name)
	return nil
}

// StartWorker starts a specific worker
func (s *ServiceImpl) StartWorker(name string) error {
	s.logger.Info("Starting worker", "name", name)

	s.mutex.Lock()
	defer s.mutex.Unlock()

	instance, exists := s.workers[name]
	if !exists {
		return errors.New(errors.CodeNotFound, "worker", fmt.Sprintf("worker not found: %s", name), nil)
	}

	if instance.status == WorkerStatusRunning {
		return errors.New(errors.CodeIoError, "worker", fmt.Sprintf("worker already running: %s", name), nil)
	}

	return s.startWorkerInstance(instance)
}

// StopWorker stops a specific worker
func (s *ServiceImpl) StopWorker(name string) error {
	s.logger.Info("Stopping worker", "name", name)

	s.mutex.Lock()
	defer s.mutex.Unlock()

	instance, exists := s.workers[name]
	if !exists {
		return errors.New(errors.CodeNotFound, "worker", fmt.Sprintf("worker not found: %s", name), nil)
	}

	if instance.status != WorkerStatusRunning {
		return errors.New(errors.CodeInternalError, "worker", fmt.Sprintf("worker not running: %s", name), nil)
	}

	return s.stopWorkerInstance(instance)
}

// RestartWorker restarts a specific worker
func (s *ServiceImpl) RestartWorker(name string) error {
	s.logger.Info("Restarting worker", "name", name)

	// Stop first if running
	if err := s.StopWorker(name); err != nil {
		// Check if the error is because the worker is not running
		if !strings.Contains(err.Error(), "worker not running") {
			return err
		}
	}

	// Then start
	return s.StartWorker(name)
}

// StartAll starts all registered workers
func (s *ServiceImpl) StartAll() error {
	s.logger.Info("Starting all workers")

	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.isRunning = true

	var errs []error
	for name, instance := range s.workers {
		if instance.worker.IsEnabled() {
			if err := s.startWorkerInstance(instance); err != nil {
				s.logger.Error("Failed to start worker", "name", name, "error", err)
				errs = append(errs, err)
			}
		}
	}

	if len(errs) > 0 {
		return errors.New(errors.CodeOperationFailed, "worker", "failed to start some workers", nil)
	}

	s.logger.Info("Successfully started all workers")
	return nil
}

// StopAll stops all running workers
func (s *ServiceImpl) StopAll() error {
	s.logger.Info("Stopping all workers")

	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.isRunning = false

	var errs []error
	for name, instance := range s.workers {
		if instance.status == WorkerStatusRunning {
			if err := s.stopWorkerInstance(instance); err != nil {
				s.logger.Error("Failed to stop worker", "name", name, "error", err)
				errs = append(errs, err)
			}
		}
	}

	if len(errs) > 0 {
		return errors.New(errors.CodeOperationFailed, "worker", "failed to stop some workers", nil)
	}

	s.logger.Info("Successfully stopped all workers")
	return nil
}

// RestartAll restarts all workers
func (s *ServiceImpl) RestartAll() error {
	s.logger.Info("Restarting all workers")

	if err := s.StopAll(); err != nil {
		return err
	}

	return s.StartAll()
}

// GetWorkerStatus returns the status of a specific worker
func (s *ServiceImpl) GetWorkerStatus(name string) (WorkerStatus, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	instance, exists := s.workers[name]
	if !exists {
		return WorkerStatusUnknown, errors.New(errors.CodeNotFound, "worker", fmt.Sprintf("worker not found: %s", name), nil)
	}

	instance.mutex.RLock()
	status := instance.status
	instance.mutex.RUnlock()

	return status, nil
}

// GetAllWorkerStatuses returns the status of all workers
func (s *ServiceImpl) GetAllWorkerStatuses() map[string]WorkerStatus {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	statuses := make(map[string]WorkerStatus)
	for name, instance := range s.workers {
		instance.mutex.RLock()
		statuses[name] = instance.status
		instance.mutex.RUnlock()
	}

	return statuses
}

// GetWorkerHealth returns the health status of a specific worker
func (s *ServiceImpl) GetWorkerHealth(name string) (WorkerHealth, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	instance, exists := s.workers[name]
	if !exists {
		return WorkerHealth{}, errors.New(errors.CodeNotFound, "worker", fmt.Sprintf("worker not found: %s", name), nil)
	}

	instance.mutex.RLock()
	health := instance.health
	instance.mutex.RUnlock()

	return health, nil
}

// GetAllWorkerHealth returns the health status of all workers
func (s *ServiceImpl) GetAllWorkerHealth() map[string]WorkerHealth {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	health := make(map[string]WorkerHealth)
	for name, instance := range s.workers {
		instance.mutex.RLock()
		health[name] = instance.health
		instance.mutex.RUnlock()
	}

	return health
}

// GetWorkerNames returns a list of all registered worker names
func (s *ServiceImpl) GetWorkerNames() []string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	names := make([]string, 0, len(s.workers))
	for name := range s.workers {
		names = append(names, name)
	}

	return names
}

// IsHealthy returns true if all workers are healthy
func (s *ServiceImpl) IsHealthy() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	for _, instance := range s.workers {
		instance.mutex.RLock()
		healthy := instance.health.Healthy
		instance.mutex.RUnlock()

		if !healthy {
			return false
		}
	}

	return true
}

// GetWorkerInfo returns detailed information about a worker
func (s *ServiceImpl) GetWorkerInfo(name string) (*WorkerInfo, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	instance, exists := s.workers[name]
	if !exists {
		return nil, errors.New(errors.CodeNotFound, "worker", fmt.Sprintf("worker not found: %s", name), nil)
	}

	instance.mutex.RLock()
	defer instance.mutex.RUnlock()

	info := &WorkerInfo{
		Name:           name,
		Status:         instance.status,
		Health:         instance.health,
		Interval:       instance.worker.Interval(),
		Enabled:        instance.worker.IsEnabled(),
		ExecutionCount: instance.executionCount,
		LastExecution:  instance.lastExecution,
		NextExecution:  instance.nextExecution,
		CreatedAt:      instance.createdAt,
		StartedAt:      instance.startedAt,
		StoppedAt:      instance.stoppedAt,
	}

	return info, nil
}

// ListWorkers returns a summary of all workers
func (s *ServiceImpl) ListWorkers() ([]WorkerSummary, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	summaries := make([]WorkerSummary, 0, len(s.workers))

	for name, instance := range s.workers {
		instance.mutex.RLock()

		var uptime time.Duration
		if instance.startedAt != nil && instance.status == WorkerStatusRunning {
			uptime = time.Since(*instance.startedAt)
		}

		summary := WorkerSummary{
			Name:           name,
			Status:         instance.status,
			Healthy:        instance.health.Healthy,
			Enabled:        instance.worker.IsEnabled(),
			ExecutionCount: instance.executionCount,
			LastExecution:  instance.lastExecution,
			Uptime:         uptime,
		}

		instance.mutex.RUnlock()
		summaries = append(summaries, summary)
	}

	return summaries, nil
}

// GetManagerStats returns statistics about the worker manager
func (s *ServiceImpl) GetManagerStats() ManagerStats {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var totalWorkers, runningWorkers, healthyWorkers, failedWorkers int
	var totalExecutions int64
	var totalExecutionTime time.Duration
	var executionCount int

	for _, instance := range s.workers {
		instance.mutex.RLock()

		totalWorkers++
		totalExecutions += instance.executionCount

		switch instance.status {
		case WorkerStatusRunning:
			runningWorkers++
		case WorkerStatusFailed:
			failedWorkers++
		}

		if instance.health.Healthy {
			healthyWorkers++
		}

		// Simulate execution time tracking
		if instance.executionCount > 0 {
			totalExecutionTime += time.Duration(instance.executionCount) * 100 * time.Millisecond
			executionCount++
		}

		instance.mutex.RUnlock()
	}

	var avgExecutionTime time.Duration
	if executionCount > 0 {
		avgExecutionTime = totalExecutionTime / time.Duration(executionCount)
	}

	return ManagerStats{
		TotalWorkers:     totalWorkers,
		RunningWorkers:   runningWorkers,
		HealthyWorkers:   healthyWorkers,
		FailedWorkers:    failedWorkers,
		TotalExecutions:  totalExecutions,
		AvgExecutionTime: avgExecutionTime,
		Uptime:           time.Since(time.Now().Add(-1 * time.Hour)), // Simulate uptime
		LastUpdate:       time.Now(),
	}
}

// Internal helper methods

func (s *ServiceImpl) startWorkerInstance(instance *workerInstance) error {
	if !instance.worker.IsEnabled() {
		return errors.New(errors.CodeInternalError, "worker", fmt.Sprintf("worker is disabled: %s", instance.worker.Name()), nil)
	}

	instance.mutex.Lock()
	defer instance.mutex.Unlock()

	instance.status = WorkerStatusStarting
	now := time.Now()
	instance.startedAt = &now
	instance.stoppedAt = nil

	// Start the worker goroutine
	go s.runWorker(instance)

	instance.status = WorkerStatusRunning
	instance.health.Status = WorkerStatusRunning
	instance.health.LastCheck = time.Now()

	s.logger.Info("Successfully started worker", "name", instance.worker.Name())
	return nil
}

func (s *ServiceImpl) stopWorkerInstance(instance *workerInstance) error {
	instance.mutex.Lock()
	defer instance.mutex.Unlock()

	instance.status = WorkerStatusStopping
	instance.cancel()

	// Wait for worker to stop with timeout
	select {
	case <-instance.doneChan:
		// Worker stopped gracefully
	case <-time.After(s.config.ShutdownTimeout):
		// Timeout - force stop
		s.logger.Warn("Worker stop timeout", "name", instance.worker.Name())
	}

	instance.status = WorkerStatusStopped
	instance.health.Status = WorkerStatusStopped
	now := time.Now()
	instance.stoppedAt = &now

	s.logger.Info("Successfully stopped worker", "name", instance.worker.Name())
	return nil
}

func (s *ServiceImpl) runWorker(instance *workerInstance) {
	defer close(instance.doneChan)

	name := instance.worker.Name()
	interval := instance.worker.Interval()

	s.logger.Info("Worker started", "name", name, "interval", interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-instance.ctx.Done():
			s.logger.Info("Worker stopped", "name", name)
			return

		case <-ticker.C:
			s.executeWorker(instance)
		}
	}
}

func (s *ServiceImpl) executeWorker(instance *workerInstance) {
	name := instance.worker.Name()

	// Update execution tracking
	instance.mutex.Lock()
	instance.executionCount++
	instance.lastExecution = time.Now()
	instance.nextExecution = time.Now().Add(instance.worker.Interval())
	instance.mutex.Unlock()

	// Execute the worker
	start := time.Now()
	err := instance.worker.Execute(instance.ctx)
	duration := time.Since(start)

	// Update health status
	instance.mutex.Lock()
	instance.health.LastCheck = time.Now()
	if err != nil {
		instance.health.Healthy = false
		instance.health.LastError = err
		instance.health.ErrorCount++
		instance.status = WorkerStatusFailed
		s.logger.Error("Worker execution failed", "name", name, "error", err, "duration", duration.String())
	} else {
		instance.health.Healthy = true
		instance.health.LastError = nil
		instance.health.SuccessCount++
		if instance.status != WorkerStatusRunning {
			instance.status = WorkerStatusRunning
		}
		s.logger.Debug("Worker executed successfully", "name", name, "duration", duration.String())
	}
	instance.health.Status = instance.status
	instance.mutex.Unlock()
}
