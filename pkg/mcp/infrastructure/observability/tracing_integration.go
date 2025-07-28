// Package observability provides unified monitoring and health infrastructure
// for the MCP components.
package observability

import (
	"context"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

type ServerInfo struct {
	ServiceName     string
	ServiceVersion  string
	Environment     string
	TraceSampleRate float64 // Kept for API compatibility but unused
}

func InitFromServerInfo(ctx context.Context, info ServerInfo, logger *slog.Logger) error {
	// No-op implementation, telemetry has been removed
	if logger != nil {
		logger.Info("Telemetry has been disabled")
	}
	return nil
}

func ConfigFromServerInfo(info ServerInfo) Config {
	return DefaultConfig()
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

// MiddlewareHandler creates middleware for MCP requests
func MiddlewareHandler(next func(context.Context) error) func(context.Context) error {
	return func(ctx context.Context) error {
		// Execute the handler without tracing
		return next(ctx)
	}
}

// WorkflowTracer provides stub utilities for workflow operations
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

// TraceStep executes a workflow step without tracing
func (wt *WorkflowTracer) TraceStep(ctx context.Context, stepName string, fn func(context.Context) error) error {
	return fn(ctx)
}

func (wt *WorkflowTracer) AddWorkflowAttributes(ctx context.Context) {
	// No-op implementation
}
