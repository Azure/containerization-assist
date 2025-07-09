package pipeline

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/config"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
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
)

// BackgroundWorkerManager manages multiple background workers with lifecycle support
type BackgroundWorkerManager struct {
	workers         map[string]BackgroundWorker
	workerStates    map[string]WorkerStatus
	lifecycle       *common.Lifecycle
	config          *config.WorkerConfig
	healthTicker    *time.Ticker
	mu              sync.RWMutex
	startTime       time.Time
	healthMetrics   map[string]WorkerHealth
	shutdownTimeout time.Duration
}

// NewBackgroundWorkerManager creates a new background worker manager
func NewBackgroundWorkerManager(cfg *config.WorkerConfig) *BackgroundWorkerManager {
	return &BackgroundWorkerManager{
		workers:         make(map[string]BackgroundWorker),
		workerStates:    make(map[string]WorkerStatus),
		lifecycle:       common.NewLifecycle(),
		config:          cfg,
		startTime:       time.Now(),
		healthMetrics:   make(map[string]WorkerHealth),
		shutdownTimeout: cfg.ShutdownTimeout,
	}
}

// RegisterWorker registers a new background worker
func (m *BackgroundWorkerManager) RegisterWorker(worker BackgroundWorker) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name := worker.Name()
	if _, exists := m.workers[name]; exists {
		return errors.NewError().Messagef("worker %s already registered", name).WithLocation().Build()
	}

	m.workers[name] = worker
	m.workerStates[name] = WorkerStatusStopped
	return nil
}

// UnregisterWorker removes a worker from management
func (m *BackgroundWorkerManager) UnregisterWorker(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	worker, exists := m.workers[name]
	if !exists {
		return errors.NewError().Messagef("worker %s not found", name).WithLocation().Build()
	}

	if m.workerStates[name] == WorkerStatusRunning {
		if err := worker.Stop(); err != nil {
			return errors.NewError().Message("failed to stop worker " + name).Cause(err).WithLocation().Build()
		}
	}

	delete(m.workers, name)
	delete(m.workerStates, name)
	delete(m.healthMetrics, name)
	return nil
}

// StartAll starts all registered workers
func (m *BackgroundWorkerManager) StartAll() error {
	m.mu.Lock()
	workers := make(map[string]BackgroundWorker)
	for name, worker := range m.workers {
		workers[name] = worker
		m.workerStates[name] = WorkerStatusStarting
	}
	m.mu.Unlock()

	m.healthTicker = time.NewTicker(m.config.HealthCheckPeriod)
	if err := m.lifecycle.Go(m.healthMonitor); err != nil {
		return errors.NewError().Message("failed to start health monitor").Cause(err).WithLocation().Build()
	}

	for name, worker := range workers {
		workerName := name
		workerInstance := worker

		if err := m.lifecycle.Go(func(ctx context.Context) {
			m.startWorker(ctx, workerName, workerInstance)
		}); err != nil {
			return errors.NewError().Message("failed to start worker " + name).Cause(err).WithLocation().Build()
		}
	}

	return nil
}

// StartWorker starts a specific worker by name
func (m *BackgroundWorkerManager) StartWorker(name string) error {
	m.mu.Lock()
	worker, exists := m.workers[name]
	if !exists {
		m.mu.Unlock()
		return errors.NewError().Messagef("worker %s not found", name).WithLocation().Build()
	}

	if m.workerStates[name] == WorkerStatusRunning {
		m.mu.Unlock()
		return errors.NewError().Messagef("worker %s is already running", name).WithLocation().Build()
	}

	m.workerStates[name] = WorkerStatusStarting
	m.mu.Unlock()

	return m.lifecycle.Go(func(ctx context.Context) {
		m.startWorker(ctx, name, worker)
	})
}

