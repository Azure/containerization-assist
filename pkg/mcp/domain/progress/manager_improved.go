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
	"github.com/briandowns/spinner"
	"github.com/mark3labs/mcp-go/mcp"
)

// progressUpdate represents an update to be processed
type progressUpdate struct {
	step     int
	message  string
	metadata map[string]interface{}
	isBegin  bool
	isEnd    bool
}

// ManagerV2 provides lock-free progress reporting using channels
type ManagerV2 struct {
	ctx           context.Context
	req           *mcp.CallToolRequest
	server        NotificationSender
	total         int
	logger        *slog.Logger
	isCLI         bool
	isCI          bool
	startTime     time.Time
	traceID       string
	errorBudget   *ErrorBudget
	progressToken interface{}

	// Channel-based updates
	updateCh chan progressUpdate
	done     chan struct{}
	wg       sync.WaitGroup

	// Read-only state (no locks needed)
	minUpdateTime time.Duration

	// State protected by atomic operations or isolated to renderer
	currentState *progressState
	stateMu      sync.RWMutex
}

// progressState holds the current progress state
type progressState struct {
	current       int
	lastUpdate    time.Time
	stepDurations map[string]time.Duration
}

// NewV2 creates a new lock-free progress manager
func NewV2(ctx context.Context, req *mcp.CallToolRequest, totalSteps int, logger *slog.Logger) *ManagerV2 {
	var server NotificationSender
	if s := getServerFromContext(ctx); s != nil {
		server = s
	}
	return NewV2WithServer(ctx, req, server, totalSteps, logger)
}

// NewV2WithServer creates a new lock-free progress manager with server
func NewV2WithServer(ctx context.Context, req *mcp.CallToolRequest, server NotificationSender, totalSteps int, logger *slog.Logger) *ManagerV2 {
	m := &ManagerV2{
		ctx:           ctx,
		req:           req,
		server:        server,
		total:         totalSteps,
		logger:        logger.With("trace_id", generateTraceID()),
		startTime:     time.Now(),
		traceID:       generateTraceID(),
		errorBudget:   NewErrorBudget(5, 10*time.Minute),
		minUpdateTime: 100 * time.Millisecond,
		isCI:          os.Getenv("CI") == "true",
		updateCh:      make(chan progressUpdate, 100), // Buffered to prevent blocking
		done:          make(chan struct{}),
		currentState: &progressState{
			current:       0,
			lastUpdate:    time.Now(),
			stepDurations: make(map[string]time.Duration),
		},
	}

	// Determine mode
	if server != nil && req != nil && req.Params.Meta != nil && req.Params.Meta.ProgressToken != nil {
		m.progressToken = req.Params.Meta.ProgressToken
		m.isCLI = false
		m.logger.Debug("MCP progress reporting enabled", "progressToken", m.progressToken)
	} else {
		m.isCLI = true
		if server != nil {
			m.logger.Info("No progress token in request, using CLI mode")
		}
	}

	// Start renderer goroutine
	m.wg.Add(1)
	go m.renderer()

	return m
}

// renderer handles all progress updates in a single goroutine
func (m *ManagerV2) renderer() {
	defer m.wg.Done()

	var spinnerInstance *spinner.Spinner
	if m.isCLI && !m.isCI {
		spinnerInstance = spinner.New(spinner.CharSets[14], 100*time.Millisecond)
		spinnerInstance.Prefix = "Progress: "
		spinnerInstance.Color("cyan", "bold")
	}

	// Heartbeat ticker
	heartbeatTicker := time.NewTicker(15 * time.Second)
	defer heartbeatTicker.Stop()

	// Throttle timer
	var throttleTimer *time.Timer
	var pendingUpdate *progressUpdate

	for {
		select {
		case update := <-m.updateCh:
			// Handle different update types
			if update.isBegin {
				m.handleBegin(update.message, spinnerInstance)
			} else if update.isEnd {
				m.handleComplete(update.message, spinnerInstance)
				return // Exit renderer
			} else {
				// Regular update - apply throttling
				now := time.Now()
				timeSinceLastUpdate := now.Sub(m.getLastUpdate())

				if timeSinceLastUpdate >= m.minUpdateTime || update.step == m.total {
					// Process immediately
					m.handleUpdate(update, spinnerInstance)
				} else {
					// Throttle - store and schedule
					pendingUpdate = &update
					if throttleTimer == nil {
						delay := m.minUpdateTime - timeSinceLastUpdate
						throttleTimer = time.AfterFunc(delay, func() {
							select {
							case m.updateCh <- *pendingUpdate:
							case <-m.done:
							}
						})
					}
				}
			}

		case <-heartbeatTicker.C:
			// Send heartbeat if no recent updates
			if time.Since(m.getLastUpdate()) >= 15*time.Second {
				m.sendHeartbeat(spinnerInstance)
			}

		case <-m.done:
			if throttleTimer != nil {
				throttleTimer.Stop()
			}
			if spinnerInstance != nil {
				spinnerInstance.Stop()
			}
			return
		}
	}
}

