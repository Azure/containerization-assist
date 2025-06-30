package observability

import (
	"fmt"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/internal/session"
	"github.com/rs/zerolog"
)

// ProgressUpdate represents a single progress update
type ProgressUpdate struct {
	OperationID string                 `json:"operation_id"`
	Progress    float64                `json:"progress"`
	Message     string                 `json:"message"`
	Timestamp   time.Time              `json:"timestamp"`
	Stage       string                 `json:"stage,omitempty"`
	Data        map[string]interface{} `json:"data,omitempty"`
}

// ProgressTracker interface for tracking operation progress
type ProgressTracker interface {
	Start(operationID string) ProgressCallback
	Update(operationID string, progress float64, message string)
	UpdateWithStage(operationID string, progress float64, message, stage string)
	UpdateWithData(operationID string, progress float64, message string, data map[string]interface{})
	Complete(operationID string, result interface{}, err error)
	GetProgress(operationID string) (*ProgressState, error)
	ListActiveOperations() []string
}

// ProgressCallback is a function type for reporting progress
type ProgressCallback func(progress float64, message string)

// ProgressState represents the current state of an operation
type ProgressState struct {
	OperationID string           `json:"operation_id"`
	Progress    float64          `json:"progress"`
	Message     string           `json:"message"`
	Stage       string           `json:"stage,omitempty"`
	StartTime   time.Time        `json:"start_time"`
	LastUpdate  time.Time        `json:"last_update"`
	IsComplete  bool             `json:"is_complete"`
	Error       error            `json:"error,omitempty"`
	Result      interface{}      `json:"result,omitempty"`
	Updates     []ProgressUpdate `json:"updates"`
	SessionID   string           `json:"session_id,omitempty"`
	ToolName    string           `json:"tool_name,omitempty"`
}

// ComprehensiveProgressTracker implements ProgressTracker with full tracking capabilities
type ComprehensiveProgressTracker struct {
	operations    map[string]*ProgressState
	mutex         sync.RWMutex
	logger        zerolog.Logger
	sessionMgr    *session.SessionManager
	maxUpdates    int // Maximum number of updates to store per operation
	cleanupTicker *time.Ticker
	done          chan bool
}

// NewComprehensiveProgressTracker creates a new comprehensive progress tracker
func NewComprehensiveProgressTracker(logger zerolog.Logger, sessionMgr *session.SessionManager) *ComprehensiveProgressTracker {
	tracker := &ComprehensiveProgressTracker{
		operations: make(map[string]*ProgressState),
		logger:     logger.With().Str("component", "progress_tracker").Logger(),
		sessionMgr: sessionMgr,
		maxUpdates: 100, // Store last 100 updates per operation
		done:       make(chan bool),
	}

	// Start cleanup routine
	tracker.startCleanupRoutine()

	return tracker
}

// Start implements ProgressTracker.Start
func (pt *ComprehensiveProgressTracker) Start(operationID string) ProgressCallback {
	pt.mutex.Lock()
	defer pt.mutex.Unlock()

	state := &ProgressState{
		OperationID: operationID,
		Progress:    0.0,
		Message:     "Starting operation",
		StartTime:   time.Now(),
		LastUpdate:  time.Now(),
		IsComplete:  false,
		Updates:     make([]ProgressUpdate, 0, pt.maxUpdates),
	}

	pt.operations[operationID] = state

	pt.logger.Debug().
		Str("operation_id", operationID).
		Msg("Started progress tracking for operation")

	// Return callback function
	return func(progress float64, message string) {
		pt.Update(operationID, progress, message)
	}
}

// Update implements ProgressTracker.Update
func (pt *ComprehensiveProgressTracker) Update(operationID string, progress float64, message string) {
	pt.UpdateWithStage(operationID, progress, message, "")
}

// UpdateWithStage implements ProgressTracker.UpdateWithStage
func (pt *ComprehensiveProgressTracker) UpdateWithStage(operationID string, progress float64, message, stage string) {
	pt.UpdateWithData(operationID, progress, message, map[string]interface{}{
		"stage": stage,
	})
}

// UpdateWithData implements ProgressTracker.UpdateWithData
func (pt *ComprehensiveProgressTracker) UpdateWithData(operationID string, progress float64, message string, data map[string]interface{}) {
	pt.mutex.Lock()
	defer pt.mutex.Unlock()

	state, exists := pt.operations[operationID]
	if !exists {
		pt.logger.Warn().
			Str("operation_id", operationID).
			Msg("Attempted to update non-existent operation")
		return
	}

	// Ensure progress is within bounds
	if progress < 0 {
		progress = 0
	} else if progress > 100 {
		progress = 100
	}

	// Update state
	state.Progress = progress
	state.Message = message
	state.LastUpdate = time.Now()

	if stage, ok := data["stage"].(string); ok && stage != "" {
		state.Stage = stage
	}

	// Create progress update
	update := ProgressUpdate{
		OperationID: operationID,
		Progress:    progress,
		Message:     message,
		Timestamp:   state.LastUpdate,
		Stage:       state.Stage,
		Data:        data,
	}

	// Add update to history (with rotation if needed)
	state.Updates = append(state.Updates, update)
	if len(state.Updates) > pt.maxUpdates {
		// Remove oldest updates to maintain limit
		copy(state.Updates, state.Updates[1:])
		state.Updates = state.Updates[:pt.maxUpdates]
	}

	pt.logger.Debug().
		Str("operation_id", operationID).
		Float64("progress", progress).
		Str("message", message).
		Str("stage", state.Stage).
		Msg("Progress update")

	// Update session if session manager is available
	if pt.sessionMgr != nil && state.SessionID != "" {
		pt.updateSessionProgress(state)
	}
}

