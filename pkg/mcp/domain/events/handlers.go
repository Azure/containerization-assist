// Package events provides built-in event handlers for Containerization Assist MCP.
package events

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"time"
)

// ProgressEventHandler updates progress tracking based on workflow events
type ProgressEventHandler struct {
	logger *slog.Logger
}

// NewProgressEventHandler creates a handler that integrates events with progress tracking
func NewProgressEventHandler(logger *slog.Logger) *ProgressEventHandler {
	return &ProgressEventHandler{
		logger: logger.With("component", "progress_event_handler"),
	}
}

// Handle processes workflow events and updates progress tracking
func (h *ProgressEventHandler) Handle(ctx context.Context, event DomainEvent) error {
	switch e := event.(type) {
	case WorkflowStepCompletedEvent:
		h.logger.Info("Step completed",
			"workflow_id", e.WorkflowID(),
			"step_name", e.StepName,
			"progress", e.Progress,
			"success", e.Success,
			"duration", e.Duration)

		// If we had access to a progress tracker here, we could update it
		// For now, we log the progress information

	case WorkflowStartedEvent:
		h.logger.Info("Workflow started",
			"workflow_id", e.WorkflowID(),
			"repo_url", e.RepoURL,
			"branch", e.Branch)

	case WorkflowCompletedEvent:
		h.logger.Info("Workflow completed",
			"workflow_id", e.WorkflowID(),
			"success", e.Success,
			"duration", e.TotalDuration,
			"image_ref", e.ImageRef,
			"endpoint", e.Endpoint)

	case SecurityScanEvent:
		h.logger.Info("Security scan completed",
			"workflow_id", e.WorkflowID(),
			"image_ref", e.ImageRef,
			"scanner", e.Scanner,
			"vulnerabilities", e.VulnCount,
			"critical", e.CriticalCount,
			"duration", e.ScanDuration)

	default:
		h.logger.Debug("Unknown event type", "event_type", event.EventType())
	}

	return nil
}

// EventUtils provides utility functions for creating events
type EventUtils struct{}

// GenerateEventID creates a unique event ID
func (EventUtils) GenerateEventID() string {
	return time.Now().Format("20060102150405") + "-" + generateShortID()
}

// generateShortID creates a short random ID for internal use
func generateShortID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based ID if crypto/rand fails
		return hex.EncodeToString([]byte{byte(time.Now().UnixNano() & 0xFF)})
	}
	return hex.EncodeToString(b)
}
