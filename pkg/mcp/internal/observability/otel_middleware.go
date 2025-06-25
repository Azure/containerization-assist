package observability

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// convertAttributesToOTEL converts a map of attributes to OpenTelemetry attributes
func convertAttributesToOTEL(attributes map[string]interface{}) []attribute.KeyValue {
	var attrs []attribute.KeyValue
	if attributes != nil {
		for key, value := range attributes {
			switch v := value.(type) {
			case string:
				attrs = append(attrs, attribute.String(key, v))
			case int:
				attrs = append(attrs, attribute.Int(key, v))
			case int64:
				attrs = append(attrs, attribute.Int64(key, v))
			case float64:
				attrs = append(attrs, attribute.Float64(key, v))
			case bool:
				attrs = append(attrs, attribute.Bool(key, v))
			default:
				attrs = append(attrs, attribute.String(key, fmt.Sprintf("%v", v)))
			}
		}
	}
	return attrs
}

// OTELMiddleware provides OpenTelemetry instrumentation for MCP tools and requests
type OTELMiddleware struct {
	tracer trace.Tracer
	logger zerolog.Logger
}

// NewOTELMiddleware creates a new OpenTelemetry middleware
func NewOTELMiddleware(serviceName string, logger zerolog.Logger) *OTELMiddleware {
	tracer := otel.Tracer(serviceName)
	return &OTELMiddleware{
		tracer: tracer,
		logger: logger,
	}
}

// ToolExecutionSpan represents a span for tool execution
type ToolExecutionSpan struct {
	span     trace.Span
	ctx      context.Context
	toolName string
	logger   zerolog.Logger
}

// StartToolSpan starts a new span for tool execution
func (m *OTELMiddleware) StartToolSpan(ctx context.Context, toolName string, attributes map[string]interface{}) *ToolExecutionSpan {
	spanName := fmt.Sprintf("mcp.tool.%s", toolName)

	ctx, span := m.tracer.Start(ctx, spanName,
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String("mcp.tool.name", toolName),
			attribute.String("mcp.operation.type", "tool_execution"),
		),
	)

	// Add additional attributes if provided
	if attributes != nil {
		var attrs []attribute.KeyValue
		for key, value := range attributes {
			switch v := value.(type) {
			case string:
				attrs = append(attrs, attribute.String(key, v))
			case int:
				attrs = append(attrs, attribute.Int(key, v))
			case int64:
				attrs = append(attrs, attribute.Int64(key, v))
			case float64:
				attrs = append(attrs, attribute.Float64(key, v))
			case bool:
				attrs = append(attrs, attribute.Bool(key, v))
			default:
				attrs = append(attrs, attribute.String(key, fmt.Sprintf("%v", v)))
			}
		}
		span.SetAttributes(attrs...)
	}

	m.logger.Debug().
		Str("tool", toolName).
		Str("span_id", span.SpanContext().SpanID().String()).
		Str("trace_id", span.SpanContext().TraceID().String()).
		Msg("Started tool execution span")

	return &ToolExecutionSpan{
		span:     span,
		ctx:      ctx,
		toolName: toolName,
		logger:   m.logger,
	}
}

// Context returns the context with the span
func (s *ToolExecutionSpan) Context() context.Context {
	return s.ctx
}

// AddEvent adds an event to the span
func (s *ToolExecutionSpan) AddEvent(name string, attributes map[string]interface{}) {
	attrs := convertAttributesToOTEL(attributes)
	s.span.AddEvent(name, trace.WithAttributes(attrs...))

	s.logger.Debug().
		Str("tool", s.toolName).
		Str("event", name).
		Msg("Added span event")
}

// SetAttributes sets additional attributes on the span
func (s *ToolExecutionSpan) SetAttributes(attributes map[string]interface{}) {
	if attributes == nil {
		return
	}

	var attrs []attribute.KeyValue
	for key, value := range attributes {
		switch v := value.(type) {
		case string:
			attrs = append(attrs, attribute.String(key, v))
		case int:
			attrs = append(attrs, attribute.Int(key, v))
		case int64:
			attrs = append(attrs, attribute.Int64(key, v))
		case float64:
			attrs = append(attrs, attribute.Float64(key, v))
		case bool:
			attrs = append(attrs, attribute.Bool(key, v))
		default:
			attrs = append(attrs, attribute.String(key, fmt.Sprintf("%v", v)))
		}
	}

	s.span.SetAttributes(attrs...)
}

