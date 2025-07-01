package server

import (
	"context"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/internal/types"
)

// Test GetLogsArgs type
func TestGetLogsArgs(t *testing.T) {
	args := GetLogsArgs{
		BaseToolArgs: types.BaseToolArgs{
			SessionID: "session-123",
			DryRun:    false,
		},
		Source:    "server",
		Lines:     50,
		Level:     "info",
		TimeRange: "1h",
		Pattern:   "error",
		Limit:     50,
		Format:    "json",
	}

	if args.BaseToolArgs.SessionID != "session-123" {
		t.Errorf("Expected SessionID to be 'session-123', got '%s'", args.BaseToolArgs.SessionID)
	}
	if args.Source != "server" {
		t.Errorf("Expected Source to be 'server', got '%s'", args.Source)
	}
	if args.Lines != 50 {
		t.Errorf("Expected Lines to be 50, got %d", args.Lines)
	}
	if args.Level != "info" {
		t.Errorf("Expected Level to be 'info', got '%s'", args.Level)
	}
	if args.TimeRange != "1h" {
		t.Errorf("Expected TimeRange to be '1h', got '%s'", args.TimeRange)
	}
	if args.Pattern != "error" {
		t.Errorf("Expected Pattern to be 'error', got '%s'", args.Pattern)
	}
	if args.Limit != 50 {
		t.Errorf("Expected Limit to be 50, got %d", args.Limit)
	}
	if args.Format != "json" {
		t.Errorf("Expected Format to be 'json', got '%s'", args.Format)
	}
}

// Test GetLogsResult type
func TestGetLogsResult(t *testing.T) {
	oldestTime := time.Now().Add(-1 * time.Hour)

	result := GetLogsResult{
		BaseToolResponse: types.BaseToolResponse{
			SessionID: "session-456",
			Tool:      "get_logs",
		},
		Source:    "server",
		SessionID: "session-456",
		Lines: []LogLine{
			{Level: "info", Message: "Test log entry", Timestamp: oldestTime, Component: "server"},
		},
		Total: 100,
	}

	if result.SessionID != "session-456" {
		t.Errorf("Expected SessionID to be 'session-456', got '%s'", result.SessionID)
	}
	if result.Tool != "get_logs" {
		t.Errorf("Expected Tool to be 'get_logs', got '%s'", result.Tool)
	}
	if result.Source != "server" {
		t.Errorf("Expected Source to be 'server', got '%s'", result.Source)
	}
	if len(result.Lines) != 1 {
		t.Errorf("Expected 1 log entry, got %d", len(result.Lines))
	}
	if result.Total != 100 {
		t.Errorf("Expected Total to be 100, got %d", result.Total)
	}
	if result.Lines[0].Level != "info" {
		t.Errorf("Expected first log level to be 'info', got '%s'", result.Lines[0].Level)
	}
	if result.Lines[0].Message != "Test log entry" {
		t.Errorf("Expected first log message to be 'Test log entry', got '%s'", result.Lines[0].Message)
	}
}

// Test GetLogsTool metadata
func TestGetLogsToolMetadata(t *testing.T) {
	tool := &GetLogsTool{}

	metadata := tool.GetMetadata()
	if metadata.Name != "get_logs" {
		t.Errorf("Expected tool name to be 'get_logs', got '%s'", metadata.Name)
	}
	if metadata.Category != "logs" {
		t.Errorf("Expected tool category to be 'logs', got '%s'", metadata.Category)
	}
	if metadata.Description == "" {
		t.Error("Expected tool description to not be empty")
	}
}

// Test GetLogsTool validation
func TestGetLogsToolValidation(t *testing.T) {
	tool := &GetLogsTool{}

	// Test with valid args
	validArgs := GetLogsArgs{
		BaseToolArgs: types.BaseToolArgs{
			SessionID: "test-session",
		},
		Source: "server",
	}

	err := tool.Validate(context.Background(), validArgs)
	if err != nil {
		t.Errorf("Validation should pass for valid args, got error: %v", err)
	}

	// Test with invalid args type
	err = tool.Validate(context.Background(), "invalid")
	if err == nil {
		t.Error("Validation should fail for invalid args type")
	}
}

// Test LogLine type
func TestLogLineType(t *testing.T) {
	timestamp := time.Now()
	logLine := LogLine{
		Timestamp: timestamp,
		Level:     "error",
		Message:   "Test error message",
		Component: "server",
	}

	if logLine.Timestamp != timestamp {
		t.Errorf("Expected timestamp to match, got %v", logLine.Timestamp)
	}
	if logLine.Level != "error" {
		t.Errorf("Expected level to be 'error', got '%s'", logLine.Level)
	}
	if logLine.Message != "Test error message" {
		t.Errorf("Expected message to be 'Test error message', got '%s'", logLine.Message)
	}
	if logLine.Component != "server" {
		t.Errorf("Expected component to be 'server', got '%s'", logLine.Component)
	}
}

// Test GetLogs function execution
func TestGetLogsFunction(t *testing.T) {
	args := GetLogsArgs{
		BaseToolArgs: types.BaseToolArgs{
			SessionID: "test-session",
		},
		Source: "server",
		Lines:  10,
	}

	result, err := GetLogs(context.Background(), args)
	if err != nil {
		t.Errorf("GetLogs should not return error, got %v", err)
	}
	if result == nil {
		t.Error("GetLogs should return a result")
	}
	if result.Source != "server" {
		t.Errorf("Expected result source to be 'server', got '%s'", result.Source)
	}
	if result.SessionID != "test-session" {
		t.Errorf("Expected result session ID to be 'test-session', got '%s'", result.SessionID)
	}
}

