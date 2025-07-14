package tracing

import (
	"context"
	"fmt"

	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// TracerAdapter implements the domain workflow.Tracer interface
type TracerAdapter struct{}

// NewTracerAdapter creates a new tracer adapter
func NewTracerAdapter() workflow.Tracer {
	return &TracerAdapter{}
}

// StartSpan creates a new span and returns the updated context and span
func (t *TracerAdapter) StartSpan(ctx context.Context, name string) (context.Context, workflow.Span) {
	ctx, span := StartSpan(ctx, name)
	return ctx, &spanAdapter{
		span: span,
	}
}

// spanAdapter implements the domain workflow.Span interface
type spanAdapter struct {
	span trace.Span
}

// End completes the span
func (s *spanAdapter) End() {
	if s.span != nil {
		s.span.End()
	}
}

// RecordError records an error on the span
func (s *spanAdapter) RecordError(err error) {
	if err != nil && s.span != nil {
		s.span.RecordError(err)
	}
}

// SetAttribute sets a key-value attribute on the span
func (s *spanAdapter) SetAttribute(key string, value interface{}) {
	if s.span == nil {
		return
	}

	// Convert value to appropriate attribute type
	switch v := value.(type) {
	case string:
		s.span.SetAttributes(attribute.String(key, v))
	case int:
		s.span.SetAttributes(attribute.Int(key, v))
	case int64:
		s.span.SetAttributes(attribute.Int64(key, v))
	case float64:
		s.span.SetAttributes(attribute.Float64(key, v))
	case bool:
		s.span.SetAttributes(attribute.Bool(key, v))
	default:
		// For other types, convert to string
		s.span.SetAttributes(attribute.String(key, fmt.Sprintf("%v", v)))
	}
}
