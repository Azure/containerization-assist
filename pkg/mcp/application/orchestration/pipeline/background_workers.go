package pipeline

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/config"
	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// BackgroundWorker represents a managed background worker
type BackgroundWorker interface {
	Start(ctx context.Context) error
	Stop() error
	Health() WorkerHealth
	Name() string
}

// WorkerHealth represents the health status of a worker
type WorkerHealth struct {
	Status      string             `json:"status"`
	LastCheck   time.Time          `json:"last_check"`
	Error       error              `json:"error,omitempty"`
	Metrics     map[string]float64 `json:"metrics,omitempty"`
	Uptime      time.Duration      `json:"uptime"`
	TasksTotal  int64              `json:"tasks_total"`
	TasksFailed int64              `json:"tasks_failed"`
}

// WorkerStatus represents the possible worker states
type WorkerStatus string

const (
	WorkerStatusStarting WorkerStatus = "starting"
	WorkerStatusRunning  WorkerStatus = "running"
	WorkerStatusStopping WorkerStatus = "stopping"
	WorkerStatusStopped  WorkerStatus = "stopped"
	WorkerStatusFailed   WorkerStatus = "failed"
	WorkerStatusUnknown  WorkerStatus = "unknown"
)

// BackgroundWorkerService defines the interface for background worker management
type BackgroundWorkerService interface {
	// Worker lifecycle operations
	RegisterWorker(worker BackgroundWorker) error
	UnregisterWorker(name string) error
	StartAll() error
	StartWorker(name string) error
	StopWorker(name string) error
	StopAll() error
	RestartWorker(name string) error
	RestartAllWorkers() error

	// Worker status and health
	GetWorkerHealth(name string) (WorkerHealth, error)
	GetAllWorkerHealth() map[string]WorkerHealth
	GetWorkerStatus(name string) (WorkerStatus, error)
	GetAllWorkerStatuses() map[string]WorkerStatus
	IsHealthy() bool
	GetWorkerNames() []string
	GetManagerStats() ManagerStats
}

// BackgroundWorkerServiceImpl implements BackgroundWorkerService
type BackgroundWorkerServiceImpl struct {
	workers         map[string]BackgroundWorker
	workerStates    map[string]WorkerStatus
	lifecycle       *shared.Lifecycle
	config          *config.WorkerConfig
	healthTicker    *time.Ticker
	mu              sync.RWMutex
	startTime       time.Time
	healthMetrics   map[string]WorkerHealth
	shutdownTimeout time.Duration
}

// Type alias for backward compatibility
type BackgroundWorkerManager = BackgroundWorkerServiceImpl

// NewBackgroundWorkerService creates a new background worker service
func NewBackgroundWorkerService(cfg *config.WorkerConfig) BackgroundWorkerService {
	return &BackgroundWorkerServiceImpl{
		workers:         make(map[string]BackgroundWorker),
		workerStates:    make(map[string]WorkerStatus),
		lifecycle:       shared.NewLifecycle(),
		config:          cfg,
		startTime:       time.Now(),
		healthMetrics:   make(map[string]WorkerHealth),
		shutdownTimeout: cfg.ShutdownTimeout,
	}
}

// NewBackgroundWorkerManager creates a new background worker manager (backward compatibility)
func NewBackgroundWorkerManager(cfg *config.WorkerConfig) *BackgroundWorkerManager {
	return &BackgroundWorkerManager{
		workers:         make(map[string]BackgroundWorker),
		workerStates:    make(map[string]WorkerStatus),
		lifecycle:       shared.NewLifecycle(),
		config:          cfg,
		startTime:       time.Now(),
		healthMetrics:   make(map[string]WorkerHealth),
		shutdownTimeout: cfg.ShutdownTimeout,
	}
}

// RegisterWorker registers a new background worker
func (s *BackgroundWorkerServiceImpl) RegisterWorker(worker BackgroundWorker) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	name := worker.Name()
	if _, exists := s.workers[name]; exists {
		return errors.NewError().Messagef("worker %s already registered", name).WithLocation().Build()
	}

	s.workers[name] = worker
	s.workerStates[name] = WorkerStatusStopped
	return nil
}