// Begin starts progress tracking
func (m *ManagerV2) Begin(msg string) {
	select {
	case m.updateCh <- progressUpdate{isBegin: true, message: msg}:
	case <-m.done:
	}
}

// Update advances progress
func (m *ManagerV2) Update(step int, msg string, metadata map[string]interface{}) {
	select {
	case m.updateCh <- progressUpdate{step: step, message: msg, metadata: metadata}:
	case <-m.done:
	}
}

// Complete finishes progress tracking
func (m *ManagerV2) Complete(msg string) {
	select {
	case m.updateCh <- progressUpdate{isEnd: true, message: msg}:
	case <-m.done:
	}

	// Wait for renderer to finish
	m.wg.Wait()
}

// Finish cleans up resources
func (m *ManagerV2) Finish() {
	close(m.done)
	m.wg.Wait()
}

// handleBegin processes begin messages
func (m *ManagerV2) handleBegin(msg string, spinnerInstance *spinner.Spinner) {
	if m.progressToken != nil && m.server != nil {
		params := map[string]interface{}{
			"progressToken": m.progressToken,
			"progress":      float64(0),
			"total":         float64(m.total),
			"message":       msg,
		}

		if err := m.server.SendNotificationToClient(m.ctx, "notifications/progress", params); err != nil {
			m.logger.Warn("Failed to send begin notification", "error", err)
		}
	} else if m.isCLI {
		if m.isCI {
			fmt.Printf("[BEGIN] %s\n", msg)
		} else if spinnerInstance != nil {
			spinnerInstance.Suffix = fmt.Sprintf(" %s", msg)
			spinnerInstance.Start()
		}
	}
}

// handleUpdate processes regular updates
func (m *ManagerV2) handleUpdate(update progressUpdate, spinnerInstance *spinner.Spinner) {
	// Update state
	m.stateMu.Lock()
	oldCurrent := m.currentState.current
	m.currentState.current = update.step
	m.currentState.lastUpdate = time.Now()

	// Track step duration
	if update.step > oldCurrent && update.metadata != nil {
		if stepName, ok := update.metadata["step_name"].(string); ok {
			m.currentState.stepDurations[stepName] = time.Since(m.currentState.lastUpdate)
		}
	}
	m.stateMu.Unlock()

	percentage := int((float64(update.step) / float64(m.total)) * 100)

	// Enrich metadata
	if update.metadata == nil {
		update.metadata = make(map[string]interface{})
	}
	update.metadata["kind"] = "progress"
	update.metadata["step"] = update.step
	update.metadata["total"] = m.total
	update.metadata["percentage"] = percentage
	update.metadata["elapsed"] = time.Since(m.startTime).String()
	update.metadata["trace_id"] = m.traceID

	// Calculate ETA
	if eta := m.calculateETA(); eta > 0 {
		update.metadata["eta_ms"] = int(eta.Milliseconds())
		update.metadata["eta_human"] = eta.Round(time.Second).String()
	}

	formattedMsg := fmt.Sprintf("[%d%%] %s", percentage, update.message)

	// Send notification or update spinnerInstance
	if m.progressToken != nil && m.server != nil {
		params := map[string]interface{}{
			"progressToken": m.progressToken,
			"progress":      float64(update.step),
			"total":         float64(m.total),
			"message":       formattedMsg,
		}

		if err := m.server.SendNotificationToClient(m.ctx, "notifications/progress", params); err != nil {
			m.logger.Warn("Failed to send progress notification", "error", err)
		}
	} else if m.isCLI {
		if m.isCI {
			fmt.Printf("[%d/%d] %s\n", update.step, m.total, formattedMsg)
		} else if spinnerInstance != nil {
			progressBar := m.renderProgressBar(percentage)
			spinnerInstance.Suffix = fmt.Sprintf(" %s %s", progressBar, update.message)
		}
	}

	// Log (without holding lock)
	maskedMetadata := security.MaskMap(update.metadata)
	m.logger.Debug("Progress update",
		"step", update.step,
		"total", m.total,
		"percentage", percentage,
		"message", security.Mask(update.message),
		"metadata", maskedMetadata)
}

