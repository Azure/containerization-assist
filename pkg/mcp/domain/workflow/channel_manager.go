// Package workflow provides a channel-based progress manager
package workflow

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/infrastructure/security"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// updateType represents the type of update
type updateType int

const (
	updateTypeBegin updateType = iota
	updateTypeProgress
	updateTypeComplete
	updateTypeHeartbeat
	updateTypeQuery
)

// update represents a progress update to be processed
type update struct {
	Type     updateType
	Step     int
	Message  string
	Metadata map[string]interface{}
}

// ChannelManager implements lock-free progress management using channels
type ChannelManager struct {
	// Immutable configuration
	ctx         context.Context
	logger      *slog.Logger
	reporter    Reporter
	total       int
	traceID     string
	errorBudget *ErrorBudget

	// Channels for communication
	updateCh chan update
	done     chan struct{}
	wg       sync.WaitGroup

	// Configuration
	minUpdateInterval time.Duration
	heartbeatInterval time.Duration

	// Atomic current value for thread-safe access
	currentAtomic int64

	// State (only accessed in renderer goroutine)
	state *managerState
}

// managerState holds mutable state (only accessed by renderer)
type managerState struct {
	current       int
	lastUpdate    time.Time
	startTime     time.Time
	stepDurations map[string]time.Duration
}

// NewChannelManager creates a new channel-based progress manager
func NewChannelManager(ctx context.Context, req *mcp.CallToolRequest, totalSteps int, logger *slog.Logger) *ChannelManager {
	traceID := generateTraceID()
	logger = logger.With("trace_id", traceID)

	// Create appropriate reporter
	var reporter Reporter
	srv := server.ServerFromContext(ctx)

	if srv != nil && req != nil && req.Params.Meta != nil && req.Params.Meta.ProgressToken != nil {
		// MCP mode
		wrapper := &mcpServerWrapper{server: srv}
		reporter = NewMCPReporter(ctx, wrapper, req.Params.Meta.ProgressToken, totalSteps, logger)
		logger.Debug("Using MCP progress reporter")
	} else {
		// CLI mode
		reporter = NewCLIReporter(ctx, totalSteps, logger)
		logger.Debug("Using CLI progress reporter")
	}

	m := &ChannelManager{
		ctx:               ctx,
		logger:            logger,
		reporter:          reporter,
		total:             totalSteps,
		traceID:           traceID,
		errorBudget:       NewErrorBudget(5, 10*time.Minute),
		updateCh:          make(chan update, 100), // Buffered to prevent blocking
		done:              make(chan struct{}),
		minUpdateInterval: 100 * time.Millisecond,
		heartbeatInterval: 15 * time.Second,
		currentAtomic:     0, // Initialize atomic value
		state: &managerState{
			startTime:     time.Now(),
			lastUpdate:    time.Now(),
			stepDurations: make(map[string]time.Duration),
		},
	}

	// Start renderer goroutine
	m.wg.Add(1)
	go m.renderer()

	return m
}

// renderer processes all updates in a single goroutine
func (m *ChannelManager) renderer() {
	defer m.wg.Done()
	defer m.reporter.Close()

	heartbeatTicker := time.NewTicker(m.heartbeatInterval)
	defer heartbeatTicker.Stop()

	var pendingUpdate *update
	var throttleTimer *time.Timer

	for {
		select {
		case upd := <-m.updateCh:
			// Cancel any pending throttled update
			if throttleTimer != nil {
				throttleTimer.Stop()
				throttleTimer = nil
				pendingUpdate = nil
			}

			switch upd.Type {
			case updateTypeBegin:
				m.handleBegin(upd)

			case updateTypeProgress:
				// Atomic value is already updated in Update() method

				// Apply throttling for actual progress reporting
				if m.shouldThrottle(upd) {
					pendingUpdate = &upd
					delay := m.minUpdateInterval - time.Since(m.state.lastUpdate)
					throttleTimer = time.AfterFunc(delay, func() {
						select {
						case m.updateCh <- *pendingUpdate:
						case <-m.done:
						}
					})
				} else {
					m.handleProgress(upd)
				}

			case updateTypeComplete:
				m.handleComplete(upd)
				return // Exit renderer

			case updateTypeHeartbeat:
				m.handleHeartbeat()

			case updateTypeQuery:
				m.handleQuery(upd)
			}

		case <-heartbeatTicker.C:
			// Send heartbeat if no recent updates
			if time.Since(m.state.lastUpdate) >= m.heartbeatInterval && m.state.current < m.total {
				select {
				case m.updateCh <- update{Type: updateTypeHeartbeat}:
				default:
					// Don't block on heartbeat
				}
			}

		case <-m.done:
			return
		}
	}
}

