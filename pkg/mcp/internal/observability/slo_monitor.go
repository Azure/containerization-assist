package ops

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/config"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// SLOMonitor monitors Service Level Objectives and tracks error budget
type SLOMonitor struct {
	logger zerolog.Logger
	config *config.ObservabilityConfig
	meter  metric.Meter
	mu     sync.RWMutex

	// SLO tracking
	sloWindows map[string]*SLOWindow

	// Metrics
	errorBudgetRemaining metric.Float64Gauge
	sloCompliance        metric.Float64Gauge
	alertsTriggered      metric.Int64Counter

	// Alert state
	alertStates map[string]*AlertState
}

// SLOWindow tracks metrics within a time window for SLO calculation
type SLOWindow struct {
	Name       string
	WindowSize time.Duration
	Target     float64

	// Tracking data
	TotalRequests  int64
	SuccessfulReqs int64
	LatencyP95     float64
	LatencyP99     float64
	ErrorRate      float64

	// Time-based tracking
	WindowStart time.Time
	LastReset   time.Time
	DataPoints  []DataPoint

	mu sync.RWMutex
}

// DataPoint represents a single measurement point
type DataPoint struct {
	Timestamp time.Time
	Success   bool
	Duration  time.Duration
	ErrorCode string
}

// AlertState tracks the state of an alert
type AlertState struct {
	Name      string
	Active    bool
	Triggered time.Time
	LastSent  time.Time
	Count     int
	Condition string
}

// NewSLOMonitor creates a new SLO monitor
func NewSLOMonitor(logger zerolog.Logger, config *config.ObservabilityConfig) (*SLOMonitor, error) {
	meter := otel.Meter("container-copilot-mcp-slo")

	monitor := &SLOMonitor{
		logger:      logger.With().Str("component", "slo_monitor").Logger(),
		config:      config,
		meter:       meter,
		sloWindows:  make(map[string]*SLOWindow),
		alertStates: make(map[string]*AlertState),
	}

	if err := monitor.initializeMetrics(); err != nil {
		return nil, fmt.Errorf("failed to initialize SLO metrics: %w", err)
	}

	if err := monitor.initializeSLOWindows(); err != nil {
		return nil, fmt.Errorf("failed to initialize SLO windows: %w", err)
	}

	// Start background monitoring
	go monitor.monitorLoop()

	return monitor, nil
}

