// Package events provides built-in event handlers for Container Kit MCP.
package events

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/infrastructure/util"
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

// WorkflowMetrics tracks workflow-level metrics
type WorkflowMetrics struct {
	// Step metrics
	stepsCompleted int64
	stepsFailed    int64

	// Workflow metrics
	workflowsCompleted int64
	workflowsFailed    int64

	// Security scan metrics
	securityScansCompleted  int64
	totalVulnerabilities    int64
	criticalVulnerabilities int64
}

// GetMetrics returns the current workflow metrics
func (wm *WorkflowMetrics) GetMetrics() map[string]int64 {
	return map[string]int64{
		"steps_completed":          atomic.LoadInt64(&wm.stepsCompleted),
		"steps_failed":             atomic.LoadInt64(&wm.stepsFailed),
		"workflows_completed":      atomic.LoadInt64(&wm.workflowsCompleted),
		"workflows_failed":         atomic.LoadInt64(&wm.workflowsFailed),
		"security_scans_completed": atomic.LoadInt64(&wm.securityScansCompleted),
		"total_vulnerabilities":    atomic.LoadInt64(&wm.totalVulnerabilities),
		"critical_vulnerabilities": atomic.LoadInt64(&wm.criticalVulnerabilities),
	}
}

// Global workflow metrics instance
var globalWorkflowMetrics = &WorkflowMetrics{}

// GetGlobalWorkflowMetrics returns the global workflow metrics
func GetGlobalWorkflowMetrics() *WorkflowMetrics {
	return globalWorkflowMetrics
}

// MetricsEventHandler collects metrics from workflow events
type MetricsEventHandler struct {
	logger          *slog.Logger
	workflowMetrics *WorkflowMetrics
}

// NewMetricsEventHandler creates a handler that collects metrics from events
func NewMetricsEventHandler(logger *slog.Logger) *MetricsEventHandler {
	return &MetricsEventHandler{
		logger:          logger.With("component", "metrics_event_handler"),
		workflowMetrics: GetGlobalWorkflowMetrics(),
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

		// Integration with metrics collection system
		if e.Success {
			atomic.AddInt64(&h.workflowMetrics.stepsCompleted, 1)
		} else {
			atomic.AddInt64(&h.workflowMetrics.stepsFailed, 1)
		}

		// Log step metrics for observability systems to pick up
		h.logger.Info("workflow_step_completed",
			"step", e.StepName,
			"success", e.Success,
			"duration_seconds", e.Duration.Seconds(),
			"metric_type", "counter")

	case WorkflowCompletedEvent:
		// Track workflow completion metrics
		h.logger.Info("Workflow metrics",
			"workflow_id", e.WorkflowID(),
			"duration_ms", e.TotalDuration.Milliseconds(),
			"success", e.Success)

		// Integration with metrics collection
		if e.Success {
			atomic.AddInt64(&h.workflowMetrics.workflowsCompleted, 1)
		} else {
			atomic.AddInt64(&h.workflowMetrics.workflowsFailed, 1)
		}

		// Log workflow metrics for observability systems to pick up
		h.logger.Info("workflow_completed",
			"success", e.Success,
			"duration_seconds", e.TotalDuration.Seconds(),
			"metric_type", "histogram")

	case SecurityScanEvent:
		// Track security scan metrics
		h.logger.Info("Security scan metrics",
			"workflow_id", e.WorkflowID(),
			"scanner", e.Scanner,
			"vulnerabilities", e.VulnCount,
			"critical", e.CriticalCount,
			"duration_ms", e.ScanDuration.Milliseconds())

		// Integration with metrics collection
		atomic.AddInt64(&h.workflowMetrics.securityScansCompleted, 1)
		atomic.AddInt64(&h.workflowMetrics.totalVulnerabilities, int64(e.VulnCount))
		atomic.AddInt64(&h.workflowMetrics.criticalVulnerabilities, int64(e.CriticalCount))

		// Log security metrics for observability systems to pick up
		h.logger.Info("security_scan_completed",
			"scanner", e.Scanner,
			"duration_seconds", e.ScanDuration.Seconds(),
			"metric_type", "histogram")

		h.logger.Info("security_vulnerabilities",
			"severity", "critical",
			"scanner", e.Scanner,
			"count", e.CriticalCount,
			"metric_type", "gauge")

		h.logger.Info("security_vulnerabilities",
			"severity", "total",
			"scanner", e.Scanner,
			"count", e.VulnCount,
			"metric_type", "gauge")
	}

	return nil
}

// EventUtils provides utility functions for creating events
type EventUtils struct{}

// GenerateEventID creates a unique event ID
func (EventUtils) GenerateEventID() string {
	return time.Now().Format("20060102150405") + "-" + util.ShortID()
}