// Public API methods (non-blocking)

// Begin starts progress tracking
func (m *ChannelManager) Begin(msg string) {
	select {
	case m.updateCh <- update{Type: updateTypeBegin, Message: msg}:
	case <-m.done:
	}
}

// Update advances progress
func (m *ChannelManager) Update(step int, msg string, metadata map[string]interface{}) {
	// Update atomic value immediately for thread-safe access
	atomic.StoreInt64(&m.currentAtomic, int64(step))

	select {
	case m.updateCh <- update{
		Type:     updateTypeProgress,
		Step:     step,
		Message:  msg,
		Metadata: metadata,
	}:
	case <-m.done:
	}
}

// Complete finishes progress tracking
func (m *ChannelManager) Complete(msg string) {
	select {
	case m.updateCh <- update{Type: updateTypeComplete, Message: msg}:
		// Wait for renderer to finish
		m.wg.Wait()
	case <-m.done:
	}
}

// Finish cleans up resources
func (m *ChannelManager) Finish() {
	close(m.done)
	m.wg.Wait()
}

// Handler methods (called only by renderer goroutine)

func (m *ChannelManager) handleBegin(upd update) {
	if err := m.reporter.Begin(upd.Message); err != nil {
		m.logger.Warn("Failed to send begin notification", "error", err)
	}
	m.state.lastUpdate = time.Now()
}

func (m *ChannelManager) handleProgress(upd update) {
	// Update state
	oldCurrent := m.state.current
	m.state.current = upd.Step
	m.state.lastUpdate = time.Now()

	// Atomic value is already updated in the renderer main loop
	// No need to update it again here

	// Track step duration
	if upd.Step > oldCurrent && upd.Metadata != nil {
		if stepName, ok := upd.Metadata["step_name"].(string); ok {
			m.state.stepDurations[stepName] = time.Since(m.state.lastUpdate)
		}
	}

	// Enrich metadata
	enrichedMetadata := m.enrichMetadata(upd.Metadata)

	// Send update
	if err := m.reporter.Update(upd.Step, m.total, upd.Message); err != nil {
		m.logger.Warn("Failed to send progress notification", "error", err)
	}

	// Log progress
	m.logProgress(upd.Step, upd.Message, enrichedMetadata)
}

func (m *ChannelManager) handleComplete(upd update) {
	if err := m.reporter.Complete(upd.Message); err != nil {
		m.logger.Warn("Failed to send complete notification", "error", err)
	}

	m.logger.Info("Progress completed",
		"duration", time.Since(m.state.startTime),
		"message", upd.Message,
		"trace_id", m.traceID)
}

func (m *ChannelManager) handleHeartbeat() {
	msg := fmt.Sprintf("Still working on step %d/%d...", m.state.current, m.total)
	if err := m.reporter.Update(m.state.current, m.total, msg); err != nil {
		m.logger.Warn("Failed to send heartbeat", "error", err)
	}
	m.state.lastUpdate = time.Now()
}

func (m *ChannelManager) handleQuery(upd update) {
	if upd.Metadata == nil {
		m.logger.Debug("Query received but no metadata")
		return
	}

	query, ok := upd.Metadata["query"].(string)
	if !ok {
		m.logger.Debug("Query metadata missing or invalid")
		return
	}

	m.logger.Debug("Handling query", "query", query, "current", m.state.current)

	switch query {
	case "current":
		if respCh, ok := upd.Metadata["response"].(chan int); ok {
			select {
			case respCh <- m.state.current:
				m.logger.Debug("Sent current value", "value", m.state.current)
			default:
				m.logger.Debug("Failed to send current value - receiver gone")
			}
		} else {
			m.logger.Debug("Response channel not found or invalid type")
		}
	default:
		m.logger.Debug("Unknown query type", "query", query)
	}
}

