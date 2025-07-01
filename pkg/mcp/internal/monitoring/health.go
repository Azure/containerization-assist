package monitoring

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// HealthChecker provides comprehensive health checking
type HealthChecker struct {
	logger       zerolog.Logger
	mutex        sync.RWMutex
	components   map[string]*ComponentHealth
	dependencies map[string]*DependencyHealth
	overall      *OverallHealth
	config       HealthConfig

	// Health check runners
	checkInterval time.Duration
	isRunning     bool
	stopChan      chan struct{}

	// Callbacks
	healthChangeCallbacks []HealthChangeCallback
}

// ComponentHealth tracks the health of individual components
type ComponentHealth struct {
	Name           string                 `json:"name"`
	Status         HealthStatus           `json:"status"`
	LastCheck      time.Time              `json:"last_check"`
	LastHealthy    time.Time              `json:"last_healthy"`
	ErrorCount     int                    `json:"error_count"`
	ErrorThreshold int                    `json:"error_threshold"`
	CheckFunc      ComponentCheckFunc     `json:"-"`
	Details        map[string]interface{} `json:"details,omitempty"`
	Dependencies   []string               `json:"dependencies,omitempty"`
}

// DependencyHealth tracks external dependency health
type DependencyHealth struct {
	Name         string                 `json:"name"`
	Type         DependencyType         `json:"type"`
	Endpoint     string                 `json:"endpoint"`
	Status       HealthStatus           `json:"status"`
	LastCheck    time.Time              `json:"last_check"`
	LastHealthy  time.Time              `json:"last_healthy"`
	ResponseTime time.Duration          `json:"response_time"`
	ErrorCount   int                    `json:"error_count"`
	CheckFunc    DependencyCheckFunc    `json:"-"`
	Details      map[string]interface{} `json:"details,omitempty"`
}

// OverallHealth represents the overall system health
type OverallHealth struct {
	Status                HealthStatus  `json:"status"`
	LastUpdated           time.Time     `json:"last_updated"`
	HealthyComponents     int           `json:"healthy_components"`
	UnhealthyComponents   int           `json:"unhealthy_components"`
	HealthyDependencies   int           `json:"healthy_dependencies"`
	UnhealthyDependencies int           `json:"unhealthy_dependencies"`
	Uptime                time.Duration `json:"uptime"`
	StartTime             time.Time     `json:"start_time"`
}

// HealthStatus represents the health status
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "HEALTHY"
	HealthStatusDegraded  HealthStatus = "DEGRADED"
	HealthStatusUnhealthy HealthStatus = "UNHEALTHY"
	HealthStatusUnknown   HealthStatus = "UNKNOWN"
)

// DependencyType represents the type of dependency
type DependencyType string

const (
	DependencyTypeHTTP     DependencyType = "HTTP"
	DependencyTypeDatabase DependencyType = "DATABASE"
	DependencyTypeCache    DependencyType = "CACHE"
	DependencyTypeQueue    DependencyType = "QUEUE"
	DependencyTypeStorage  DependencyType = "STORAGE"
	DependencyTypeExternal DependencyType = "EXTERNAL"
)

// ComponentCheckFunc is a function that checks component health
type ComponentCheckFunc func(ctx context.Context) (HealthStatus, map[string]interface{}, error)

// DependencyCheckFunc is a function that checks dependency health
type DependencyCheckFunc func(ctx context.Context) (HealthStatus, time.Duration, map[string]interface{}, error)

// HealthChangeCallback is called when health status changes
type HealthChangeCallback func(name string, oldStatus, newStatus HealthStatus, details map[string]interface{})

// HealthConfig configures the health checker
type HealthConfig struct {
	CheckInterval    time.Duration     `json:"check_interval"`
	Timeout          time.Duration     `json:"timeout"`
	ErrorThreshold   int               `json:"error_threshold"`
	EnableCallbacks  bool              `json:"enable_callbacks"`
	EnableMetrics    bool              `json:"enable_metrics"`
	MetricsCollector *MetricsCollector `json:"-"`
}