// UnregisterWorker removes a worker from management
func (s *BackgroundWorkerServiceImpl) UnregisterWorker(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	worker, exists := s.workers[name]
	if !exists {
		return errors.NewError().Messagef("worker %s not found", name).WithLocation().Build()
	}

	if s.workerStates[name] == WorkerStatusRunning {
		if err := worker.Stop(); err != nil {
			return errors.NewError().Message("failed to stop worker " + name).Cause(err).WithLocation().Build()
		}
	}

	delete(s.workers, name)
	delete(s.workerStates, name)
	delete(s.healthMetrics, name)
	return nil
}

// StartAll starts all registered workers
func (s *BackgroundWorkerServiceImpl) StartAll() error {
	s.mu.Lock()
	workers := make(map[string]BackgroundWorker)
	for name, worker := range s.workers {
		workers[name] = worker
		s.workerStates[name] = WorkerStatusStarting
	}
	s.mu.Unlock()

	s.healthTicker = time.NewTicker(s.config.HealthCheckPeriod)
	if err := s.lifecycle.Go(s.healthMonitor); err != nil {
		return errors.NewError().Message("failed to start health monitor").Cause(err).WithLocation().Build()
	}

	for name, worker := range workers {
		workerName := name
		workerInstance := worker

		if err := s.lifecycle.Go(func(ctx context.Context) {
			s.startWorker(ctx, workerName, workerInstance)
		}); err != nil {
			return errors.NewError().Message("failed to start worker " + name).Cause(err).WithLocation().Build()
		}
	}

	return nil
}

// StartWorker starts a specific worker by name
func (s *BackgroundWorkerServiceImpl) StartWorker(name string) error {
	s.mu.Lock()
	worker, exists := s.workers[name]
	if !exists {
		s.mu.Unlock()
		return errors.NewError().Messagef("worker %s not found", name).WithLocation().Build()
	}

	if s.workerStates[name] == WorkerStatusRunning {
		s.mu.Unlock()
		return errors.NewError().Messagef("worker %s is already running", name).WithLocation().Build()
	}

	s.workerStates[name] = WorkerStatusStarting
	s.mu.Unlock()

	return s.lifecycle.Go(func(ctx context.Context) {
		s.startWorker(ctx, name, worker)
	})
}

// StopWorker stops a specific worker by name
func (s *BackgroundWorkerServiceImpl) StopWorker(name string) error {
	s.mu.Lock()
	worker, exists := s.workers[name]
	if !exists {
		s.mu.Unlock()
		return errors.NewError().Messagef("worker %s not found", name).WithLocation().Build()
	}

	if s.workerStates[name] != WorkerStatusRunning {
		s.mu.Unlock()
		return errors.NewError().Messagef("worker %s is not running", name).WithLocation().Build()
	}

	s.workerStates[name] = WorkerStatusStopping
	s.mu.Unlock()

	return worker.Stop()
}

// StopAll stops all workers gracefully
func (s *BackgroundWorkerServiceImpl) StopAll() error {
	if s.healthTicker != nil {
		s.healthTicker.Stop()
	}

	s.mu.RLock()
	workers := make([]BackgroundWorker, 0, len(s.workers))
	names := make([]string, 0, len(s.workers))
	for name, worker := range s.workers {
		if s.workerStates[name] == WorkerStatusRunning {
			workers = append(workers, worker)
			names = append(names, name)
		}
	}
	s.mu.RUnlock()

	var wg sync.WaitGroup
	errChan := make(chan error, len(workers))

	for i, worker := range workers {
		wg.Add(1)
		go func(w BackgroundWorker, name string) {
			defer wg.Done()

			s.mu.Lock()
			s.workerStates[name] = WorkerStatusStopping
			s.mu.Unlock()

			if err := w.Stop(); err != nil {
				s.mu.Lock()
				s.workerStates[name] = WorkerStatusFailed
				s.mu.Unlock()
				errChan <- errors.NewError().Message("failed to stop worker " + name).Cause(err).WithLocation().Build()
			} else {
				s.mu.Lock()
				s.workerStates[name] = WorkerStatusStopped
				s.mu.Unlock()
			}
		}(worker, names[i])
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(s.shutdownTimeout):
		return errors.NewError().Messagef("worker shutdown timeout after %v", s.shutdownTimeout).WithLocation().Build()
	}

	close(errChan)
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors stopping workers: %v", errors)
	}

	return s.lifecycle.Shutdown(s.shutdownTimeout)
}