// Helper methods

func (m *ChannelManager) shouldThrottle(upd update) bool {
	// Don't throttle the final update
	if upd.Step >= m.total {
		return false
	}

	// Check time since last update
	return time.Since(m.state.lastUpdate) < m.minUpdateInterval
}

func (m *ChannelManager) enrichMetadata(metadata map[string]interface{}) map[string]interface{} {
	if metadata == nil {
		metadata = make(map[string]interface{})
	}

	// Create a copy to avoid mutation
	enriched := make(map[string]interface{}, len(metadata)+10)
	for k, v := range metadata {
		enriched[k] = v
	}

	// Add standard fields
	enriched["kind"] = "progress"
	enriched["step"] = m.state.current
	enriched["total"] = m.total
	enriched["percentage"] = int((float64(m.state.current) / float64(m.total)) * 100)
	enriched["elapsed"] = time.Since(m.state.startTime).String()
	enriched["trace_id"] = m.traceID

	// Calculate ETA
	if eta := m.calculateETA(); eta > 0 {
		enriched["eta_ms"] = int(eta.Milliseconds())
		enriched["eta_human"] = eta.Round(time.Second).String()
	}

	// Map status to code
	if status, ok := enriched["status"].(string); ok {
		enriched["status_code"] = mapStatusToCode(status)
	}

	return enriched
}

func (m *ChannelManager) calculateETA() time.Duration {
	if m.state.current == 0 || m.state.current >= m.total {
		return 0
	}

	elapsed := time.Since(m.state.startTime)
	avgStepDuration := elapsed / time.Duration(m.state.current)
	stepsRemaining := m.total - m.state.current

	return avgStepDuration * time.Duration(stepsRemaining)
}

func (m *ChannelManager) logProgress(step int, message string, metadata map[string]interface{}) {
	maskedMetadata := security.MaskMap(metadata)
	m.logger.Debug("Progress update",
		"step", step,
		"total", m.total,
		"percentage", metadata["percentage"],
		"message", security.Mask(message),
		"metadata", maskedMetadata)
}

// Thread-safe getters (can be called from any goroutine)

// GetCurrent returns the current step (thread-safe)
func (m *ChannelManager) GetCurrent() int {
	return int(atomic.LoadInt64(&m.currentAtomic))
}

// SetCurrent sets the current step (thread-safe)
func (m *ChannelManager) SetCurrent(current int) {
	m.Update(current, fmt.Sprintf("Step %d/%d", current, m.total), nil)
}

// GetTotal returns total steps
func (m *ChannelManager) GetTotal() int {
	return m.total
}

// IsComplete checks if all steps are done
func (m *ChannelManager) IsComplete() bool {
	return m.GetCurrent() >= m.total
}

// GetTraceID returns the trace ID
func (m *ChannelManager) GetTraceID() string {
	return m.traceID
}

// Error budget methods

func (m *ChannelManager) RecordError(err error) bool {
	within := m.errorBudget.RecordError(err)
	if !within {
		m.logger.Error("Error budget exceeded",
			"error", err,
			"budget_status", m.errorBudget.GetStatus().String(),
			"trace_id", m.traceID)
	}
	return within
}

func (m *ChannelManager) RecordSuccess() {
	m.errorBudget.RecordSuccess()
}

func (m *ChannelManager) IsCircuitOpen() bool {
	return m.errorBudget.IsCircuitOpen()
}

func (m *ChannelManager) GetErrorBudgetStatus() ErrorBudgetStatus {
	return m.errorBudget.GetStatus()
}

func (m *ChannelManager) UpdateWithErrorHandling(step int, msg string, metadata map[string]interface{}, err error) bool {
	if metadata == nil {
		metadata = make(map[string]interface{})
	}

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
