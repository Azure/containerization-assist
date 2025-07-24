// Package observability provides unified monitoring, tracing, and health infrastructure
// for the MCP components. It consolidates telemetry, distributed tracing, health checks,
// and logging enrichment into a single coherent package.
package observability

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel/attribute"
)

type ServerInfo struct {
	ServiceName     string
	ServiceVersion  string
	Environment     string
	TraceSampleRate float64
}

func InitFromServerInfo(ctx context.Context, info ServerInfo, logger *slog.Logger) error {
	tracingConfig := ConfigFromServerInfo(info)

	if logger != nil {
		logger.Info("Initializing OpenTelemetry tracing",
			"enabled", tracingConfig.Enabled,
			"service_name", tracingConfig.ServiceName,
			"environment", tracingConfig.Environment,
			"sample_rate", tracingConfig.SampleRate,
		)
	}

	return InitializeTracing(ctx, tracingConfig)
}

func ConfigFromServerInfo(info ServerInfo) Config {
	tracingConfig := DefaultConfig()

	// Override with server info values
	tracingConfig.ServiceName = info.ServiceName
	tracingConfig.ServiceVersion = info.ServiceVersion
	tracingConfig.Environment = info.Environment
	tracingConfig.SampleRate = info.TraceSampleRate

	// Check environment variables for OTEL configuration
	if enabled := getEnvBool("CONTAINER_KIT_OTEL_ENABLED", false); enabled {
		tracingConfig.Enabled = true
	}

	if endpoint := getEnvString("CONTAINER_KIT_OTEL_ENDPOINT", ""); endpoint != "" {
		tracingConfig.Endpoint = endpoint
	}

	if headers := getEnvString("CONTAINER_KIT_OTEL_HEADERS", ""); headers != "" {
		tracingConfig.Headers = parseHeaders(headers)
	}

	if sampleRate := getEnvFloat64("CONTAINER_KIT_TRACE_SAMPLE_RATE", -1); sampleRate >= 0 {
		tracingConfig.SampleRate = sampleRate
	}

	return tracingConfig
}

func getEnvString(key, defaultValue string) string {
	if value := getEnv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := getEnv(key); value != "" {
		return value == "true" || value == "1"
	}
	return defaultValue
}

func getEnvFloat64(key string, defaultValue float64) float64 {
	if value := getEnv(key); value != "" {
		if parsed, err := strconv.ParseFloat(value, 64); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func parseHeaders(headers string) map[string]string {
	headerMap := make(map[string]string)
	pairs := strings.Split(headers, ",")
	for _, pair := range pairs {
		if kv := strings.SplitN(strings.TrimSpace(pair), "=", 2); len(kv) == 2 {
			headerMap[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}
	return headerMap
}

var getEnv = os.Getenv

// MiddlewareHandler creates tracing middleware for MCP requests
func MiddlewareHandler(next func(context.Context) error) func(context.Context) error {
	return func(ctx context.Context) error {
		// Extract operation name from context if available
		operationName := "mcp.request"
		if op := ctx.Value("operation"); op != nil {
			if opStr, ok := op.(string); ok {
				operationName = fmt.Sprintf("mcp.%s", opStr)
			}
		}

		ctx, span := StartSpan(ctx, operationName)
		defer span.End()

		// Add MCP-specific attributes
		span.SetAttributes(
			attribute.String(AttrComponent, "mcp"),
		)

		// Execute the handler
		err := next(ctx)

		if err != nil {
			span.RecordError(err)
		}

		return err
	}
}

// WorkflowTracer provides tracing utilities for workflow operations
type WorkflowTracer struct {
	workflowID   string
	workflowName string
}

func NewWorkflowTracer(workflowID, workflowName string) *WorkflowTracer {
	return &WorkflowTracer{
		workflowID:   workflowID,
		workflowName: workflowName,
	}
}

// TraceStep executes a workflow step with tracing
func (wt *WorkflowTracer) TraceStep(ctx context.Context, stepName string, fn func(context.Context) error) error {
	return TraceWorkflowStep(ctx, wt.workflowID, stepName, fn)
}

func (wt *WorkflowTracer) AddWorkflowAttributes(ctx context.Context) {
	span := SpanFromContext(ctx)
	if span.IsRecording() {
		span.SetAttributes(
			attribute.String(AttrProgressWorkflowID, wt.workflowID),
			attribute.String("workflow.name", wt.workflowName),
			attribute.String(AttrComponent, "workflow"),
		)
	}
}