// GetWorkerHealth returns health status for a specific worker
func (s *BackgroundWorkerServiceImpl) GetWorkerHealth(name string) (WorkerHealth, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// First check if worker exists
	worker, workerExists := s.workers[name]
	if !workerExists {
		return WorkerHealth{}, errors.NewError().Messagef("worker %s not found", name).WithLocation().Build()
	}

	// Check if health metrics are available
	if health, exists := s.healthMetrics[name]; exists {
		return health, nil
	}

	// If health metrics haven't been populated yet, get current health from worker
	health := worker.Health()
	health.LastCheck = time.Now()
	health.Uptime = time.Since(s.startTime)

	return health, nil
}

// GetAllWorkerHealth returns health status for all workers
func (s *BackgroundWorkerServiceImpl) GetAllWorkerHealth() map[string]WorkerHealth {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]WorkerHealth)

	// First add any health metrics we have
	for name, health := range s.healthMetrics {
		result[name] = health
	}

	// For workers without health metrics yet, get their current health
	for name, worker := range s.workers {
		if _, exists := result[name]; !exists {
			health := worker.Health()
			health.LastCheck = time.Now()
			health.Uptime = time.Since(s.startTime)
			result[name] = health
		}
	}

	return result
}

// GetWorkerStatus returns the current status of a worker
func (s *BackgroundWorkerServiceImpl) GetWorkerStatus(name string) (WorkerStatus, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status, exists := s.workerStates[name]
	if !exists {
		return "", errors.NewError().Messagef("worker %s not found", name).WithLocation().Build()
	}

	return status, nil
}

// GetAllWorkerStatuses returns status for all workers
func (s *BackgroundWorkerServiceImpl) GetAllWorkerStatuses() map[string]WorkerStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]WorkerStatus)
	for name, status := range s.workerStates {
		result[name] = status
	}
	return result
}

// IsHealthy returns true if all workers are healthy
func (s *BackgroundWorkerServiceImpl) IsHealthy() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, health := range s.healthMetrics {
		if health.Status != "healthy" {
			return false
		}
	}
	return true
}

// GetManagerStats returns statistics about the worker manager
func (s *BackgroundWorkerServiceImpl) GetManagerStats() ManagerStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := ManagerStats{
		TotalWorkers:   len(s.workers),
		RunningWorkers: 0,
		FailedWorkers:  0,
		Uptime:         time.Since(s.startTime),
	}

	for _, status := range s.workerStates {
		switch status {
		case WorkerStatusRunning:
			stats.RunningWorkers++
		case WorkerStatusFailed:
			stats.FailedWorkers++
		}
	}

	return stats
}

// ManagerStats contains statistics about the worker manager
type ManagerStats struct {
	TotalWorkers   int           `json:"total_workers"`
	RunningWorkers int           `json:"running_workers"`
	FailedWorkers  int           `json:"failed_workers"`
	Uptime         time.Duration `json:"uptime"`
}

// startWorker starts an individual worker with error handling
func (s *BackgroundWorkerServiceImpl) startWorker(ctx context.Context, name string, worker BackgroundWorker) {
	defer func() {
		if r := recover(); r != nil {
			s.mu.Lock()
			s.workerStates[name] = WorkerStatusFailed
			s.healthMetrics[name] = WorkerHealth{
				Status:    "failed",
				LastCheck: time.Now(),
				Error:     fmt.Errorf("worker panicked: %v", r),
			}
			s.mu.Unlock()
		}
	}()

	if err := worker.Start(ctx); err != nil {
		s.mu.Lock()
		s.workerStates[name] = WorkerStatusFailed
		s.healthMetrics[name] = WorkerHealth{
			Status:    "failed",
			LastCheck: time.Now(),
			Error:     err,
		}
		s.mu.Unlock()
		return
	}

	s.mu.Lock()
	s.workerStates[name] = WorkerStatusRunning
	s.mu.Unlock()

	<-ctx.Done()

	s.mu.Lock()
	s.workerStates[name] = WorkerStatusStopping
	s.mu.Unlock()

	if err := worker.Stop(); err != nil {
		s.mu.Lock()
		s.workerStates[name] = WorkerStatusFailed
		s.healthMetrics[name] = WorkerHealth{
			Status:    "failed",
			LastCheck: time.Now(),
			Error:     fmt.Errorf("failed to stop: %w", err),
		}
		s.mu.Unlock()
	} else {
		s.mu.Lock()
		s.workerStates[name] = WorkerStatusStopped
		s.mu.Unlock()
	}
}

// healthMonitor periodically checks the health of all workers
func (s *BackgroundWorkerServiceImpl) healthMonitor(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.healthTicker.C:
			s.checkAllWorkerHealth()
		}
	}
}

