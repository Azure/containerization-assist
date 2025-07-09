// Package security provides health monitoring capabilities for the security scanning framework
package security

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	mcperrors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/rs/zerolog"
)

// HealthStatus represents the overall health status
type HealthStatus string

// Health status constants
const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
)

// ComponentHealth represents the health of an individual component
type ComponentHealth struct {
	Name         string                 `json:"name"`
	Status       HealthStatus           `json:"status"`
	Message      string                 `json:"message,omitempty"`
	LastChecked  time.Time              `json:"last_checked"`
	ResponseTime time.Duration          `json:"response_time"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	Dependencies []string               `json:"dependencies,omitempty"`
	CheckCount   int64                  `json:"check_count"`
	FailureCount int64                  `json:"failure_count"`
	LastFailure  *time.Time             `json:"last_failure,omitempty"`
	LastSuccess  *time.Time             `json:"last_success,omitempty"`
}

// OverallHealth represents the overall system health
type OverallHealth struct {
	Status     HealthStatus               `json:"status"`
	Timestamp  time.Time                  `json:"timestamp"`
	Version    string                     `json:"version,omitempty"`
	Uptime     time.Duration              `json:"uptime"`
	Components map[string]ComponentHealth `json:"components"`
	Summary    HealthSummary              `json:"summary"`
	Details    map[string]interface{}     `json:"details,omitempty"`
}

// HealthSummary provides aggregated health information
type HealthSummary struct {
	TotalComponents     int `json:"total_components"`
	HealthyComponents   int `json:"healthy_components"`
	DegradedComponents  int `json:"degraded_components"`
	UnhealthyComponents int `json:"unhealthy_components"`
}

// ReadinessCheck represents a readiness check result
type ReadinessCheck struct {
	Component    string        `json:"component"`
	Ready        bool          `json:"ready"`
	Message      string        `json:"message,omitempty"`
	CheckTime    time.Time     `json:"check_time"`
	ResponseTime time.Duration `json:"response_time"`
}

// ReadinessStatus represents overall readiness status
type ReadinessStatus struct {
	Ready     bool             `json:"ready"`
	Timestamp time.Time        `json:"timestamp"`
	Checks    []ReadinessCheck `json:"checks"`
	Message   string           `json:"message,omitempty"`
}

// HealthChecker interface defines health check operations
type HealthChecker interface {
	CheckHealth(ctx context.Context) ComponentHealth
	GetName() string
	GetDependencies() []string
}

// HealthMonitor manages health checks for the security scanning framework
type HealthMonitor struct {
	logger        zerolog.Logger
	checkers      map[string]HealthChecker
	results       map[string]ComponentHealth
	mutex         sync.RWMutex
	startTime     time.Time
	version       string
	checkInterval time.Duration
	timeout       time.Duration
	stopChan      chan struct{}
	running       bool
}

// NewHealthMonitor creates a new health monitor
func NewHealthMonitor(logger zerolog.Logger) *HealthMonitor {
	return &HealthMonitor{
		logger:        logger.With().Str("component", "health_monitor").Logger(),
		checkers:      make(map[string]HealthChecker),
		results:       make(map[string]ComponentHealth),
		startTime:     time.Now(),
		checkInterval: 30 * time.Second,
		timeout:       10 * time.Second,
		stopChan:      make(chan struct{}),
	}
}

// SetVersion sets the application version for health reporting
func (hm *HealthMonitor) SetVersion(version string) {
	hm.mutex.Lock()
	defer hm.mutex.Unlock()
	hm.version = version
}

// SetCheckInterval sets the health check interval
func (hm *HealthMonitor) SetCheckInterval(interval time.Duration) {
	hm.mutex.Lock()
	defer hm.mutex.Unlock()
	hm.checkInterval = interval
}

// SetTimeout sets the health check timeout
func (hm *HealthMonitor) SetTimeout(timeout time.Duration) {
	hm.mutex.Lock()
	defer hm.mutex.Unlock()
	hm.timeout = timeout
}

// RegisterChecker registers a health checker
func (hm *HealthMonitor) RegisterChecker(checker HealthChecker) {
	hm.mutex.Lock()
	defer hm.mutex.Unlock()

	name := checker.GetName()
	hm.checkers[name] = checker

	// Initialize result entry
	hm.results[name] = ComponentHealth{
		Name:         name,
		Status:       HealthStatusUnhealthy,
		Message:      "Not yet checked",
		LastChecked:  time.Time{},
		Dependencies: checker.GetDependencies(),
	}

	hm.logger.Info().Str("checker", name).Msg("Health checker registered")
}

// UnregisterChecker unregisters a health checker
func (hm *HealthMonitor) UnregisterChecker(name string) {
	hm.mutex.Lock()
	defer hm.mutex.Unlock()

	delete(hm.checkers, name)
	delete(hm.results, name)

	hm.logger.Info().Str("checker", name).Msg("Health checker unregistered")
}

// Start begins periodic health checking
func (hm *HealthMonitor) Start(ctx context.Context) error {
	hm.mutex.Lock()
	if hm.running {
		hm.mutex.Unlock()
		return mcperrors.NewError().Messagef("health monitor is already running").WithLocation().Build()
	}
	hm.running = true
	hm.mutex.Unlock()

	hm.logger.Info().
		Dur("interval", hm.checkInterval).
		Dur("timeout", hm.timeout).
		Msg("Starting health monitor")

	// Initial health check
	hm.checkAllHealth(ctx)

	// Start periodic checking
	go hm.runPeriodicChecks(ctx)

	return nil
}

// Stop stops the health monitor
func (hm *HealthMonitor) Stop() error {
	hm.mutex.Lock()
	defer hm.mutex.Unlock()

	if !hm.running {
		return mcperrors.NewError().Messagef("health monitor is not running").WithLocation().Build()
	}

	hm.logger.Info().Msg("Stopping health monitor")

	close(hm.stopChan)
	hm.running = false

	return nil
}

// runPeriodicChecks runs health checks on a schedule
func (hm *HealthMonitor) runPeriodicChecks(ctx context.Context) {
	ticker := time.NewTicker(hm.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			hm.logger.Info().Msg("Health monitor stopped due to context cancellation")
			return
		case <-hm.stopChan:
			hm.logger.Info().Msg("Health monitor stopped")
			return
		case <-ticker.C:
			hm.checkAllHealth(ctx)
		}
	}
}

// checkAllHealth runs all registered health checks
func (hm *HealthMonitor) checkAllHealth(ctx context.Context) {
	hm.mutex.RLock()
	checkers := make(map[string]HealthChecker)
	for name, checker := range hm.checkers {
		checkers[name] = checker
	}
	hm.mutex.RUnlock()

	var wg sync.WaitGroup
	for name, checker := range checkers {
		wg.Add(1)
		go func(name string, checker HealthChecker) {
			defer wg.Done()
			hm.runHealthCheck(ctx, name, checker)
		}(name, checker)
	}
	wg.Wait()
}

// runHealthCheck runs a single health check
func (hm *HealthMonitor) runHealthCheck(ctx context.Context, name string, checker HealthChecker) {
	checkCtx, cancel := context.WithTimeout(ctx, hm.timeout)
	defer cancel()

	startTime := time.Now()
	result := checker.CheckHealth(checkCtx)
	responseTime := time.Since(startTime)

	// Update result timing
	result.LastChecked = startTime
	result.ResponseTime = responseTime

	hm.mutex.Lock()
	if existing, ok := hm.results[name]; ok {
		result.CheckCount = existing.CheckCount + 1
		result.FailureCount = existing.FailureCount
		result.LastSuccess = existing.LastSuccess
		result.LastFailure = existing.LastFailure

		if result.Status == HealthStatusHealthy {
			now := time.Now()
			result.LastSuccess = &now
		} else {
			result.FailureCount++
			now := time.Now()
			result.LastFailure = &now
		}
	} else {
		result.CheckCount = 1
		if result.Status != HealthStatusHealthy {
			result.FailureCount = 1
			now := time.Now()
			result.LastFailure = &now
		} else {
			now := time.Now()
			result.LastSuccess = &now
		}
	}

	hm.results[name] = result
	hm.mutex.Unlock()

	hm.logger.Debug().
		Str("checker", name).
		Str("status", string(result.Status)).
		Dur("response_time", responseTime).
		Msg("Health check completed")
}

// GetHealth returns the current overall health status
func (hm *HealthMonitor) GetHealth() OverallHealth {
	hm.mutex.RLock()
	defer hm.mutex.RUnlock()

	components := make(map[string]ComponentHealth)
	for name, result := range hm.results {
		components[name] = result
	}

	summary := hm.calculateSummary()
	status := hm.calculateOverallStatus(summary)

	return OverallHealth{
		Status:     status,
		Timestamp:  time.Now(),
		Version:    hm.version,
		Uptime:     time.Since(hm.startTime),
		Components: components,
		Summary:    summary,
		Details: map[string]interface{}{
			"check_interval": hm.checkInterval,
			"timeout":        hm.timeout,
			"running":        hm.running,
		},
	}
}

// GetReadiness returns the current readiness status
func (hm *HealthMonitor) GetReadiness() ReadinessStatus {
	hm.mutex.RLock()
	defer hm.mutex.RUnlock()

	checks := make([]ReadinessCheck, 0, len(hm.results))
	overallReady := true
	var messages []string

	for name, result := range hm.results {
		ready := result.Status == HealthStatusHealthy
		if !ready {
			overallReady = false
			messages = append(messages, fmt.Sprintf("%s: %s", name, result.Message))
		}

		checks = append(checks, ReadinessCheck{
			Component:    name,
			Ready:        ready,
			Message:      result.Message,
			CheckTime:    result.LastChecked,
			ResponseTime: result.ResponseTime,
		})
	}

	message := ""
	if !overallReady {
		message = fmt.Sprintf("Not ready: %v", messages)
	}

	return ReadinessStatus{
		Ready:     overallReady,
		Timestamp: time.Now(),
		Checks:    checks,
		Message:   message,
	}
}

// calculateSummary calculates health summary statistics
func (hm *HealthMonitor) calculateSummary() HealthSummary {
	summary := HealthSummary{
		TotalComponents: len(hm.results),
	}

	for _, result := range hm.results {
		switch result.Status {
		case HealthStatusHealthy:
			summary.HealthyComponents++
		case HealthStatusDegraded:
			summary.DegradedComponents++
		case HealthStatusUnhealthy:
			summary.UnhealthyComponents++
		}
	}

	return summary
}

// calculateOverallStatus determines overall status from component statuses
func (hm *HealthMonitor) calculateOverallStatus(summary HealthSummary) HealthStatus {
	if summary.UnhealthyComponents > 0 {
		return HealthStatusUnhealthy
	}
	if summary.DegradedComponents > 0 {
		return HealthStatusDegraded
	}
	if summary.HealthyComponents > 0 {
		return HealthStatusHealthy
	}
	return HealthStatusUnhealthy
}

// HealthEndpointHandler provides HTTP handlers for health endpoints
type HealthEndpointHandler struct {
	monitor *HealthMonitor
	logger  zerolog.Logger
}

// NewHealthEndpointHandler creates a new health endpoint handler
func NewHealthEndpointHandler(monitor *HealthMonitor, logger zerolog.Logger) *HealthEndpointHandler {
	return &HealthEndpointHandler{
		monitor: monitor,
		logger:  logger.With().Str("component", "health_endpoints").Logger(),
	}
}

// HealthzHandler handles /healthz endpoint requests
func (h *HealthEndpointHandler) HealthzHandler(w http.ResponseWriter, _ *http.Request) {
	health := h.monitor.GetHealth()

	// Set status code based on health
	var statusCode int
	switch health.Status {
	case HealthStatusUnhealthy:
		statusCode = http.StatusServiceUnavailable
	case HealthStatusDegraded:
		statusCode = http.StatusOK // Still available but degraded
	default:
		statusCode = http.StatusOK
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(health); err != nil {
		h.logger.Error().Err(err).Msg("Failed to encode health response")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.logger.Debug().
		Str("status", string(health.Status)).
		Int("status_code", statusCode).
		Msg("Health check endpoint accessed")
}

// ReadyzHandler handles /readyz endpoint requests
func (h *HealthEndpointHandler) ReadyzHandler(w http.ResponseWriter, _ *http.Request) {
	readiness := h.monitor.GetReadiness()

	// Set status code based on readiness
	statusCode := http.StatusOK
	if !readiness.Ready {
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(readiness); err != nil {
		h.logger.Error().Err(err).Msg("Failed to encode readiness response")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.logger.Debug().
		Bool("ready", readiness.Ready).
		Int("status_code", statusCode).
		Msg("Readiness check endpoint accessed")
}

// LivezHandler handles /livez endpoint requests (simple liveness check)
func (h *HealthEndpointHandler) LivezHandler(w http.ResponseWriter, _ *http.Request) {
	liveness := map[string]interface{}{
		"status":    "alive",
		"timestamp": time.Now(),
		"uptime":    time.Since(h.monitor.startTime),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(liveness); err != nil {
		h.logger.Error().Err(err).Msg("Failed to encode liveness response")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.logger.Debug().Msg("Liveness check endpoint accessed")
}

// RegisterRoutes registers health check routes with a HTTP mux
func (h *HealthEndpointHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/healthz", h.HealthzHandler)
	mux.HandleFunc("/readyz", h.ReadyzHandler)
	mux.HandleFunc("/livez", h.LivezHandler)

	h.logger.Info().Msg("Health check endpoints registered: /healthz, /readyz, /livez")
}