// HealthReport represents a comprehensive health report
type HealthReport struct {
	Overall      *OverallHealth               `json:"overall"`
	Components   map[string]*ComponentHealth  `json:"components"`
	Dependencies map[string]*DependencyHealth `json:"dependencies"`
	Timestamp    time.Time                    `json:"timestamp"`
	Version      string                       `json:"version"`
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(config HealthConfig, logger zerolog.Logger) *HealthChecker {
	if config.CheckInterval == 0 {
		config.CheckInterval = 30 * time.Second
	}
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}
	if config.ErrorThreshold == 0 {
		config.ErrorThreshold = 5
	}

	return &HealthChecker{
		logger:                logger.With().Str("component", "health_checker").Logger(),
		components:            make(map[string]*ComponentHealth),
		dependencies:          make(map[string]*DependencyHealth),
		config:                config,
		checkInterval:         config.CheckInterval,
		stopChan:              make(chan struct{}),
		healthChangeCallbacks: make([]HealthChangeCallback, 0),
		overall: &OverallHealth{
			Status:    HealthStatusUnknown,
			StartTime: time.Now(),
		},
	}
}

// RegisterComponent registers a component for health checking
func (hc *HealthChecker) RegisterComponent(name string, checkFunc ComponentCheckFunc, dependencies ...string) error {
	hc.mutex.Lock()
	defer hc.mutex.Unlock()

	if _, exists := hc.components[name]; exists {
		return fmt.Errorf("component %s already registered", name)
	}

	hc.components[name] = &ComponentHealth{
		Name:           name,
		Status:         HealthStatusUnknown,
		ErrorThreshold: hc.config.ErrorThreshold,
		CheckFunc:      checkFunc,
		Dependencies:   dependencies,
		Details:        make(map[string]interface{}),
	}

	hc.logger.Info().
		Str("component", name).
		Strs("dependencies", dependencies).
		Msg("Component registered for health checking")

	return nil
}

// RegisterDependency registers a dependency for health checking
func (hc *HealthChecker) RegisterDependency(name string, depType DependencyType, endpoint string, checkFunc DependencyCheckFunc) error {
	hc.mutex.Lock()
	defer hc.mutex.Unlock()

	if _, exists := hc.dependencies[name]; exists {
		return fmt.Errorf("dependency %s already registered", name)
	}

	hc.dependencies[name] = &DependencyHealth{
		Name:      name,
		Type:      depType,
		Endpoint:  endpoint,
		Status:    HealthStatusUnknown,
		CheckFunc: checkFunc,
		Details:   make(map[string]interface{}),
	}

	hc.logger.Info().
		Str("dependency", name).
		Str("type", string(depType)).
		Str("endpoint", endpoint).
		Msg("Dependency registered for health checking")

	return nil
}

// AddHealthChangeCallback adds a callback for health status changes
func (hc *HealthChecker) AddHealthChangeCallback(callback HealthChangeCallback) {
	if !hc.config.EnableCallbacks {
		return
	}

	hc.mutex.Lock()
	defer hc.mutex.Unlock()

	hc.healthChangeCallbacks = append(hc.healthChangeCallbacks, callback)
}

// Start begins health checking
func (hc *HealthChecker) Start(ctx context.Context) error {
	hc.mutex.Lock()
	if hc.isRunning {
		hc.mutex.Unlock()
		return fmt.Errorf("health checker is already running")
	}
	hc.isRunning = true
	hc.mutex.Unlock()

	// Initial health check
	hc.performHealthChecks(ctx)

	// Start periodic health checking
	go hc.healthCheckLoop(ctx)

	hc.logger.Info().
		Dur("interval", hc.checkInterval).
		Msg("Health checker started")

	return nil
}

// Stop stops health checking
func (hc *HealthChecker) Stop() {
	hc.mutex.Lock()
	defer hc.mutex.Unlock()

	if !hc.isRunning {
		return
	}

	close(hc.stopChan)
	hc.isRunning = false

	hc.logger.Info().Msg("Health checker stopped")
}

