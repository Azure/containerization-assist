// Package progress provides a unified progress reporting system that works with both MCP and CLI
package progress

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/infrastructure/security"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/tracing"
	"github.com/briandowns/spinner"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// NotificationSender is an interface for sending MCP notifications
type NotificationSender interface {
	SendNotificationToClient(ctx context.Context, method string, params interface{}) error
}

// mcpServerWrapper wraps the mcp-go MCPServer to match our interface
type mcpServerWrapper struct {
	server interface {
		SendNotificationToClient(ctx context.Context, method string, params map[string]any) error
	}
}

// SendNotificationToClient implements NotificationSender interface
func (w *mcpServerWrapper) SendNotificationToClient(ctx context.Context, method string, params interface{}) error {
	// Convert params to map[string]any
	paramsMap, ok := params.(map[string]interface{})
	if !ok {
		return fmt.Errorf("params must be map[string]interface{}, got %T", params)
	}

	// Convert map[string]interface{} to map[string]any
	anyMap := make(map[string]any, len(paramsMap))
	for k, v := range paramsMap {
		anyMap[k] = v
	}

	return w.server.SendNotificationToClient(ctx, method, anyMap)
}

// getServerFromContext attempts to extract the MCP server from the context
func getServerFromContext(ctx context.Context) NotificationSender {
	if s := server.ServerFromContext(ctx); s != nil {
		return &mcpServerWrapper{server: s}
	}
	return nil
}

// Manager provides a unified progress reporting interface that bridges MCP and CLI
type Manager struct {
	ctx           context.Context
	req           *mcp.CallToolRequest
	server        NotificationSender
	spinner       *spinner.Spinner
	total         int
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
	progressToken interface{}
}

// New creates a new progress manager that automatically falls back to CLI mode
// when no MCP token exists
func New(ctx context.Context, req *mcp.CallToolRequest, totalSteps int, logger *slog.Logger) *Manager {
	// Try to get server from context
	var server NotificationSender
	if s := getServerFromContext(ctx); s != nil {
		server = s
	}
	return NewWithServer(ctx, req, server, totalSteps, logger)
}

// NewWithServer creates a new progress manager with an optional server for sending notifications
func NewWithServer(ctx context.Context, req *mcp.CallToolRequest, server NotificationSender, totalSteps int, logger *slog.Logger) *Manager {
	m := &Manager{
		ctx:           ctx,
		req:           req,
		server:        server,
		total:         totalSteps,
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

	// Check if we have MCP progress token in request
	// If we have both server and progress token, use MCP mode
	if server != nil && req != nil && req.Params.Meta != nil && req.Params.Meta.ProgressToken != nil {
		m.progressToken = req.Params.Meta.ProgressToken
		m.isCLI = false
		m.logger.Debug("MCP progress reporting enabled", "progressToken", m.progressToken)
	} else {
		// Fall back to CLI mode
		if server != nil {
			m.logger.Info("No progress token in request, progress notifications disabled",
				"has_server", server != nil,
				"has_req", req != nil,
				"has_meta", req != nil && req.Params.Meta != nil,
				"has_token", req != nil && req.Params.Meta != nil && req.Params.Meta.ProgressToken != nil)
		}
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

	if m.progressToken != nil && m.server != nil {
		// Send progress notification using mcp-go
		params := map[string]interface{}{
			"progressToken": m.progressToken,
			"progress":      float64(m.current),
			"total":         float64(m.total),
			"message":       msg,
		}

		if err := m.server.SendNotificationToClient(m.ctx, "notifications/progress", params); err != nil {
			m.logger.Warn("Failed to send progress notification", "error", err)
		}
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
	// Create tracing span for progress update
	ctx := context.Background()
	if m.logger != nil {
		// Try to extract context from logger if available
		ctx = context.WithValue(ctx, "logger", m.logger)
	}

	stepName := "unknown"
	if metadata != nil {
		if name, ok := metadata["step_name"].(string); ok {
			stepName = name
		}
	}

	err := tracing.TraceProgressUpdate(ctx, m.traceID, stepName, step, int(m.total), func(tracedCtx context.Context) error {
		m.updateInternal(step, msg, metadata)
		return nil
	})

	if err != nil && m.logger != nil {
		m.logger.Warn("Progress update tracing failed", "error", err)
	}
}

func (m *Manager) updateInternal(step int, msg string, metadata map[string]interface{}) {
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
	percentage := int((float64(step) / float64(m.total)) * 100)

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

	if m.progressToken != nil && m.server != nil {
		// Send progress notification using mcp-go
		params := map[string]interface{}{
			"progressToken": m.progressToken,
			"progress":      float64(step),
			"total":         float64(m.total),
			"message":       formattedMsg,
		}

		if err := m.server.SendNotificationToClient(m.ctx, "notifications/progress", params); err != nil {
			m.logger.Warn("Failed to send progress notification", "error", err)
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

	if m.progressToken != nil && m.server != nil {
		// Send final progress notification
		params := map[string]interface{}{
			"progressToken": m.progressToken,
			"progress":      float64(m.total),
			"total":         float64(m.total),
			"message":       finalMsg,
		}

		if err := m.server.SendNotificationToClient(m.ctx, "notifications/progress", params); err != nil {
			m.logger.Warn("Failed to send final progress notification", "error", err)
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

	if m.progressToken != nil {
		// Progress reporting done
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

// SetCurrent sets the current progress step
func (m *Manager) SetCurrent(current int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.current = current
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
						"percentage": int((float64(m.current) / float64(m.total)) * 100),
						"trace_id":   m.traceID,
						"elapsed":    time.Since(m.startTime).String(),
					}

					if m.progressToken != nil && m.server != nil {
						// Send heartbeat progress notification
						params := map[string]interface{}{
							"progressToken": m.progressToken,
							"progress":      float64(m.current),
							"total":         float64(m.total),
							"message":       msg,
						}

						if err := m.server.SendNotificationToClient(m.ctx, "notifications/progress", params); err != nil {
							m.logger.Warn("Failed to send heartbeat notification", "error", err)
						}
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
			"percentage": int((float64(m.current) / float64(m.total)) * 100),
			"trace_id":   m.traceID,
		}

		if m.progressToken != nil && m.server != nil {
			// Send heartbeat progress notification
			params := map[string]interface{}{
				"progressToken": m.progressToken,
				"progress":      float64(m.current),
				"total":         float64(m.total),
				"message":       msg,
			}

			if err := m.server.SendNotificationToClient(m.ctx, "notifications/progress", params); err != nil {
				m.logger.Warn("Failed to send watchdog heartbeat", "error", err)
			}
		} else if m.isCLI && m.spinner != nil {
			m.spinner.Suffix = " " + msg
		}

		// Log heartbeat with metadata
		m.logger.Debug("Watchdog heartbeat sent",
			"current_step", m.current,
			"metadata", metadata)

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
