// Package observability provides unified monitoring and health infrastructure
// for the MCP components.
package observability

import (
	"context"
)

// SpanHelper provides a no-op implementation for API compatibility
type SpanHelper struct{}

// NewSpanHelper creates a new span helper
func NewSpanHelper(span interface{}) *SpanHelper {
	return &SpanHelper{}
}

// SetAttributes is a no-op method for API compatibility
func (h *SpanHelper) SetAttributes(attrs ...interface{}) {
	// No-op
}

// SetStatus is a no-op method for API compatibility
func (h *SpanHelper) SetStatus(code interface{}, description string) {
	// No-op
}

// RecordError is a no-op method for API compatibility
func (h *SpanHelper) RecordError(err error, opts ...interface{}) {
	// No-op
}

// End is a no-op method for API compatibility
func (h *SpanHelper) End() {
	// No-op
}

// AddEvent is a no-op method for API compatibility
func (h *SpanHelper) AddEvent(name string, attrs ...interface{}) {
	// No-op
}

// Common attribute keys for tracing (kept for reference)
const (
	// Progress attributes
	AttrProgressWorkflowID = "progress.workflow_id"
	AttrProgressStepName   = "progress.step_name"
	AttrProgressStepNumber = "progress.step_number"
	AttrProgressTotalSteps = "progress.total_steps"
	AttrProgressStatus     = "progress.status"
	AttrProgressPercentage = "progress.percentage"
	AttrProgressSessionID  = "progress.session_id"

	// General attributes
	AttrComponent = "component"
	AttrOperation = "operation"
	AttrUserID    = "user.id"
	AttrRequestID = "request.id"
	AttrErrorType = "error.type"
	AttrDuration  = "duration_ms"
)

// TraceSamplingRequest is a no-op function for API compatibility
func TraceSamplingRequest(ctx context.Context, templateID string, fn func(context.Context) error) error {
	return fn(ctx)
}

// TraceSamplingValidation is a no-op function for API compatibility
func TraceSamplingValidation(ctx context.Context, contentType string, fn func(context.Context) (bool, error)) (bool, error) {
	return fn(ctx)
}

// TraceProgressUpdate is a no-op function for API compatibility
func TraceProgressUpdate(ctx context.Context, workflowID, stepName string, stepNumber, totalSteps int, fn func(context.Context) error) error {
	return fn(ctx)
}

// TraceWorkflowStep is a no-op function for API compatibility
func TraceWorkflowStep(ctx context.Context, workflowID, stepName string, fn func(context.Context) error) error {
	return fn(ctx)
}

// WithTraceID is a no-op function for API compatibility
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return ctx
}

// ExtractTraceID is a no-op function for API compatibility
func ExtractTraceID(ctx context.Context) string {
	return ""
}
