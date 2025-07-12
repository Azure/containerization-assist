// Package lifecycle handles server lifecycle management
package lifecycle

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// State represents the server lifecycle state
type State int

const (
	StateUninitialized State = iota
	StateInitialized
	StateStarting
	StateRunning
	StateStopping
	StateStopped
	StateError
)

// String returns the string representation of the state
func (s State) String() string {
	switch s {
	case StateUninitialized:
		return "uninitialized"
	case StateInitialized:
		return "initialized"
	case StateStarting:
		return "starting"
	case StateRunning:
		return "running"
	case StateStopping:
		return "stopping"
	case StateStopped:
		return "stopped"
	case StateError:
		return "error"
	default:
		return "unknown"
	}
}

// Status represents the server status
type Status struct {
	State     State
	StartTime time.Time
	Uptime    time.Duration
	Error     error
}

// Manager handles server lifecycle management
type Manager interface {
	Initialize() error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	GetStatus() Status
	OnStart(fn func(context.Context) error)
	OnStop(fn func(context.Context) error)
}

// managerImpl implements the lifecycle manager
type managerImpl struct {
	mu        sync.RWMutex
	state     State
	startTime time.Time
	lastError error
	logger    *slog.Logger

	// Lifecycle hooks
	onStartFuncs []func(context.Context) error
	onStopFuncs  []func(context.Context) error
}

// NewManager creates a new lifecycle manager
func NewManager(logger *slog.Logger) Manager {
	return &managerImpl{
		state:  StateUninitialized,
		logger: logger.With("component", "lifecycle_manager"),
	}
}

// Initialize initializes the server
func (m *managerImpl) Initialize() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state != StateUninitialized {
		return fmt.Errorf("cannot initialize from state %s", m.state)
	}

	m.logger.Info("Initializing server lifecycle")
	m.state = StateInitialized
	return nil
}

// Start starts the server lifecycle
func (m *managerImpl) Start(ctx context.Context) error {
	m.mu.Lock()

	// Check if we can start
	if m.state != StateInitialized && m.state != StateStopped {
		m.mu.Unlock()
		return fmt.Errorf("cannot start from state %s", m.state)
	}

	m.state = StateStarting
	m.startTime = time.Now()
	m.lastError = nil
	m.mu.Unlock()

	m.logger.Info("Starting server lifecycle")

	// Run start hooks
	for i, fn := range m.onStartFuncs {
		m.logger.Debug("Running start hook", "index", i)
		if err := fn(ctx); err != nil {
			m.mu.Lock()
			m.state = StateError
			m.lastError = err
			m.mu.Unlock()
			return fmt.Errorf("start hook %d failed: %w", i, err)
		}
	}

	// Update state to running
	m.mu.Lock()
	m.state = StateRunning
	m.mu.Unlock()

	m.logger.Info("Server lifecycle started successfully")
	return nil
}

// Stop stops the server lifecycle
func (m *managerImpl) Stop(ctx context.Context) error {
	m.mu.Lock()

	// Check if we can stop
	if m.state != StateRunning && m.state != StateError {
		m.mu.Unlock()
		return fmt.Errorf("cannot stop from state %s", m.state)
	}

	m.state = StateStopping
	m.mu.Unlock()

	m.logger.Info("Stopping server lifecycle")

	// Run stop hooks in reverse order
	var stopErrors []error
	for i := len(m.onStopFuncs) - 1; i >= 0; i-- {
		m.logger.Debug("Running stop hook", "index", i)
		if err := m.onStopFuncs[i](ctx); err != nil {
			m.logger.Error("Stop hook failed", "index", i, "error", err)
			stopErrors = append(stopErrors, err)
		}
	}

	// Update state
	m.mu.Lock()
	m.state = StateStopped
	if len(stopErrors) > 0 {
		m.lastError = fmt.Errorf("stop errors: %v", stopErrors)
	}
	m.mu.Unlock()

	if len(stopErrors) > 0 {
		return fmt.Errorf("encountered %d errors during stop", len(stopErrors))
	}

	m.logger.Info("Server lifecycle stopped successfully")
	return nil
}

// GetStatus returns the current server status
func (m *managerImpl) GetStatus() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := Status{
		State:     m.state,
		StartTime: m.startTime,
		Error:     m.lastError,
	}

	if m.state == StateRunning && !m.startTime.IsZero() {
		status.Uptime = time.Since(m.startTime)
	}

	return status
}

// OnStart registers a function to be called during server start
func (m *managerImpl) OnStart(fn func(context.Context) error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.onStartFuncs = append(m.onStartFuncs, fn)
}

// OnStop registers a function to be called during server stop
func (m *managerImpl) OnStop(fn func(context.Context) error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.onStopFuncs = append(m.onStopFuncs, fn)
}

// Coordinator coordinates multiple lifecycle managers
type Coordinator struct {
	managers []Manager
	logger   *slog.Logger
}

// NewCoordinator creates a new lifecycle coordinator
func NewCoordinator(logger *slog.Logger) *Coordinator {
	return &Coordinator{
		logger: logger.With("component", "lifecycle_coordinator"),
	}
}

// Register registers a lifecycle manager with the coordinator
func (c *Coordinator) Register(manager Manager) {
	c.managers = append(c.managers, manager)
}

// StartAll starts all registered managers
func (c *Coordinator) StartAll(ctx context.Context) error {
	c.logger.Info("Starting all lifecycle managers", "count", len(c.managers))

	for i, manager := range c.managers {
		if err := manager.Start(ctx); err != nil {
			// Stop already started managers
			c.logger.Error("Failed to start manager, stopping already started ones", "index", i, "error", err)
			for j := i - 1; j >= 0; j-- {
				_ = c.managers[j].Stop(ctx)
			}
			return fmt.Errorf("failed to start manager %d: %w", i, err)
		}
	}

	return nil
}

// StopAll stops all registered managers in reverse order
func (c *Coordinator) StopAll(ctx context.Context) error {
	c.logger.Info("Stopping all lifecycle managers", "count", len(c.managers))

	var errors []error
	for i := len(c.managers) - 1; i >= 0; i-- {
		if err := c.managers[i].Stop(ctx); err != nil {
			c.logger.Error("Failed to stop manager", "index", i, "error", err)
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("encountered %d errors during stop", len(errors))
	}

	return nil
}