// healthCheckLoop runs the periodic health checks
func (hc *HealthChecker) healthCheckLoop(ctx context.Context) {
	ticker := time.NewTicker(hc.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-hc.stopChan:
			return
		case <-ticker.C:
			hc.performHealthChecks(ctx)
		}
	}
}

// performHealthChecks executes all health checks
func (hc *HealthChecker) performHealthChecks(ctx context.Context) {
	checkCtx, cancel := context.WithTimeout(ctx, hc.config.Timeout)
	defer cancel()

	// Check dependencies first
	hc.checkDependencies(checkCtx)

	// Check components
	hc.checkComponents(checkCtx)

	// Update overall health
	hc.updateOverallHealth()

	hc.logger.Debug().Msg("Health checks completed")
}

// checkDependencies checks all registered dependencies
func (hc *HealthChecker) checkDependencies(ctx context.Context) {
	hc.mutex.Lock()
	dependencies := make([]*DependencyHealth, 0, len(hc.dependencies))
	for _, dep := range hc.dependencies {
		dependencies = append(dependencies, dep)
	}
	hc.mutex.Unlock()

	for _, dep := range dependencies {
		hc.checkSingleDependency(ctx, dep)
	}
}

// checkSingleDependency checks a single dependency
func (hc *HealthChecker) checkSingleDependency(ctx context.Context, dep *DependencyHealth) {
	oldStatus := dep.Status

	status, responseTime, details, err := dep.CheckFunc(ctx)

	dep.LastCheck = time.Now()
	dep.ResponseTime = responseTime

	if err != nil {
		dep.ErrorCount++
		dep.Status = HealthStatusUnhealthy
		dep.Details["error"] = err.Error()

		hc.logger.Error().
			Err(err).
			Str("dependency", dep.Name).
			Msg("Dependency health check failed")
	} else {
		dep.ErrorCount = 0
		dep.Status = status
		dep.LastHealthy = time.Now()
		dep.Details = details
	}

	// Update metrics if enabled
	if hc.config.EnableMetrics && hc.config.MetricsCollector != nil {
		hc.config.MetricsCollector.UpdateDependencyHealth(dep.Name, dep.Status == HealthStatusHealthy)
	}

	// Trigger callbacks if status changed
	if oldStatus != dep.Status && hc.config.EnableCallbacks {
		hc.triggerHealthChangeCallbacks(dep.Name, oldStatus, dep.Status, dep.Details)
	}
}

// checkComponents checks all registered components
func (hc *HealthChecker) checkComponents(ctx context.Context) {
	hc.mutex.Lock()
	components := make([]*ComponentHealth, 0, len(hc.components))
	for _, comp := range hc.components {
		components = append(components, comp)
	}
	hc.mutex.Unlock()

	for _, comp := range components {
		hc.checkSingleComponent(ctx, comp)
	}
}

// checkSingleComponent checks a single component
func (hc *HealthChecker) checkSingleComponent(ctx context.Context, comp *ComponentHealth) {
	oldStatus := comp.Status

	// Check if dependencies are healthy first
	if !hc.areDependenciesHealthy(comp.Dependencies) {
		comp.Status = HealthStatusDegraded
		comp.Details["reason"] = "unhealthy dependencies"
		comp.LastCheck = time.Now()
		return
	}

	status, details, err := comp.CheckFunc(ctx)

	comp.LastCheck = time.Now()

	if err != nil {
		comp.ErrorCount++
		if comp.ErrorCount >= comp.ErrorThreshold {
			comp.Status = HealthStatusUnhealthy
		} else {
			comp.Status = HealthStatusDegraded
		}
		comp.Details["error"] = err.Error()
		comp.Details["error_count"] = comp.ErrorCount

		hc.logger.Error().
			Err(err).
			Str("component", comp.Name).
			Int("error_count", comp.ErrorCount).
			Msg("Component health check failed")
	} else {
		comp.ErrorCount = 0
		comp.Status = status
		comp.LastHealthy = time.Now()
		comp.Details = details
	}

	// Update metrics if enabled
	if hc.config.EnableMetrics && hc.config.MetricsCollector != nil {
		hc.config.MetricsCollector.UpdateComponentHealth(comp.Name, comp.Status == HealthStatusHealthy)
	}

	// Trigger callbacks if status changed
	if oldStatus != comp.Status && hc.config.EnableCallbacks {
		hc.triggerHealthChangeCallbacks(comp.Name, oldStatus, comp.Status, comp.Details)
	}
}