// handleComplete processes completion
func (m *ManagerV2) handleComplete(msg string, spinnerInstance *spinner.Spinner) {
	duration := time.Since(m.startTime)
	finalMsg := fmt.Sprintf("%s (completed in %s)", msg, duration.Round(time.Second))

	if m.progressToken != nil && m.server != nil {
		params := map[string]interface{}{
			"progressToken": m.progressToken,
			"progress":      float64(m.total),
			"total":         float64(m.total),
			"message":       finalMsg,
		}

		if err := m.server.SendNotificationToClient(m.ctx, "notifications/progress", params); err != nil {
			m.logger.Warn("Failed to send complete notification", "error", err)
		}
	} else if m.isCLI {
		if m.isCI {
			fmt.Printf("[COMPLETE] %s\n", finalMsg)
		} else {
			if spinnerInstance != nil {
				spinnerInstance.Stop()
			}
			fmt.Printf("✅ %s\n", finalMsg)
		}
	}

	m.logger.Info("Progress completed",
		"duration", duration,
		"message", msg,
		"trace_id", m.traceID)
}

// sendHeartbeat sends a heartbeat update
func (m *ManagerV2) sendHeartbeat(spinnerInstance *spinner.Spinner) {
	current := m.GetCurrent()
	msg := fmt.Sprintf("Still working on step %d/%d...", current, m.total)

	if m.progressToken != nil && m.server != nil {
		params := map[string]interface{}{
			"progressToken": m.progressToken,
			"progress":      float64(current),
			"total":         float64(m.total),
			"message":       msg,
		}

		if err := m.server.SendNotificationToClient(m.ctx, "notifications/progress", params); err != nil {
			m.logger.Warn("Failed to send heartbeat", "error", err)
		}
	} else if m.isCLI && !m.isCI && spinnerInstance != nil {
		spinnerInstance.Suffix = fmt.Sprintf(" %s", msg)
	}

	// Update last update time
	m.stateMu.Lock()
	m.currentState.lastUpdate = time.Now()
	m.stateMu.Unlock()
}

// Thread-safe getters

func (m *ManagerV2) GetCurrent() int {
	m.stateMu.RLock()
	defer m.stateMu.RUnlock()
	return m.currentState.current
}

func (m *ManagerV2) SetCurrent(current int) {
	m.stateMu.Lock()
	defer m.stateMu.Unlock()
	m.currentState.current = current
}

func (m *ManagerV2) getLastUpdate() time.Time {
	m.stateMu.RLock()
	defer m.stateMu.RUnlock()
	return m.currentState.lastUpdate
}

func (m *ManagerV2) GetTotal() int {
	return m.total
}

func (m *ManagerV2) IsComplete() bool {
	return m.GetCurrent() >= m.total
}

func (m *ManagerV2) GetTraceID() string {
	return m.traceID
}

// Error budget methods (already thread-safe)

func (m *ManagerV2) RecordError(err error) bool {
	return m.errorBudget.RecordError(err)
}

func (m *ManagerV2) RecordSuccess() {
	m.errorBudget.RecordSuccess()
}

func (m *ManagerV2) IsCircuitOpen() bool {
	return m.errorBudget.IsCircuitOpen()
}

func (m *ManagerV2) GetErrorBudgetStatus() ErrorBudgetStatus {
	return m.errorBudget.GetStatus()
}

func (m *ManagerV2) UpdateWithErrorHandling(step int, msg string, metadata map[string]interface{}, err error) bool {
	if err != nil {
		if !m.RecordError(err) {
			metadata["error_budget_exceeded"] = true
			metadata["circuit_open"] = true
		}
		metadata["error"] = err.Error()
		metadata["status"] = "failed"
	} else {
		m.RecordSuccess()
		metadata["status"] = "completed"
	}

	m.Update(step, msg, metadata)
	return err == nil && !m.IsCircuitOpen()
}

// Helper methods

func (m *ManagerV2) calculateETA() time.Duration {
	current := m.GetCurrent()
	if current == 0 || current >= m.total {
		return 0
	}

	elapsed := time.Since(m.startTime)
	avgStepDuration := elapsed / time.Duration(current)
	stepsRemaining := m.total - current
	return avgStepDuration * time.Duration(stepsRemaining)
}

func (m *ManagerV2) renderProgressBar(percentage int) string {
	const barWidth = 20
	filled := (percentage * barWidth) / 100
	empty := barWidth - filled

	return fmt.Sprintf("[%s%s]",
		repeatChar('█', filled),
		repeatChar('░', empty))
}