// RecordError records an error on the span
func (s *ToolExecutionSpan) RecordError(err error, description string) {
	if err == nil {
		return
	}

	s.span.RecordError(err, trace.WithAttributes(
		attribute.String("error.description", description),
	))
	s.span.SetStatus(codes.Error, description)

	s.logger.Error().
		Err(err).
		Str("tool", s.toolName).
		Str("description", description).
		Msg("Recorded error in span")
}

// Finish completes the span
func (s *ToolExecutionSpan) Finish(success bool, resultSize int) {
	// Set final attributes
	s.span.SetAttributes(
		attribute.Bool("mcp.tool.success", success),
		attribute.Int("mcp.tool.result_size", resultSize),
	)

	// Set status
	if success {
		s.span.SetStatus(codes.Ok, "Tool execution completed successfully")
	}

	s.span.End()

	s.logger.Debug().
		Str("tool", s.toolName).
		Bool("success", success).
		Int("result_size", resultSize).
		Msg("Finished tool execution span")
}

// RequestSpan represents a span for MCP requests
type RequestSpan struct {
	span   trace.Span
	ctx    context.Context
	method string
	logger zerolog.Logger
}

// StartRequestSpan starts a new span for MCP request processing
func (m *OTELMiddleware) StartRequestSpan(ctx context.Context, method string, attributes map[string]interface{}) *RequestSpan {
	spanName := fmt.Sprintf("mcp.request.%s", method)

	ctx, span := m.tracer.Start(ctx, spanName,
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String("mcp.method", method),
			attribute.String("mcp.operation.type", "request_processing"),
		),
	)

	// Add additional attributes if provided
	if attributes != nil {
		var attrs []attribute.KeyValue
		for key, value := range attributes {
			switch v := value.(type) {
			case string:
				attrs = append(attrs, attribute.String(key, v))
			case int:
				attrs = append(attrs, attribute.Int(key, v))
			case int64:
				attrs = append(attrs, attribute.Int64(key, v))
			case float64:
				attrs = append(attrs, attribute.Float64(key, v))
			case bool:
				attrs = append(attrs, attribute.Bool(key, v))
			default:
				attrs = append(attrs, attribute.String(key, fmt.Sprintf("%v", v)))
			}
		}
		span.SetAttributes(attrs...)
	}

	m.logger.Debug().
		Str("method", method).
		Str("span_id", span.SpanContext().SpanID().String()).
		Str("trace_id", span.SpanContext().TraceID().String()).
		Msg("Started request processing span")

	return &RequestSpan{
		span:   span,
		ctx:    ctx,
		method: method,
		logger: m.logger,
	}
}

// Context returns the context with the span
func (r *RequestSpan) Context() context.Context {
	return r.ctx
}

// AddEvent adds an event to the span
func (r *RequestSpan) AddEvent(name string, attributes map[string]interface{}) {
	attrs := convertAttributesToOTEL(attributes)
	r.span.AddEvent(name, trace.WithAttributes(attrs...))

	r.logger.Debug().
		Str("method", r.method).
		Str("event", name).
		Msg("Added span event")
}

// SetAttributes sets additional attributes on the span
func (r *RequestSpan) SetAttributes(attributes map[string]interface{}) {
	if attributes == nil {
		return
	}

	var attrs []attribute.KeyValue
	for key, value := range attributes {
		switch v := value.(type) {
		case string:
			attrs = append(attrs, attribute.String(key, v))
		case int:
			attrs = append(attrs, attribute.Int(key, v))
		case int64:
			attrs = append(attrs, attribute.Int64(key, v))
		case float64:
			attrs = append(attrs, attribute.Float64(key, v))
		case bool:
			attrs = append(attrs, attribute.Bool(key, v))
		default:
			attrs = append(attrs, attribute.String(key, fmt.Sprintf("%v", v)))
		}
	}

	r.span.SetAttributes(attrs...)
}

// RecordError records an error on the span
func (r *RequestSpan) RecordError(err error, description string) {
	if err == nil {
		return
	}

	r.span.RecordError(err, trace.WithAttributes(
		attribute.String("error.description", description),
	))
	r.span.SetStatus(codes.Error, description)

	r.logger.Error().
		Err(err).
		Str("method", r.method).
		Str("description", description).
		Msg("Recorded error in span")
}

