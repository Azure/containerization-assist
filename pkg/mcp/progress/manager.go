// Package progress provides a unified progress reporting system that works with both MCP and CLI
package progress

import (
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/security"
	"github.com/briandowns/spinner"
	"github.com/localrivet/gomcp/mcp"
	"github.com/localrivet/gomcp/server"
)

// Manager provides a unified progress reporting interface that bridges MCP and CLI
type Manager struct {
	reporter      *mcp.ProgressReporter
	spinner       *spinner.Spinner
	total         float64
	current       int
	logger        *slog.Logger
	mu            sync.Mutex
	isCLI         bool
	isCI          bool
	startTime     time.Time
	lastUpdate    time.Time
	minUpdateTime time.Duration
	watchdogTimer *time.Timer
	watchdogStop  chan bool
	stepDurations map[string]time.Duration
	traceID       string
	errorBudget   *ErrorBudget
}

// New creates a new progress manager that automatically falls back to CLI mode
// when no MCP token exists
func New(ctx *server.Context, totalSteps int, logger *slog.Logger) *Manager {
	m := &Manager{
		total:         float64(totalSteps),
		current:       0,
		logger:        logger,
		startTime:     time.Now(),
		lastUpdate:    time.Now(),
		minUpdateTime: 100 * time.Millisecond, // Throttle updates to max 10/second
		isCI:          os.Getenv("CI") == "true",
		watchdogStop:  make(chan bool, 1),
		stepDurations: make(map[string]time.Duration),
		traceID:       generateTraceID(),
		errorBudget:   NewErrorBudget(5, 10*time.Minute), // Allow 5 errors per 10 minutes
	}

	// Add trace ID to logger context
	m.logger = logger.With("trace_id", m.traceID)

	// Check if we have MCP context with progress token
	if ctx != nil && ctx.HasProgressToken() {
		reporter := ctx.CreateSimpleProgressReporter(&m.total)
		if reporter != nil {
			m.reporter = reporter
			m.isCLI = false
			m.logger.Debug("MCP progress reporting enabled")
		} else {
			m.logger.Warn("Failed to create MCP progress reporter, falling back to CLI")
			m.setupCLIFallback()
		}
	} else {
		m.setupCLIFallback()
	}

	// Start watchdog timer
	m.startWatchdog()

	return m
}

// setupCLIFallback configures spinner for non-MCP environments
func (m *Manager) setupCLIFallback() {
	m.isCLI = true

	// Don't use spinner in CI environments
	if !m.isCI {
		m.spinner = spinner.New(spinner.CharSets[14], 100*time.Millisecond)
		m.spinner.Prefix = "Progress: "
		m.spinner.Color("cyan", "bold")
	}

	m.logger.Debug("CLI progress reporting enabled", "isCI", m.isCI)
}

// Begin starts the progress tracking (required by VS Code progress UI)
func (m *Manager) Begin(msg string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Start watchdog timer
	m.resetWatchdog()

	if m.reporter != nil {
		// TODO: Use correct MCP progress API when available
		// The gomcp library may not expose Begin/Update/Done methods yet
		m.logger.Debug("MCP progress begin", "message", msg)
	} else if m.isCLI {
		if m.isCI {
			fmt.Printf("[BEGIN] %s\n", msg)
		} else {
			m.spinner.Suffix = fmt.Sprintf(" %s", msg)
			m.spinner.Start()
		}
	}
}