// StopWorker stops a specific worker by name
func (m *BackgroundWorkerManager) StopWorker(name string) error {
	m.mu.Lock()
	worker, exists := m.workers[name]
	if !exists {
		m.mu.Unlock()
		return errors.NewError().Messagef("worker %s not found", name).WithLocation().Build()
	}

	if m.workerStates[name] != WorkerStatusRunning {
		m.mu.Unlock()
		return errors.NewError().Messagef("worker %s is not running", name).WithLocation().Build()
	}

	m.workerStates[name] = WorkerStatusStopping
	m.mu.Unlock()

	return worker.Stop()
}

// StopAll stops all workers gracefully
func (m *BackgroundWorkerManager) StopAll() error {
	if m.healthTicker != nil {
		m.healthTicker.Stop()
	}

	m.mu.RLock()
	workers := make([]BackgroundWorker, 0, len(m.workers))
	names := make([]string, 0, len(m.workers))
	for name, worker := range m.workers {
		if m.workerStates[name] == WorkerStatusRunning {
			workers = append(workers, worker)
			names = append(names, name)
		}
	}
	m.mu.RUnlock()

	var wg sync.WaitGroup
	errChan := make(chan error, len(workers))

	for i, worker := range workers {
		wg.Add(1)
		go func(w BackgroundWorker, name string) {
			defer wg.Done()

			m.mu.Lock()
			m.workerStates[name] = WorkerStatusStopping
			m.mu.Unlock()

			if err := w.Stop(); err != nil {
				m.mu.Lock()
				m.workerStates[name] = WorkerStatusFailed
				m.mu.Unlock()
				errChan <- errors.NewError().Message("failed to stop worker " + name).Cause(err).WithLocation().Build()
			} else {
				m.mu.Lock()
				m.workerStates[name] = WorkerStatusStopped
				m.mu.Unlock()
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
	case <-time.After(m.shutdownTimeout):
		return errors.NewError().Messagef("worker shutdown timeout after %v", m.shutdownTimeout).WithLocation().Build()
	}

	close(errChan)
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors stopping workers: %v", errors)
	}

	return m.lifecycle.Shutdown(m.shutdownTimeout)
}

// GetWorkerHealth returns health status for a specific worker
func (m *BackgroundWorkerManager) GetWorkerHealth(name string) (WorkerHealth, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// First check if worker exists
	worker, workerExists := m.workers[name]
	if !workerExists {
		return WorkerHealth{}, errors.NewError().Messagef("worker %s not found", name).WithLocation().Build()
	}

	// Check if health metrics are available
	if health, exists := m.healthMetrics[name]; exists {
		return health, nil
	}

	// If health metrics haven't been populated yet, get current health from worker
	health := worker.Health()
	health.LastCheck = time.Now()
	health.Uptime = time.Since(m.startTime)

	return health, nil
}

// GetAllWorkerHealth returns health status for all workers
func (m *BackgroundWorkerManager) GetAllWorkerHealth() map[string]WorkerHealth {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]WorkerHealth)

	// First add any health metrics we have
	for name, health := range m.healthMetrics {
		result[name] = health
	}

	// For workers without health metrics yet, get their current health
	for name, worker := range m.workers {
		if _, exists := result[name]; !exists {
			health := worker.Health()
			health.LastCheck = time.Now()
			health.Uptime = time.Since(m.startTime)
			result[name] = health
		}
	}

	return result
}

// GetWorkerStatus returns the current status of a worker
func (m *BackgroundWorkerManager) GetWorkerStatus(name string) (WorkerStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status, exists := m.workerStates[name]
	if !exists {
		return "", errors.NewError().Messagef("worker %s not found", name).WithLocation().Build()
	}

	return status, nil
}

// GetAllWorkerStatuses returns status for all workers
func (m *BackgroundWorkerManager) GetAllWorkerStatuses() map[string]WorkerStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]WorkerStatus)
	for name, status := range m.workerStates {
		result[name] = status
	}
	return result
}

// IsHealthy returns true if all workers are healthy
func (m *BackgroundWorkerManager) IsHealthy() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, health := range m.healthMetrics {
		if health.Status != "healthy" {
			return false
		}
	}
	return true
}