// Finish completes the span
func (r *RequestSpan) Finish(statusCode int, responseSize int) {
	// Set final attributes
	r.span.SetAttributes(
		attribute.Int("mcp.response.status_code", statusCode),
		attribute.Int("mcp.response.size", responseSize),
	)

	// Set status based on status code
	if statusCode >= 200 && statusCode < 400 {
		r.span.SetStatus(codes.Ok, "Request processed successfully")
	} else {
		r.span.SetStatus(codes.Error, fmt.Sprintf("Request failed with status %d", statusCode))
	}

	r.span.End()

	r.logger.Debug().
		Str("method", r.method).
		Int("status_code", statusCode).
		Int("response_size", responseSize).
		Msg("Finished request processing span")
}

// ConversationSpan represents a span for conversation stages
type ConversationSpan struct {
	span   trace.Span
	ctx    context.Context
	stage  string
	logger zerolog.Logger
}

// StartConversationSpan starts a new span for conversation stage processing
func (m *OTELMiddleware) StartConversationSpan(ctx context.Context, stage string, sessionID string) *ConversationSpan {
	spanName := fmt.Sprintf("mcp.conversation.%s", stage)

	ctx, span := m.tracer.Start(ctx, spanName,
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("mcp.conversation.stage", stage),
			attribute.String("mcp.session.id", sessionID),
			attribute.String("mcp.operation.type", "conversation_processing"),
		),
	)

	m.logger.Debug().
		Str("stage", stage).
		Str("session_id", sessionID).
		Str("span_id", span.SpanContext().SpanID().String()).
		Str("trace_id", span.SpanContext().TraceID().String()).
		Msg("Started conversation stage span")

	return &ConversationSpan{
		span:   span,
		ctx:    ctx,
		stage:  stage,
		logger: m.logger,
	}
}

// Context returns the context with the span
func (c *ConversationSpan) Context() context.Context {
	return c.ctx
}

// AddEvent adds an event to the span
func (c *ConversationSpan) AddEvent(name string, attributes map[string]interface{}) {
	attrs := convertAttributesToOTEL(attributes)
	c.span.AddEvent(name, trace.WithAttributes(attrs...))

	c.logger.Debug().
		Str("stage", c.stage).
		Str("event", name).
		Msg("Added span event")
}

// Finish completes the span
func (c *ConversationSpan) Finish(success bool, nextStage string) {
	// Set final attributes
	c.span.SetAttributes(
		attribute.Bool("mcp.conversation.success", success),
		attribute.String("mcp.conversation.next_stage", nextStage),
	)

	// Set status
	if success {
		c.span.SetStatus(codes.Ok, "Conversation stage completed successfully")
	}

	c.span.End()

	c.logger.Debug().
		Str("stage", c.stage).
		Bool("success", success).
		Str("next_stage", nextStage).
		Msg("Finished conversation stage span")
}

// MCPServerInstrumentation provides high-level instrumentation for the MCP server
type MCPServerInstrumentation struct {
	middleware *OTELMiddleware
	logger     zerolog.Logger
}

// NewMCPServerInstrumentation creates a new MCP server instrumentation
func NewMCPServerInstrumentation(serviceName string, logger zerolog.Logger) *MCPServerInstrumentation {
	return &MCPServerInstrumentation{
		middleware: NewOTELMiddleware(serviceName, logger),
		logger:     logger,
	}
}

// InstrumentTool wraps tool execution with tracing
func (msi *MCPServerInstrumentation) InstrumentTool(ctx context.Context, toolName string, fn func(context.Context) (interface{}, error)) (interface{}, error) {
	span := msi.middleware.StartToolSpan(ctx, toolName, map[string]interface{}{
		"mcp.tool.instrumented": true,
	})
	defer func() {
		// We'll set success/failure in the deferred function
	}()

	ctx = span.Context()

	start := time.Now()
	result, err := fn(ctx)
	duration := time.Since(start)

	// Add performance metrics
	span.SetAttributes(map[string]interface{}{
		"mcp.tool.duration_ms": float64(duration.Nanoseconds()) / 1e6,
	})

	if err != nil {
		span.RecordError(err, "Tool execution failed")
		span.Finish(false, 0)
		return nil, err
	}

	// Calculate result size (rough estimate)
	resultSize := len(fmt.Sprintf("%+v", result))
	span.Finish(true, resultSize)

	return result, nil
}

// GetMiddleware returns the underlying OTEL middleware
func (msi *MCPServerInstrumentation) GetMiddleware() *OTELMiddleware {
	return msi.middleware
}