// Update advances the progress bar or prints to stdout
func (m *Manager) Update(step int, msg string, metadata map[string]interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Throttle updates to prevent overwhelming the UI
	if time.Since(m.lastUpdate) < m.minUpdateTime && step != int(m.total) {
		return
	}

	// Reset watchdog timer on update
	m.resetWatchdog()

	// Track step duration if we moved forward
	if step > m.current && metadata != nil {
		if stepName, ok := metadata["step_name"].(string); ok {
			m.stepDurations[stepName] = time.Since(m.lastUpdate)
		}
	}

	m.current = step
	m.lastUpdate = time.Now()
	percentage := int((float64(step) / m.total) * 100)

	// Enrich metadata
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	metadata["kind"] = "progress"
	metadata["step"] = step
	metadata["total"] = m.total
	metadata["percentage"] = percentage
	metadata["elapsed"] = time.Since(m.startTime).String()
	metadata["trace_id"] = m.traceID

	// Add status codes for styling
	if status, ok := metadata["status"].(string); ok {
		metadata["status_code"] = mapStatusToCode(status)
	}

	// Calculate ETA
	if eta := m.calculateETA(); eta > 0 {
		metadata["eta_ms"] = int(eta.Milliseconds())
		metadata["eta_human"] = eta.Round(time.Second).String()
	}

	// Format message with percentage
	formattedMsg := fmt.Sprintf("[%d%%] %s", percentage, msg)

	if m.reporter != nil {
		if err := m.reporter.Update(float64(step), formattedMsg); err != nil {
			m.logger.Warn("Failed to send progress update",
				"error", err,
				"step", step,
				"message", msg)
		}
	} else if m.isCLI {
		if m.isCI {
			// Simple output for CI
			fmt.Printf("[%d/%d] %s\n", step, int(m.total), formattedMsg)
		} else {
			// Update spinner with progress
			progressBar := m.renderProgressBar(percentage)
			m.spinner.Suffix = fmt.Sprintf(" %s %s", progressBar, msg)
		}
	}

	// Log structured progress for debugging (with masked sensitive data)
	maskedMetadata := security.MaskMap(metadata)
	m.logger.Debug("Progress update",
		"step", step,
		"total", m.total,
		"percentage", percentage,
		"message", security.Mask(msg),
		"metadata", maskedMetadata)
}

// Complete finishes with a final message (required by VS Code progress UI)
func (m *Manager) Complete(msg string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Stop watchdog timer
	m.stopWatchdog()

	duration := time.Since(m.startTime)
	finalMsg := fmt.Sprintf("%s (completed in %s)", msg, duration.Round(time.Second))

	if m.reporter != nil {
		if err := m.reporter.Complete(finalMsg); err != nil {
			m.logger.Warn("Failed to send complete progress", "error", err)
		}
	} else if m.isCLI {
		if m.isCI {
			fmt.Printf("[COMPLETE] %s\n", finalMsg)
		} else {
			m.spinner.Stop()
			fmt.Printf("✅ %s\n", finalMsg)
		}
	}

	m.logger.Info("Progress completed",
		"duration", duration,
		"message", msg,
		"trace_id", m.traceID)
}

// Finish cleans up resources
func (m *Manager) Finish() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Stop watchdog timer
	m.stopWatchdog()

	if m.reporter != nil {
		// TODO: Use correct MCP progress API when available
		m.logger.Debug("MCP progress done")
	} else if m.spinner != nil && !m.isCI {
		m.spinner.Stop()
	}
}

// renderProgressBar creates a visual progress bar for CLI
func (m *Manager) renderProgressBar(percentage int) string {
	const barWidth = 20
	filled := (percentage * barWidth) / 100
	empty := barWidth - filled

	return fmt.Sprintf("[%s%s]",
		repeatChar('█', filled),
		repeatChar('░', empty))
}

// repeatChar repeats a character n times
func repeatChar(char rune, n int) string {
	if n <= 0 {
		return ""
	}
	result := make([]rune, n)
	for i := range result {
		result[i] = char
	}
	return string(result)
}

// GetCurrent returns the current step number
func (m *Manager) GetCurrent() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.current
}

// GetTotal returns the total number of steps
func (m *Manager) GetTotal() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return int(m.total)
}

// IsComplete returns true if all steps are completed
func (m *Manager) IsComplete() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.current >= int(m.total)
}

// GetTraceID returns the trace ID for correlation
func (m *Manager) GetTraceID() string {
	return m.traceID
}

// RecordError records an error in the error budget
func (m *Manager) RecordError(err error) bool {
	within := m.errorBudget.RecordError(err)
	if !within {
		m.logger.Error("Error budget exceeded",
			"error", err,
			"budget_status", m.errorBudget.GetStatus().String(),
			"trace_id", m.traceID)
	}
	return within
}

// RecordSuccess records a successful operation
func (m *Manager) RecordSuccess() {
	m.errorBudget.RecordSuccess()
}

// IsCircuitOpen returns whether the circuit breaker is open
func (m *Manager) IsCircuitOpen() bool {
	return m.errorBudget.IsCircuitOpen()
}

// GetErrorBudgetStatus returns the current error budget status
func (m *Manager) GetErrorBudgetStatus() ErrorBudgetStatus {
	return m.errorBudget.GetStatus()
}

