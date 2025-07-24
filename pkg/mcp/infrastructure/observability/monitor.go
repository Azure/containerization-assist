package observability

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusDegraded  Status = "degraded"
	StatusUnhealthy Status = "unhealthy"
)

type Check struct {
	Name         string            `json:"name"`
	Status       Status            `json:"status"`
	Message      string            `json:"message,omitempty"`
	LastChecked  time.Time         `json:"last_checked"`
	ResponseTime time.Duration     `json:"response_time"`
	Details      map[string]string `json:"details,omitempty"`
}

type Report struct {
	Status    Status            `json:"status"`
	Timestamp time.Time         `json:"timestamp"`
	Uptime    time.Duration     `json:"uptime"`
	Version   string            `json:"version"`
	Checks    map[string]Check  `json:"checks"`
	Summary   map[string]int    `json:"summary"`
	Details   map[string]string `json:"details,omitempty"`
}

type Checker interface {
	Name() string
	Check(ctx context.Context) Check
}

type Monitor struct {
	mu        sync.RWMutex
	logger    *slog.Logger
	checkers  map[string]Checker
	results   map[string]Check
	startTime time.Time
	version   string
}

func NewMonitor(logger *slog.Logger) *Monitor {
	return &Monitor{
		logger:    logger.With("component", "health_monitor"),
		checkers:  make(map[string]Checker),
		results:   make(map[string]Check),
		startTime: time.Now(),
		version:   "0.0.6",
	}
}

func (m *Monitor) SetVersion(version string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.version = version
}

func (m *Monitor) RegisterChecker(checker Checker) {
	m.mu.Lock()
	defer m.mu.Unlock()

	name := checker.Name()
	m.checkers[name] = checker
	m.logger.Debug("Health checker registered", "name", name)
}

func (m *Monitor) UnregisterChecker(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.checkers, name)
	delete(m.results, name)
	m.logger.Debug("Health checker unregistered", "name", name)
}

func (m *Monitor) CheckAll(ctx context.Context) {
	m.mu.RLock()
	checkers := make([]Checker, 0, len(m.checkers))
	for _, checker := range m.checkers {
		checkers = append(checkers, checker)
	}
	m.mu.RUnlock()

	// Run checks concurrently
	var wg sync.WaitGroup
	results := make(chan Check, len(checkers))

	for _, checker := range checkers {
		wg.Add(1)
		go func(c Checker) {
			defer wg.Done()
			start := time.Now()

			// Run check with timeout
			checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			result := c.Check(checkCtx)
			result.LastChecked = start
			result.ResponseTime = time.Since(start)

			results <- result
		}(checker)
	}

	// Close results channel when all checks complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	m.mu.Lock()
	defer m.mu.Unlock()

	for result := range results {
		m.results[result.Name] = result
		m.logger.Debug("Health check completed",
			"name", result.Name,
			"status", result.Status,
			"duration", result.ResponseTime)
	}
}

func (m *Monitor) GetReport() Report {
	m.mu.RLock()
	defer m.mu.RUnlock()

	checks := make(map[string]Check)
	for name, result := range m.results {
		checks[name] = result
	}

	// Calculate summary
	summary := map[string]int{
		"total":     0,
		"healthy":   0,
		"degraded":  0,
		"unhealthy": 0,
	}

	overallStatus := StatusHealthy
	for _, check := range checks {
		summary["total"]++
		switch check.Status {
		case StatusHealthy:
			summary["healthy"]++
		case StatusDegraded:
			summary["degraded"]++
			if overallStatus == StatusHealthy {
				overallStatus = StatusDegraded
			}
		case StatusUnhealthy:
			summary["unhealthy"]++
			overallStatus = StatusUnhealthy
		}
	}

	// If no checks registered, still consider healthy
	if summary["total"] == 0 {
		overallStatus = StatusHealthy
	}

	return Report{
		Status:    overallStatus,
		Timestamp: time.Now(),
		Uptime:    time.Since(m.startTime),
		Version:   m.version,
		Checks:    checks,
		Summary:   summary,
		Details: map[string]string{
			"service": "container-kit-mcp",
		},
	}
}

// IsHealthy returns true if the overall status is healthy or degraded
func (m *Monitor) IsHealthy() bool {
	report := m.GetReport()
	return report.Status == StatusHealthy || report.Status == StatusDegraded
}

// IsReady returns true if all critical services are healthy
func (m *Monitor) IsReady() bool {
	report := m.GetReport()
	return report.Status == StatusHealthy
}

type BasicChecker struct {
	name    string
	checkFn func(ctx context.Context) (Status, string, map[string]string)
}

func NewBasicChecker(name string, checkFn func(ctx context.Context) (Status, string, map[string]string)) *BasicChecker {
	return &BasicChecker{
		name:    name,
		checkFn: checkFn,
	}
}

func (c *BasicChecker) Name() string {
	return c.name
}

func (c *BasicChecker) Check(ctx context.Context) Check {
	status, message, details := c.checkFn(ctx)
	return Check{
		Name:    c.name,
		Status:  status,
		Message: message,
		Details: details,
	}
}