// areDependenciesHealthy checks if all specified dependencies are healthy
func (hc *HealthChecker) areDependenciesHealthy(dependencies []string) bool {
	hc.mutex.RLock()
	defer hc.mutex.RUnlock()

	for _, depName := range dependencies {
		if dep, exists := hc.dependencies[depName]; exists {
			if dep.Status != HealthStatusHealthy {
				return false
			}
		} else {
			// Unknown dependency is considered unhealthy
			return false
		}
	}

	return true
}

// updateOverallHealth calculates and updates the overall system health
func (hc *HealthChecker) updateOverallHealth() {
	hc.mutex.Lock()
	defer hc.mutex.Unlock()

	healthyComponents := 0
	unhealthyComponents := 0
	healthyDependencies := 0
	unhealthyDependencies := 0

	// Count component health
	for _, comp := range hc.components {
		if comp.Status == HealthStatusHealthy {
			healthyComponents++
		} else {
			unhealthyComponents++
		}
	}

	// Count dependency health
	for _, dep := range hc.dependencies {
		if dep.Status == HealthStatusHealthy {
			healthyDependencies++
		} else {
			unhealthyDependencies++
		}
	}

	// Calculate overall status
	var overallStatus HealthStatus
	if unhealthyComponents == 0 && unhealthyDependencies == 0 {
		overallStatus = HealthStatusHealthy
	} else if unhealthyComponents < len(hc.components)/2 && unhealthyDependencies < len(hc.dependencies)/2 {
		overallStatus = HealthStatusDegraded
	} else {
		overallStatus = HealthStatusUnhealthy
	}

	hc.overall.Status = overallStatus
	hc.overall.LastUpdated = time.Now()
	hc.overall.HealthyComponents = healthyComponents
	hc.overall.UnhealthyComponents = unhealthyComponents
	hc.overall.HealthyDependencies = healthyDependencies
	hc.overall.UnhealthyDependencies = unhealthyDependencies
	hc.overall.Uptime = time.Since(hc.overall.StartTime)
}

// triggerHealthChangeCallbacks triggers all registered health change callbacks
func (hc *HealthChecker) triggerHealthChangeCallbacks(name string, oldStatus, newStatus HealthStatus, details map[string]interface{}) {
	for _, callback := range hc.healthChangeCallbacks {
		go func(cb HealthChangeCallback) {
			defer func() {
				if r := recover(); r != nil {
					hc.logger.Error().
						Interface("panic", r).
						Str("component", name).
						Msg("Health change callback panicked")
				}
			}()
			cb(name, oldStatus, newStatus, details)
		}(callback)
	}
}

// GetHealth returns the current health status
func (hc *HealthChecker) GetHealth() *HealthReport {
	hc.mutex.RLock()
	defer hc.mutex.RUnlock()

	// Deep copy components
	components := make(map[string]*ComponentHealth)
	for name, comp := range hc.components {
		componentCopy := *comp
		componentCopy.Details = make(map[string]interface{})
		for k, v := range comp.Details {
			componentCopy.Details[k] = v
		}
		components[name] = &componentCopy
	}

	// Deep copy dependencies
	dependencies := make(map[string]*DependencyHealth)
	for name, dep := range hc.dependencies {
		dependencyCopy := *dep
		dependencyCopy.Details = make(map[string]interface{})
		for k, v := range dep.Details {
			dependencyCopy.Details[k] = v
		}
		dependencies[name] = &dependencyCopy
	}

	// Copy overall health
	overallCopy := *hc.overall

	return &HealthReport{
		Overall:      &overallCopy,
		Components:   components,
		Dependencies: dependencies,
		Timestamp:    time.Now(),
		Version:      "1.0.0",
	}
}

