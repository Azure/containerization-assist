// Package workflow provides shared types and utilities for progress reporting
package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/server"
)

// NotificationSender is an interface for sending MCP notifications
type NotificationSender interface {
	SendNotificationToClient(ctx context.Context, method string, params interface{}) error
}

// mcpServerWrapper wraps the mcp-go MCPServer to match our interface
type mcpServerWrapper struct {
	server interface {
		SendNotificationToClient(ctx context.Context, method string, params map[string]any) error
	}
}

// SendNotificationToClient implements NotificationSender interface
func (w *mcpServerWrapper) SendNotificationToClient(ctx context.Context, method string, params interface{}) error {
	// Convert params to map[string]any
	paramsMap, ok := params.(map[string]interface{})
	if !ok {
		return fmt.Errorf("params must be map[string]interface{}, got %T", params)
	}

	// Convert map[string]interface{} to map[string]any
	anyMap := make(map[string]any, len(paramsMap))
	for k, v := range paramsMap {
		anyMap[k] = v
	}

	return w.server.SendNotificationToClient(ctx, method, anyMap)
}

// getServerFromContext attempts to extract the MCP server from the context
func getServerFromContext(ctx context.Context) NotificationSender {
	if s := server.ServerFromContext(ctx); s != nil {
		return &mcpServerWrapper{server: s}
	}
	return nil
}

// generateTraceID generates a unique trace ID for correlation
func generateTraceID() string {
	// Simple trace ID: timestamp + random suffix
	return fmt.Sprintf("trace-%d-%d", time.Now().Unix(), time.Now().Nanosecond()%1000)
}

// mapStatusToCode maps status strings to numeric codes for UI styling
func mapStatusToCode(status string) int {
	switch status {
	case "running":
		return 1
	case "completed":
		return 2
	case "failed":
		return 3
	case "skipped":
		return 4
	case "retrying":
		return 5
	default:
		return 0
	}
}

// repeatChar repeats a character n times
func repeatChar(char rune, n int) string {
	if n <= 0 {
		return ""
	}
	result := make([]rune, n)
	for i := range result {
		result[i] = char
	}
	return string(result)
}