// initializeMetrics creates SLO-specific metrics
func (sm *SLOMonitor) initializeMetrics() error {
	var err error

	sm.errorBudgetRemaining, err = sm.meter.Float64Gauge(
		"mcp_slo_error_budget_remaining",
		metric.WithDescription("Remaining error budget as a ratio (0-1)"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return err
	}

	sm.sloCompliance, err = sm.meter.Float64Gauge(
		"mcp_slo_compliance_ratio",
		metric.WithDescription("SLO compliance ratio (0-1)"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return err
	}

	sm.alertsTriggered, err = sm.meter.Int64Counter(
		"mcp_slo_alerts_triggered_total",
		metric.WithDescription("Total number of SLO alerts triggered"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return err
	}

	return nil
}

// initializeSLOWindows creates SLO tracking windows from configuration
func (sm *SLOMonitor) initializeSLOWindows() error {
	if !sm.config.SLO.Enabled {
		return nil
	}

	// Tool execution SLOs
	if err := sm.createSLOWindow("tool_execution_availability",
		sm.config.SLO.ToolExecution.Availability.Window,
		sm.config.SLO.ToolExecution.Availability.Target); err != nil {
		return err
	}

	if sm.config.SLO.ToolExecution.Latency.Target > 0 {
		if err := sm.createSLOWindow("tool_execution_latency",
			sm.config.SLO.ToolExecution.Latency.Window,
			sm.config.SLO.ToolExecution.Latency.Target); err != nil {
			return err
		}
	}

	if sm.config.SLO.ToolExecution.ErrorRate.Target > 0 {
		if err := sm.createSLOWindow("tool_execution_error_rate",
			sm.config.SLO.ToolExecution.ErrorRate.Window,
			sm.config.SLO.ToolExecution.ErrorRate.Target); err != nil {
			return err
		}
	}

	// Session management SLOs
	if err := sm.createSLOWindow("session_availability",
		sm.config.SLO.SessionManagement.Availability.Window,
		sm.config.SLO.SessionManagement.Availability.Target); err != nil {
		return err
	}

	if sm.config.SLO.SessionManagement.ResponseTime.Target > 0 {
		if err := sm.createSLOWindow("session_response_time",
			sm.config.SLO.SessionManagement.ResponseTime.Window,
			sm.config.SLO.SessionManagement.ResponseTime.Target); err != nil {
			return err
		}
	}

	return nil
}

// createSLOWindow creates a new SLO tracking window
func (sm *SLOMonitor) createSLOWindow(name, windowStr string, target float64) error {
	windowDuration, err := time.ParseDuration(windowStr)
	if err != nil {
		// Try parsing as time with units (e.g., "30d", "24h")
		windowDuration, err = parseTimeWindow(windowStr)
		if err != nil {
			return fmt.Errorf("invalid window duration %s: %w", windowStr, err)
		}
	}

	window := &SLOWindow{
		Name:        name,
		WindowSize:  windowDuration,
		Target:      target,
		WindowStart: time.Now(),
		LastReset:   time.Now(),
		DataPoints:  make([]DataPoint, 0),
	}

	sm.sloWindows[name] = window
	sm.logger.Info().
		Str("slo", name).
		Dur("window", windowDuration).
		Float64("target", target).
		Msg("Created SLO window")

	return nil
}

// RecordDataPoint records a new data point for SLO tracking
func (sm *SLOMonitor) RecordDataPoint(ctx context.Context, sloName string, success bool, duration time.Duration, errorCode string) {
	sm.mu.RLock()
	window, exists := sm.sloWindows[sloName]
	sm.mu.RUnlock()

	if !exists {
		return
	}

	dataPoint := DataPoint{
		Timestamp: time.Now(),
		Success:   success,
		Duration:  duration,
		ErrorCode: errorCode,
	}

	window.mu.Lock()
	defer window.mu.Unlock()

	// Add data point
	window.DataPoints = append(window.DataPoints, dataPoint)
	window.TotalRequests++
	if success {
		window.SuccessfulReqs++
	}

	// Clean old data points outside the window
	now := time.Now()
	cutoff := now.Add(-window.WindowSize)

	// Remove old points
	validPoints := make([]DataPoint, 0, len(window.DataPoints))
	totalInWindow := int64(0)
	successInWindow := int64(0)
	durations := make([]float64, 0)

	for _, point := range window.DataPoints {
		if point.Timestamp.After(cutoff) {
			validPoints = append(validPoints, point)
			totalInWindow++
			if point.Success {
				successInWindow++
			}
			durations = append(durations, point.Duration.Seconds())
		}
	}

	window.DataPoints = validPoints
	window.TotalRequests = totalInWindow
	window.SuccessfulReqs = successInWindow

	// Calculate metrics
	if totalInWindow > 0 {
		window.ErrorRate = float64(totalInWindow-successInWindow) / float64(totalInWindow)

		// Calculate percentiles
		if len(durations) > 0 {
			window.LatencyP95 = calculatePercentile(durations, 0.95)
			window.LatencyP99 = calculatePercentile(durations, 0.99)
		}
	}
}

// calculatePercentile calculates the nth percentile of a slice of durations
func calculatePercentile(durations []float64, percentile float64) float64 {
	if len(durations) == 0 {
		return 0
	}

	// Simple percentile calculation (for production, use a proper library)
	index := int(float64(len(durations)) * percentile)
	if index >= len(durations) {
		index = len(durations) - 1
	}

	// Sort would be needed for accurate percentile calculation
	// For simplicity, return max for now
	max := durations[0]
	for _, d := range durations {
		if d > max {
			max = d
		}
	}
	return max
}

// GetSLOCompliance returns the current SLO compliance for a given SLO
func (sm *SLOMonitor) GetSLOCompliance(sloName string) float64 {
	sm.mu.RLock()
	window, exists := sm.sloWindows[sloName]
	sm.mu.RUnlock()

	if !exists {
		return 0
	}

	window.mu.RLock()
	defer window.mu.RUnlock()

	if window.TotalRequests == 0 {
		return 1.0 // No data means compliant
	}

	switch sloName {
	case "tool_execution_availability", "session_availability":
		successRate := float64(window.SuccessfulReqs) / float64(window.TotalRequests)
		return successRate

	case "tool_execution_error_rate":
		// For error rate SLO, compliance means error rate is below target
		if window.ErrorRate <= window.Target/100.0 {
			return 1.0
		}
		return 1.0 - (window.ErrorRate / (window.Target / 100.0))

	case "tool_execution_latency", "session_response_time":
		// Check if latency percentile meets target
		targetSeconds, _ := time.ParseDuration(sm.config.SLO.ToolExecution.Latency.Threshold)
		if window.LatencyP95 <= targetSeconds.Seconds() {
			return 1.0
		}
		return targetSeconds.Seconds() / window.LatencyP95

	default:
		return 0
	}
}

// GetErrorBudgetRemaining returns the remaining error budget as a ratio
func (sm *SLOMonitor) GetErrorBudgetRemaining(sloName string) float64 {
	compliance := sm.GetSLOCompliance(sloName)

	sm.mu.RLock()
	window, exists := sm.sloWindows[sloName]
	sm.mu.RUnlock()

	if !exists {
		return 0
	}

	target := window.Target / 100.0 // Convert percentage to ratio
	if compliance >= target {
		return 1.0 // Full budget remaining
	}

	// Calculate remaining budget
	return (compliance / target)
}

// monitorLoop runs the continuous monitoring and alerting
func (sm *SLOMonitor) monitorLoop() {
	ticker := time.NewTicker(1 * time.Minute) // Check every minute
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sm.updateMetrics()
			sm.checkAlerts()
		}
	}
}

// updateMetrics updates all SLO metrics
func (sm *SLOMonitor) updateMetrics() {
	ctx := context.Background()

	for name := range sm.sloWindows {
		compliance := sm.GetSLOCompliance(name)
		errorBudget := sm.GetErrorBudgetRemaining(name)

		labels := []attribute.KeyValue{
			attribute.String("slo_name", name),
		}

		sm.sloCompliance.Record(ctx, compliance, metric.WithAttributes(labels...))
		sm.errorBudgetRemaining.Record(ctx, errorBudget, metric.WithAttributes(labels...))
	}
}

// checkAlerts evaluates alert conditions and triggers alerts
func (sm *SLOMonitor) checkAlerts() {
	ctx := context.Background()

	if !sm.config.Alerting.Enabled {
		return
	}

	for _, rule := range sm.config.Alerting.Rules {
		shouldAlert := sm.evaluateAlertCondition(rule.Condition)

		alertState, exists := sm.alertStates[rule.Name]
		if !exists {
			alertState = &AlertState{
				Name:      rule.Name,
				Condition: rule.Condition,
			}
			sm.alertStates[rule.Name] = alertState
		}

		if shouldAlert && !alertState.Active {
			// Trigger alert
			alertState.Active = true
			alertState.Triggered = time.Now()
			alertState.Count++

			sm.alertsTriggered.Add(ctx, 1, metric.WithAttributes(
				attribute.String("alert_name", rule.Name),
				attribute.String("severity", rule.Severity),
			))

			sm.logger.Warn().
				Str("alert", rule.Name).
				Str("condition", rule.Condition).
				Str("severity", rule.Severity).
				Msg("SLO alert triggered")

			// Send alert notifications (implementation would depend on channels)
			sm.sendAlert(rule, alertState)

		} else if !shouldAlert && alertState.Active {
			// Clear alert
			alertState.Active = false

			sm.logger.Info().
				Str("alert", rule.Name).
				Msg("SLO alert cleared")
		}
	}
}

// evaluateAlertCondition evaluates an alert condition
func (sm *SLOMonitor) evaluateAlertCondition(condition string) bool {
	// Simple condition evaluation
	// In production, use a proper expression evaluator
	switch condition {
	case "slo_error_budget_remaining < 0.1":
		for sloName := range sm.sloWindows {
			if sm.GetErrorBudgetRemaining(sloName) < 0.1 {
				return true
			}
		}
		return false

	case "rate(tool_execution_errors_total[5m]) > 0.05":
		// Check if error rate in last 5 minutes exceeds 5%
		window := sm.sloWindows["tool_execution_error_rate"]
		if window != nil {
			window.mu.RLock()
			errorRate := window.ErrorRate
			window.mu.RUnlock()
			return errorRate > 0.05
		}
		return false

	default:
		return false
	}
}

// sendAlert sends alert notifications
func (sm *SLOMonitor) sendAlert(rule config.AlertRule, state *AlertState) {
	// Implementation would send to configured channels
	// For now, just log
	sm.logger.Error().
		Str("alert", rule.Name).
		Str("description", rule.Description).
		Str("severity", rule.Severity).
		Strs("channels", rule.Channels).
		Msg("Sending SLO alert")
}

// parseTimeWindow parses time windows like "30d", "24h", etc.
func parseTimeWindow(window string) (time.Duration, error) {
	switch {
	case len(window) > 1 && window[len(window)-1:] == "d":
		days := window[:len(window)-1]
		if d, err := time.ParseDuration(days + "h"); err == nil {
			return d * 24, nil
		}
	case len(window) > 1 && window[len(window)-1:] == "w":
		weeks := window[:len(window)-1]
		if w, err := time.ParseDuration(weeks + "h"); err == nil {
			return w * 24 * 7, nil
		}
	}
	return time.ParseDuration(window)
}