// UpdateWithErrorHandling updates progress and handles errors through error budget
func (m *Manager) UpdateWithErrorHandling(step int, msg string, metadata map[string]interface{}, err error) bool {
	if err != nil {
		if !m.RecordError(err) {
			// Error budget exceeded
			if metadata == nil {
				metadata = make(map[string]interface{})
			}
			metadata["error_budget_exceeded"] = true
			metadata["circuit_open"] = m.IsCircuitOpen()
		}

		// Add error info to metadata
		if metadata == nil {
			metadata = make(map[string]interface{})
		}
		metadata["error"] = err.Error()
		metadata["error_budget_status"] = m.GetErrorBudgetStatus()
	} else {
		m.RecordSuccess()
	}

	// Regular update
	m.Update(step, msg, metadata)

	return err == nil && !m.IsCircuitOpen()
}

// startWatchdog starts a timer that sends heartbeat updates if no progress for 30s
func (m *Manager) startWatchdog() {
	go func() {
		ticker := time.NewTicker(15 * time.Second) // Send heartbeat every 15s instead of 30s
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				m.mu.Lock()
				if time.Since(m.lastUpdate) >= 15*time.Second && !m.IsComplete() {
					// Send heartbeat with kind=heartbeat metadata
					msg := fmt.Sprintf("Still working on step %d/%d...", m.current, int(m.total))
					metadata := map[string]interface{}{
						"kind":       "heartbeat",
						"step":       m.current,
						"total":      m.total,
						"percentage": int((float64(m.current) / m.total) * 100),
						"trace_id":   m.traceID,
						"elapsed":    time.Since(m.startTime).String(),
					}
					
					if m.reporter != nil {
						m.reporter.Update(float64(m.current), msg)
					} else if !m.isCI && m.spinner != nil {
						m.spinner.Suffix = fmt.Sprintf(" Still working... (step %d/%d)", m.current, int(m.total))
					}
					
					// Log heartbeat with metadata
					m.logger.Debug("Watchdog heartbeat sent", 
						"current_step", m.current,
						"metadata", metadata)
						
					// Update lastUpdate to prevent immediate re-triggering
					m.lastUpdate = time.Now()
				}
				m.mu.Unlock()
			case <-m.watchdogStop:
				return
			}
		}
	}()
}

// stopWatchdog stops the watchdog timer
func (m *Manager) stopWatchdog() {
	if m.watchdogTimer != nil {
		m.watchdogTimer.Stop()
		m.watchdogTimer = nil
	}
	select {
	case m.watchdogStop <- true:
	default:
	}
}

// resetWatchdog resets the watchdog timer
func (m *Manager) resetWatchdog() {
	if m.watchdogTimer != nil {
		m.watchdogTimer.Stop()
		m.watchdogTimer = nil
	}

	// Set up new watchdog timer (15 seconds to match startWatchdog)
	m.watchdogTimer = time.AfterFunc(15*time.Second, func() {
		m.mu.Lock()
		defer m.mu.Unlock()

		// Send a heartbeat message with metadata
		msg := fmt.Sprintf("Still working on step %d/%d...", m.current, int(m.total))
		metadata := map[string]interface{}{
			"kind":       "heartbeat",
			"step":       m.current,
			"total":      m.total,
			"percentage": int((float64(m.current) / m.total) * 100),
			"trace_id":   m.traceID,
		}
		
		if m.reporter != nil {
			// TODO: When gomcp is updated, use proper progress/update with metadata
			m.reporter.Update(float64(m.current), msg)
			m.logger.Debug("Watchdog heartbeat", "message", msg, "metadata", metadata)
		} else if m.isCLI && m.spinner != nil {
			m.spinner.Suffix = " " + msg
		}

		// Reset watchdog again
		m.resetWatchdog()
	})
}

// calculateETA estimates time remaining based on average step duration
func (m *Manager) calculateETA() time.Duration {
	if m.current == 0 || m.current >= int(m.total) {
		return 0
	}

	// Calculate average step duration
	elapsed := time.Since(m.startTime)
	avgStepDuration := elapsed / time.Duration(m.current)

	// Estimate remaining time
	stepsRemaining := int(m.total) - m.current
	return avgStepDuration * time.Duration(stepsRemaining)
}

// mapStatusToCode maps status strings to numeric codes for UI styling
func mapStatusToCode(status string) int {
	switch status {
	case "running":
		return 1
	case "completed":
		return 2
	case "failed":
		return 3
	case "skipped":
		return 4
	case "retrying":
		return 5
	default:
		return 0
	}
}

// generateTraceID generates a unique trace ID for correlation
func generateTraceID() string {
	// Simple trace ID: timestamp + random suffix
	return fmt.Sprintf("trace-%d-%d", time.Now().Unix(), time.Now().Nanosecond()%1000)
}