// Complete implements ProgressTracker.Complete
func (pt *ComprehensiveProgressTracker) Complete(operationID string, result interface{}, err error) {
	pt.mutex.Lock()
	defer pt.mutex.Unlock()

	state, exists := pt.operations[operationID]
	if !exists {
		pt.logger.Warn().
			Str("operation_id", operationID).
			Msg("Attempted to complete non-existent operation")
		return
	}

	state.IsComplete = true
	state.LastUpdate = time.Now()
	state.Result = result
	state.Error = err

	if err != nil {
		state.Message = "Operation failed: " + err.Error()
		state.Progress = 0 // Reset progress on error
		pt.logger.Error().
			Str("operation_id", operationID).
			Err(err).
			Msg("Operation completed with error")
	} else {
		state.Message = "Operation completed successfully"
		state.Progress = 100
		pt.logger.Info().
			Str("operation_id", operationID).
			Msg("Operation completed successfully")
	}

	// Add completion update
	update := ProgressUpdate{
		OperationID: operationID,
		Progress:    state.Progress,
		Message:     state.Message,
		Timestamp:   state.LastUpdate,
		Stage:       "completed",
		Data: map[string]interface{}{
			"completed": true,
			"success":   err == nil,
		},
	}

	state.Updates = append(state.Updates, update)
	if len(state.Updates) > pt.maxUpdates {
		copy(state.Updates, state.Updates[1:])
		state.Updates = state.Updates[:pt.maxUpdates]
	}

	// Update session if session manager is available
	if pt.sessionMgr != nil && state.SessionID != "" {
		pt.updateSessionProgress(state)
	}
}

// GetProgress implements ProgressTracker.GetProgress
func (pt *ComprehensiveProgressTracker) GetProgress(operationID string) (*ProgressState, error) {
	pt.mutex.RLock()
	defer pt.mutex.RUnlock()

	state, exists := pt.operations[operationID]
	if !exists {
		return nil, fmt.Errorf("operation not found: %s", operationID)
	}

	// Return a copy to avoid concurrent access issues
	stateCopy := *state
	stateCopy.Updates = make([]ProgressUpdate, len(state.Updates))
	copy(stateCopy.Updates, state.Updates)

	return &stateCopy, nil
}

// ListActiveOperations implements ProgressTracker.ListActiveOperations
func (pt *ComprehensiveProgressTracker) ListActiveOperations() []string {
	pt.mutex.RLock()
	defer pt.mutex.RUnlock()

	var active []string
	for operationID, state := range pt.operations {
		if !state.IsComplete {
			active = append(active, operationID)
		}
	}

	return active
}

// SetSessionInfo associates an operation with a session and tool
func (pt *ComprehensiveProgressTracker) SetSessionInfo(operationID, sessionID, toolName string) {
	pt.mutex.Lock()
	defer pt.mutex.Unlock()

	if state, exists := pt.operations[operationID]; exists {
		state.SessionID = sessionID
		state.ToolName = toolName
	}
}

// GetOperationsBySession returns all operations for a specific session
func (pt *ComprehensiveProgressTracker) GetOperationsBySession(sessionID string) []*ProgressState {
	pt.mutex.RLock()
	defer pt.mutex.RUnlock()

	var operations []*ProgressState
	for _, state := range pt.operations {
		if state.SessionID == sessionID {
			// Return a copy
			stateCopy := *state
			stateCopy.Updates = make([]ProgressUpdate, len(state.Updates))
			copy(stateCopy.Updates, state.Updates)
			operations = append(operations, &stateCopy)
		}
	}

	return operations
}

// Stop gracefully stops the progress tracker
func (pt *ComprehensiveProgressTracker) Stop() {
	if pt.cleanupTicker != nil {
		pt.cleanupTicker.Stop()
		close(pt.done)
	}

	pt.logger.Info().Msg("Progress tracker stopped")
}

// Private methods

func (pt *ComprehensiveProgressTracker) updateSessionProgress(state *ProgressState) {
	// This would integrate with the session manager to update session state
	// For now, we'll just log the progress
	pt.logger.Debug().
		Str("session_id", state.SessionID).
		Str("tool", state.ToolName).
		Str("operation_id", state.OperationID).
		Float64("progress", state.Progress).
		Msg("Updated session progress")
}

func (pt *ComprehensiveProgressTracker) startCleanupRoutine() {
	pt.cleanupTicker = time.NewTicker(1 * time.Hour)

	go func() {
		for {
			select {
			case <-pt.cleanupTicker.C:
				pt.cleanupCompletedOperations()
			case <-pt.done:
				return
			}
		}
	}()
}

func (pt *ComprehensiveProgressTracker) cleanupCompletedOperations() {
	pt.mutex.Lock()
	defer pt.mutex.Unlock()

	cutoff := time.Now().Add(-24 * time.Hour) // Keep completed operations for 24 hours
	cleaned := 0

	for operationID, state := range pt.operations {
		if state.IsComplete && state.LastUpdate.Before(cutoff) {
			delete(pt.operations, operationID)
			cleaned++
		}
	}

	if cleaned > 0 {
		pt.logger.Info().
			Int("cleaned_operations", cleaned).
			Msg("Cleaned up completed operations")
	}
}