// Test GetLogs function with invalid args
func TestGetLogsFunctionInvalidArgs(t *testing.T) {
	// Test with empty source
	args := GetLogsArgs{
		BaseToolArgs: types.BaseToolArgs{
			SessionID: "test-session",
		},
		Source: "", // Empty source should cause error
	}

	result, err := GetLogs(context.Background(), args)
	if err == nil {
		t.Error("GetLogs should return error for empty source")
	}
	if result != nil && result.Source == "" {
		// This is expected behavior - the function returns a result with empty source
		// and an error, which is fine
	}
}

// Test GetLogsTool Execute with valid args
func TestGetLogsTool_Execute_ValidArgs(t *testing.T) {
	tool := &GetLogsTool{}

	args := GetLogsArgs{
		BaseToolArgs: types.BaseToolArgs{
			SessionID: "test-session",
		},
		Source: "server",
		Level:  "info",
		Format: "json",
	}

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Errorf("Execute should not return error, got %v", err)
	}
	if result == nil {
		t.Error("Execute should return result")
	}

	// Verify result type
	if logsResult, ok := result.(*GetLogsResult); ok {
		if logsResult.Source != "server" {
			t.Errorf("Expected result source to be 'server', got '%s'", logsResult.Source)
		}
	} else {
		t.Error("Expected result to be of type *GetLogsResult")
	}
}

// Test GetLogsTool Execute with invalid args
func TestGetLogsTool_Execute_InvalidArgs(t *testing.T) {
	tool := &GetLogsTool{}

	// Invalid args type
	result, err := tool.Execute(context.Background(), "invalid")
	if err == nil {
		t.Error("Execute should return error for invalid args type")
	}
	if result != nil {
		t.Error("Execute should not return result for invalid args")
	}
}

// Test GetLogsArgs variations
func TestGetLogsArgsVariations(t *testing.T) {
	// Test minimal args
	minimalArgs := GetLogsArgs{
		BaseToolArgs: types.BaseToolArgs{
			SessionID: "minimal-session",
		},
	}

	if minimalArgs.BaseToolArgs.SessionID != "minimal-session" {
		t.Errorf("Expected SessionID to be 'minimal-session', got '%s'", minimalArgs.BaseToolArgs.SessionID)
	}
	if minimalArgs.Source != "" {
		t.Errorf("Expected Source to be empty by default, got '%s'", minimalArgs.Source)
	}
	if minimalArgs.Level != "" {
		t.Errorf("Expected Level to be empty by default, got '%s'", minimalArgs.Level)
	}
	if minimalArgs.Lines != 0 {
		t.Errorf("Expected Lines to be 0 by default, got %d", minimalArgs.Lines)
	}
	if minimalArgs.Limit != 0 {
		t.Errorf("Expected Limit to be 0 by default, got %d", minimalArgs.Limit)
	}

	// Test full args
	fullArgs := GetLogsArgs{
		BaseToolArgs: types.BaseToolArgs{
			SessionID: "full-session",
			DryRun:    true,
		},
		Source:    "session",
		Lines:     200,
		Level:     "debug",
		TimeRange: "24h",
		Pattern:   "critical",
		Limit:     200,
		Format:    "text",
	}

	if fullArgs.BaseToolArgs.SessionID != "full-session" {
		t.Errorf("Expected SessionID to be 'full-session', got '%s'", fullArgs.BaseToolArgs.SessionID)
	}
	if !fullArgs.BaseToolArgs.DryRun {
		t.Error("Expected DryRun to be true")
	}
	if fullArgs.Source != "session" {
		t.Errorf("Expected Source to be 'session', got '%s'", fullArgs.Source)
	}
	if fullArgs.Lines != 200 {
		t.Errorf("Expected Lines to be 200, got %d", fullArgs.Lines)
	}
	if fullArgs.Level != "debug" {
		t.Errorf("Expected Level to be 'debug', got '%s'", fullArgs.Level)
	}
	if fullArgs.TimeRange != "24h" {
		t.Errorf("Expected TimeRange to be '24h', got '%s'", fullArgs.TimeRange)
	}
	if fullArgs.Pattern != "critical" {
		t.Errorf("Expected Pattern to be 'critical', got '%s'", fullArgs.Pattern)
	}
	if fullArgs.Limit != 200 {
		t.Errorf("Expected Limit to be 200, got %d", fullArgs.Limit)
	}
	if fullArgs.Format != "text" {
		t.Errorf("Expected Format to be 'text', got '%s'", fullArgs.Format)
	}
}

// Test GetLogsTool struct
func TestGetLogsToolStruct(t *testing.T) {
	tool := GetLogsTool{}

	// Test that the struct can be created without error
	if &tool == nil {
		t.Error("GetLogsTool struct should be creatable")
	}

	// Test basic functionality
	metadata := tool.GetMetadata()
	if metadata.Name == "" {
		t.Error("Expected tool to have a name")
	}

	// Test validation with valid args should pass
	validArgs := GetLogsArgs{
		BaseToolArgs: types.BaseToolArgs{
			SessionID: "test-session",
		},
		Source: "server",
	}
	err := tool.Validate(context.Background(), validArgs)
	if err != nil {
		t.Errorf("Expected validation to pass for valid args, got: %v", err)
	}
}