// checkAllWorkerHealth checks the health of all registered workers
func (s *BackgroundWorkerServiceImpl) checkAllWorkerHealth() {
	s.mu.Lock()
	workers := make(map[string]BackgroundWorker)
	for name, worker := range s.workers {
		workers[name] = worker
	}
	s.mu.Unlock()

	for name, worker := range workers {
		health := worker.Health()
		health.LastCheck = time.Now()
		health.Uptime = time.Since(s.startTime)

		s.mu.Lock()
		s.healthMetrics[name] = health

		if health.Error != nil && s.workerStates[name] == WorkerStatusRunning {
			s.workerStates[name] = WorkerStatusFailed
		}
		s.mu.Unlock()
	}
}

// RestartWorker stops and starts a worker
func (s *BackgroundWorkerServiceImpl) RestartWorker(name string) error {
	if err := s.StopWorker(name); err != nil {
		return errors.NewError().Message("failed to stop worker for restart").Cause(err).WithLocation().Build()
	}

	time.Sleep(100 * time.Millisecond)

	if err := s.StartWorker(name); err != nil {
		return errors.NewError().Message("failed to start worker after restart").Cause(err).WithLocation().Build()
	}

	return nil
}

func (s *BackgroundWorkerServiceImpl) RestartAllWorkers() error {
	if err := s.StopAll(); err != nil {
		return errors.NewError().Message("failed to stop all workers for restart").Cause(err).WithLocation().Build()
	}

	time.Sleep(500 * time.Millisecond)

	if err := s.StartAll(); err != nil {
		return errors.NewError().Message("failed to start all workers after restart").Cause(err).WithLocation().Build()
	}

	return nil
}

func (s *BackgroundWorkerServiceImpl) GetWorkerNames() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]string, 0, len(s.workers))
	for name := range s.workers {
		names = append(names, name)
	}
	return names
}

// SimpleBackgroundWorker is a basic implementation of BackgroundWorker
type SimpleBackgroundWorker struct {
	name        string
	taskFunc    func(ctx context.Context) error
	interval    time.Duration
	lifecycle   *shared.Lifecycle
	lastError   error
	tasksTotal  int64
	tasksFailed int64
	startTime   time.Time
}

// NewSimpleBackgroundWorker creates a new simple background worker
func NewSimpleBackgroundWorker(name string, taskFunc func(ctx context.Context) error, interval time.Duration) *SimpleBackgroundWorker {
	return &SimpleBackgroundWorker{
		name:      name,
		taskFunc:  taskFunc,
		interval:  interval,
		lifecycle: shared.NewLifecycle(),
		startTime: time.Now(),
	}
}

// Name returns the worker's name
func (w *SimpleBackgroundWorker) Name() string {
	return w.name
}

// Start starts the worker
func (w *SimpleBackgroundWorker) Start(ctx context.Context) error {
	return w.lifecycle.Go(func(workerCtx context.Context) {
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()

		for {
			select {
			case <-workerCtx.Done():
				return
			case <-ticker.C:
				w.tasksTotal++
				if err := w.taskFunc(workerCtx); err != nil {
					w.lastError = err
					w.tasksFailed++
				} else {
					w.lastError = nil
				}
			}
		}
	})
}

// Stop stops the worker
func (w *SimpleBackgroundWorker) Stop() error {
	return w.lifecycle.Shutdown(10 * time.Second)
}

// Health returns the worker's health status
func (w *SimpleBackgroundWorker) Health() WorkerHealth {
	status := "healthy"
	if w.lastError != nil {
		status = "unhealthy"
	}

	return WorkerHealth{
		Status:      status,
		LastCheck:   time.Now(),
		Error:       w.lastError,
		Uptime:      time.Since(w.startTime),
		TasksTotal:  w.tasksTotal,
		TasksFailed: w.tasksFailed,
		Metrics: map[string]float64{
			"tasks_per_minute": float64(w.tasksTotal) / time.Since(w.startTime).Minutes(),
			"failure_rate":     float64(w.tasksFailed) / float64(w.tasksTotal),
		},
	}
}

// Backward compatibility methods for BackgroundWorkerManager
// All methods now delegate to the new WorkerManagementService

// Deprecated: Use WorkerManagementService instead
// These methods provide backward compatibility but create new service instances
// In production, you should use a single WorkerManagementService instance