// GetComponentHealth returns the health of a specific component
func (hc *HealthChecker) GetComponentHealth(name string) (*ComponentHealth, bool) {
	hc.mutex.RLock()
	defer hc.mutex.RUnlock()

	comp, exists := hc.components[name]
	if !exists {
		return nil, false
	}

	// Return a copy
	compCopy := *comp
	compCopy.Details = make(map[string]interface{})
	for k, v := range comp.Details {
		compCopy.Details[k] = v
	}

	return &compCopy, true
}

// GetDependencyHealth returns the health of a specific dependency
func (hc *HealthChecker) GetDependencyHealth(name string) (*DependencyHealth, bool) {
	hc.mutex.RLock()
	defer hc.mutex.RUnlock()

	dep, exists := hc.dependencies[name]
	if !exists {
		return nil, false
	}

	// Return a copy
	depCopy := *dep
	depCopy.Details = make(map[string]interface{})
	for k, v := range dep.Details {
		depCopy.Details[k] = v
	}

	return &depCopy, true
}

// IsHealthy returns whether the system is healthy
func (hc *HealthChecker) IsHealthy() bool {
	hc.mutex.RLock()
	defer hc.mutex.RUnlock()

	return hc.overall.Status == HealthStatusHealthy
}

// HTTPHandler returns an HTTP handler for health checks
func (hc *HealthChecker) HTTPHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		health := hc.GetHealth()

		// Set response code based on health status
		switch health.Overall.Status {
		case HealthStatusHealthy:
			w.WriteHeader(http.StatusOK)
		case HealthStatusDegraded:
			w.WriteHeader(http.StatusOK) // 200 but degraded
		case HealthStatusUnhealthy:
			w.WriteHeader(http.StatusServiceUnavailable)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}

		w.Header().Set("Content-Type", "application/json")

		// Simple JSON response
		fmt.Fprintf(w, `{
			"status": "%s",
			"timestamp": "%s",
			"uptime": "%s",
			"healthy_components": %d,
			"unhealthy_components": %d,
			"healthy_dependencies": %d,
			"unhealthy_dependencies": %d
		}`,
			health.Overall.Status,
			health.Timestamp.Format(time.RFC3339),
			health.Overall.Uptime.String(),
			health.Overall.HealthyComponents,
			health.Overall.UnhealthyComponents,
			health.Overall.HealthyDependencies,
			health.Overall.UnhealthyDependencies,
		)
	}
}

// DefaultHTTPDependencyCheck creates a default HTTP dependency check function
func DefaultHTTPDependencyCheck(endpoint string) DependencyCheckFunc {
	return func(ctx context.Context) (HealthStatus, time.Duration, map[string]interface{}, error) {
		start := time.Now()

		req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
		if err != nil {
			return HealthStatusUnhealthy, 0, nil, fmt.Errorf("failed to create request: %w", err)
		}

		client := &http.Client{
			Timeout: 10 * time.Second,
		}

		resp, err := client.Do(req)
		responseTime := time.Since(start)

		if err != nil {
			return HealthStatusUnhealthy, responseTime, map[string]interface{}{
				"endpoint": endpoint,
				"error":    err.Error(),
			}, err
		}
		defer resp.Body.Close()

		details := map[string]interface{}{
			"endpoint":    endpoint,
			"status_code": resp.StatusCode,
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return HealthStatusHealthy, responseTime, details, nil
		} else if resp.StatusCode >= 300 && resp.StatusCode < 500 {
			return HealthStatusDegraded, responseTime, details, nil
		} else {
			return HealthStatusUnhealthy, responseTime, details, fmt.Errorf("HTTP %d", resp.StatusCode)
		}
	}
}
