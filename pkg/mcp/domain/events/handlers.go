// Package events provides built-in event handlers for Container Kit MCP.
package events

import (
	"context"
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

// MetricsEventHandler collects metrics from workflow events
type MetricsEventHandler struct {
	logger *slog.Logger
}

// NewMetricsEventHandler creates a handler that collects metrics from events
func NewMetricsEventHandler(logger *slog.Logger) *MetricsEventHandler {
	return &MetricsEventHandler{
		logger: logger.With("component", "metrics_event_handler"),
	}
}

// Handle processes events and extracts metrics
func (h *MetricsEventHandler) Handle(ctx context.Context, event DomainEvent) error {
	switch e := event.(type) {
	case WorkflowStepCompletedEvent:
		// Track step completion metrics
		h.logger.Info("Step metrics",
			"workflow_id", e.WorkflowID(),
			"step_name", e.StepName,
			"duration_ms", e.Duration.Milliseconds(),
			"success", e.Success)

		// TODO: Integration with metrics collection system
		// metrics.Counter("workflow_steps_completed").
		//   WithTags("step", e.StepName, "success", fmt.Sprintf("%v", e.Success)).
		//   Increment()
		// metrics.Histogram("workflow_step_duration").
		//   WithTags("step", e.StepName).
		//   Observe(e.Duration.Seconds())

	case WorkflowCompletedEvent:
		// Track workflow completion metrics
		h.logger.Info("Workflow metrics",
			"workflow_id", e.WorkflowID(),
			"duration_ms", e.TotalDuration.Milliseconds(),
			"success", e.Success)

		// TODO: Integration with metrics collection
		// metrics.Counter("workflows_completed").
		//   WithTags("success", fmt.Sprintf("%v", e.Success)).
		//   Increment()
		// metrics.Histogram("workflow_duration").
		//   Observe(e.TotalDuration.Seconds())

	case SecurityScanEvent:
		// Track security scan metrics
		h.logger.Info("Security scan metrics",
			"workflow_id", e.WorkflowID(),
			"scanner", e.Scanner,
			"vulnerabilities", e.VulnCount,
			"critical", e.CriticalCount,
			"duration_ms", e.ScanDuration.Milliseconds())

		// TODO: Integration with metrics collection
		// metrics.Histogram("security_scan_duration").
		//   WithTags("scanner", e.Scanner).
		//   Observe(e.ScanDuration.Seconds())
		// metrics.Gauge("security_vulnerabilities").
		//   WithTags("severity", "critical", "scanner", e.Scanner).
		//   Set(float64(e.CriticalCount))
	}

	return nil
}

// EventUtils provides utility functions for creating events
type EventUtils struct{}

// GenerateEventID creates a unique event ID
func (EventUtils) GenerateEventID() string {
	return time.Now().Format("20060102150405") + "-" + generateRandomString(6)
}

// generateRandomString creates a random string for event IDs
func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}