// GetManagerStats returns statistics about the worker manager
func (m *BackgroundWorkerManager) GetManagerStats() ManagerStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := ManagerStats{
		TotalWorkers:   len(m.workers),
		RunningWorkers: 0,
		FailedWorkers:  0,
		Uptime:         time.Since(m.startTime),
	}

	for _, status := range m.workerStates {
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
func (m *BackgroundWorkerManager) startWorker(ctx context.Context, name string, worker BackgroundWorker) {
	defer func() {
		if r := recover(); r != nil {
			m.mu.Lock()
			m.workerStates[name] = WorkerStatusFailed
			m.healthMetrics[name] = WorkerHealth{
				Status:    "failed",
				LastCheck: time.Now(),
				Error:     fmt.Errorf("worker panicked: %v", r),
			}
			m.mu.Unlock()
		}
	}()

	if err := worker.Start(ctx); err != nil {
		m.mu.Lock()
		m.workerStates[name] = WorkerStatusFailed
		m.healthMetrics[name] = WorkerHealth{
			Status:    "failed",
			LastCheck: time.Now(),
			Error:     err,
		}
		m.mu.Unlock()
		return
	}

	m.mu.Lock()
	m.workerStates[name] = WorkerStatusRunning
	m.mu.Unlock()

	<-ctx.Done()

	m.mu.Lock()
	m.workerStates[name] = WorkerStatusStopping
	m.mu.Unlock()

	if err := worker.Stop(); err != nil {
		m.mu.Lock()
		m.workerStates[name] = WorkerStatusFailed
		m.healthMetrics[name] = WorkerHealth{
			Status:    "failed",
			LastCheck: time.Now(),
			Error:     fmt.Errorf("failed to stop: %w", err),
		}
		m.mu.Unlock()
	} else {
		m.mu.Lock()
		m.workerStates[name] = WorkerStatusStopped
		m.mu.Unlock()
	}
}

// healthMonitor periodically checks the health of all workers
func (m *BackgroundWorkerManager) healthMonitor(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.healthTicker.C:
			m.checkAllWorkerHealth()
		}
	}
}

// checkAllWorkerHealth checks the health of all registered workers
func (m *BackgroundWorkerManager) checkAllWorkerHealth() {
	m.mu.Lock()
	workers := make(map[string]BackgroundWorker)
	for name, worker := range m.workers {
		workers[name] = worker
	}
	m.mu.Unlock()

	for name, worker := range workers {
		health := worker.Health()
		health.LastCheck = time.Now()
		health.Uptime = time.Since(m.startTime)

		m.mu.Lock()
		m.healthMetrics[name] = health

		if health.Error != nil && m.workerStates[name] == WorkerStatusRunning {
			m.workerStates[name] = WorkerStatusFailed
		}
		m.mu.Unlock()
	}
}

// RestartWorker stops and starts a worker
func (m *BackgroundWorkerManager) RestartWorker(name string) error {
	if err := m.StopWorker(name); err != nil {
		return errors.NewError().Message("failed to stop worker for restart").Cause(err).WithLocation().Build()
	}

	time.Sleep(100 * time.Millisecond)

	if err := m.StartWorker(name); err != nil {
		return errors.NewError().Message("failed to start worker after restart").Cause(err).WithLocation().Build()
	}

	return nil
}

func (m *BackgroundWorkerManager) RestartAllWorkers() error {
	if err := m.StopAll(); err != nil {
		return errors.NewError().Message("failed to stop all workers for restart").Cause(err).WithLocation().Build()
	}

	time.Sleep(500 * time.Millisecond)

	if err := m.StartAll(); err != nil {
		return errors.NewError().Message("failed to start all workers after restart").Cause(err).WithLocation().Build()
	}

	return nil
}

func (m *BackgroundWorkerManager) GetWorkerNames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.workers))
	for name := range m.workers {
		names = append(names, name)
	}
	return names
}

// SimpleBackgroundWorker is a basic implementation of BackgroundWorker
type SimpleBackgroundWorker struct {
	name        string
	taskFunc    func(ctx context.Context) error
	interval    time.Duration
	lifecycle   *common.Lifecycle
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
		lifecycle: common.NewLifecycle(),
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
