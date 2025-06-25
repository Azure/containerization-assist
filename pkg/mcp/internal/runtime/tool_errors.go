package runtime

// ToolErrorExtensions provides tool-specific error handling utilities
// This file contains extensions and utilities for tool errors that are specific
// to the runtime package and not part of the core error types in errors.go

import (
	"context"
	"time"
)

// ToolErrorReporter provides reporting capabilities for tool errors
type ToolErrorReporter struct {
	sessionID string
	toolName  string
}

// NewToolErrorReporter creates a new tool error reporter
func NewToolErrorReporter(sessionID, toolName string) *ToolErrorReporter {
	return &ToolErrorReporter{
		sessionID: sessionID,
		toolName:  toolName,
	}
}

// ReportError reports a tool error with context
func (r *ToolErrorReporter) ReportError(ctx context.Context, err error) {
	// Tool-specific error reporting logic would go here
	// This could include metrics collection, logging, or alerting
}

// ReportMetrics reports error metrics for tools
func (r *ToolErrorReporter) ReportMetrics(ctx context.Context, errorType ErrorType, duration time.Duration) {
	// Tool-specific metrics reporting logic would go here
}
