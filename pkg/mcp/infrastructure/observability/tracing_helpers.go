package observability

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// SpanHelper provides convenient methods for working with spans
type SpanHelper struct {
	span trace.Span
}

// NewSpanHelper creates a new span helper
func NewSpanHelper(span trace.Span) *SpanHelper {
	return &SpanHelper{span: span}
}

// SetAttributes sets multiple attributes on the span
func (h *SpanHelper) SetAttributes(attrs ...attribute.KeyValue) {
	h.span.SetAttributes(attrs...)
}

// SetStatus sets the span status
func (h *SpanHelper) SetStatus(code codes.Code, description string) {
	h.span.SetStatus(code, description)
}

// RecordError records an error on the span
func (h *SpanHelper) RecordError(err error, opts ...trace.EventOption) {
	h.span.RecordError(err, opts...)
}

// End ends the span
func (h *SpanHelper) End() {
	h.span.End()
}

// AddEvent adds an event to the span
func (h *SpanHelper) AddEvent(name string, attrs ...attribute.KeyValue) {
	h.span.AddEvent(name, trace.WithAttributes(attrs...))
}

// Common attribute keys for MCP tracing
const (
	// Sampling attributes
	AttrSamplingTemplateID      = "sampling.template_id"
	AttrSamplingTokensUsed      = "sampling.tokens_used"
	AttrSamplingPromptTokens    = "sampling.prompt_tokens"
	AttrSamplingResponseTokens  = "sampling.response_tokens"
	AttrSamplingContentType     = "sampling.content_type"
	AttrSamplingContentSize     = "sampling.content_size"
	AttrSamplingRetryAttempt    = "sampling.retry_attempt"
	AttrSamplingValidationValid = "sampling.validation_valid"
	AttrSamplingSecurityIssues  = "sampling.security_issues"

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

// TraceSamplingRequest creates a traced sampling request
func TraceSamplingRequest(ctx context.Context, templateID string, fn func(context.Context) error) error {
	ctx, span := StartSpan(ctx, "sampling.request",
		trace.WithAttributes(
			attribute.String(AttrComponent, "sampling"),
			attribute.String(AttrSamplingTemplateID, templateID),
		),
	)
	defer span.End()

	helper := NewSpanHelper(span)
	startTime := time.Now()

	err := fn(ctx)

	// Record duration
	duration := time.Since(startTime)
	helper.SetAttributes(attribute.Int64(AttrDuration, duration.Milliseconds()))

	if err != nil {
		helper.SetStatus(codes.Error, "sampling request failed")
		helper.RecordError(err)
		return err
	}

	helper.SetStatus(codes.Ok, "sampling request completed")
	return nil
}

// TraceSamplingValidation creates a traced validation operation
func TraceSamplingValidation(ctx context.Context, contentType string, fn func(context.Context) (bool, error)) (bool, error) {
	ctx, span := StartSpan(ctx, "sampling.validation",
		trace.WithAttributes(
			attribute.String(AttrComponent, "sampling"),
			attribute.String(AttrSamplingContentType, contentType),
		),
	)
	defer span.End()

	helper := NewSpanHelper(span)
	startTime := time.Now()

	valid, err := fn(ctx)

	// Record results
	duration := time.Since(startTime)
	helper.SetAttributes(
		attribute.Bool(AttrSamplingValidationValid, valid),
		attribute.Int64(AttrDuration, duration.Milliseconds()),
	)

	if err != nil {
		helper.SetStatus(codes.Error, "validation failed")
		helper.RecordError(err)
		return valid, err
	}

	helper.SetStatus(codes.Ok, "validation completed")
	return valid, nil
}

// TraceProgressUpdate creates a traced progress update
func TraceProgressUpdate(ctx context.Context, workflowID, stepName string, stepNumber, totalSteps int, fn func(context.Context) error) error {
	ctx, span := StartSpan(ctx, "progress.update",
		trace.WithAttributes(
			attribute.String(AttrComponent, "progress"),
			attribute.String(AttrProgressWorkflowID, workflowID),
			attribute.String(AttrProgressStepName, stepName),
			attribute.Int(AttrProgressStepNumber, stepNumber),
			attribute.Int(AttrProgressTotalSteps, totalSteps),
		),
	)
	defer span.End()

	helper := NewSpanHelper(span)
	startTime := time.Now()

	// Calculate percentage
	percentage := float64(stepNumber) / float64(totalSteps) * 100
	helper.SetAttributes(attribute.Float64(AttrProgressPercentage, percentage))

	err := fn(ctx)

	// Record duration
	duration := time.Since(startTime)
	helper.SetAttributes(attribute.Int64(AttrDuration, duration.Milliseconds()))

	if err != nil {
		helper.SetStatus(codes.Error, "progress update failed")
		helper.RecordError(err)
		return err
	}

	helper.SetStatus(codes.Ok, "progress update completed")
	return nil
}

// TraceWorkflowStep creates a traced workflow step execution
func TraceWorkflowStep(ctx context.Context, workflowID, stepName string, fn func(context.Context) error) error {
	ctx, span := StartSpan(ctx, fmt.Sprintf("workflow.step.%s", stepName),
		trace.WithAttributes(
			attribute.String(AttrComponent, "workflow"),
			attribute.String(AttrProgressWorkflowID, workflowID),
			attribute.String(AttrProgressStepName, stepName),
		),
	)
	defer span.End()

	helper := NewSpanHelper(span)
	startTime := time.Now()

	helper.AddEvent("step.started")

	err := fn(ctx)

	// Record duration
	duration := time.Since(startTime)
	helper.SetAttributes(attribute.Int64(AttrDuration, duration.Milliseconds()))

	if err != nil {
		helper.SetStatus(codes.Error, fmt.Sprintf("workflow step %s failed", stepName))
		helper.RecordError(err)
		helper.AddEvent("step.failed", attribute.String("error", err.Error()))
		return err
	}

	helper.SetStatus(codes.Ok, fmt.Sprintf("workflow step %s completed", stepName))
	helper.AddEvent("step.completed")
	return nil
}

// WithTraceID injects a trace ID into the context for correlation
func WithTraceID(ctx context.Context, traceID string) context.Context {
	span := SpanFromContext(ctx)
	if span.IsRecording() {
		span.SetAttributes(attribute.String("trace.id", traceID))
	}
	return ctx
}

// ExtractTraceID extracts the current trace ID from context
func ExtractTraceID(ctx context.Context) string {
	span := SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		return span.SpanContext().TraceID().String()
	}
	return ""
}
