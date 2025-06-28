package server

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/Azure/container-kit/pkg/mcp/internal/utils"
	"github.com/rs/zerolog"
)

// Test GetLogsArgs type
func TestGetLogsArgs(t *testing.T) {
	args := GetLogsArgs{
		BaseToolArgs: types.BaseToolArgs{
			SessionID: "session-123",
			DryRun:    false,
		},
		Level:          "info",
		TimeRange:      "1h",
		Pattern:        "error",
		Limit:          50,
		Format:         "json",
		IncludeCallers: true,
	}

	if args.SessionID != "session-123" {
		t.Errorf("Expected SessionID to be 'session-123', got '%s'", args.SessionID)
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
	if !args.IncludeCallers {
		t.Error("Expected IncludeCallers to be true")
	}
}

// Test GetLogsResult type
func TestGetLogsResult(t *testing.T) {
	oldestTime := time.Now().Add(-1 * time.Hour)
	newestTime := time.Now()

	result := GetLogsResult{
		BaseToolResponse: types.BaseToolResponse{
			SessionID: "session-456",
			Tool:      "get_logs",
		},
		Logs: []utils.LogEntry{
			{Level: "info", Message: "Test log entry"},
		},
		TotalCount:    100,
		FilteredCount: 10,
		TimeRange:     "1h",
		OldestEntry:   &oldestTime,
		NewestEntry:   &newestTime,
		Format:        "json",
		LogText:       "test log text",
	}

	if result.SessionID != "session-456" {
		t.Errorf("Expected SessionID to be 'session-456', got '%s'", result.SessionID)
	}
	if result.Tool != "get_logs" {
		t.Errorf("Expected Tool to be 'get_logs', got '%s'", result.Tool)
	}
	if len(result.Logs) != 1 {
		t.Errorf("Expected 1 log entry, got %d", len(result.Logs))
	}
	if result.TotalCount != 100 {
		t.Errorf("Expected TotalCount to be 100, got %d", result.TotalCount)
	}
	if result.FilteredCount != 10 {
		t.Errorf("Expected FilteredCount to be 10, got %d", result.FilteredCount)
	}
	if result.TimeRange != "1h" {
		t.Errorf("Expected TimeRange to be '1h', got '%s'", result.TimeRange)
	}
	if result.Format != "json" {
		t.Errorf("Expected Format to be 'json', got '%s'", result.Format)
	}
	if result.LogText != "test log text" {
		t.Errorf("Expected LogText to be 'test log text', got '%s'", result.LogText)
	}
}

// Test RingBufferLogProvider
func TestRingBufferLogProvider_GetLogs(t *testing.T) {
	logs := []utils.LogEntry{
		{Level: "info", Message: "Info message", Timestamp: time.Now()},
		{Level: "warn", Message: "Warning message", Timestamp: time.Now()},
		{Level: "error", Message: "Error message", Timestamp: time.Now()},
	}

	buffer := utils.NewRingBuffer(10)
	for _, log := range logs {
		buffer.Add(log)
	}
	provider := NewRingBufferLogProvider(buffer)

	entries, err := provider.GetLogs("info", time.Now().Add(-1*time.Hour), "", 0)
	if err != nil {
		t.Errorf("GetLogs should not return error, got %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("Expected 3 log entries, got %d", len(entries))
	}
}

// Test RingBufferLogProvider with limit
func TestRingBufferLogProvider_GetLogsWithLimit(t *testing.T) {
	logs := []utils.LogEntry{
		{Level: "info", Message: "Info 1", Timestamp: time.Now()},
		{Level: "info", Message: "Info 2", Timestamp: time.Now()},
		{Level: "info", Message: "Info 3", Timestamp: time.Now()},
		{Level: "info", Message: "Info 4", Timestamp: time.Now()},
		{Level: "info", Message: "Info 5", Timestamp: time.Now()},
	}

	buffer := utils.NewRingBuffer(10)
	for _, log := range logs {
		buffer.Add(log)
	}
	provider := NewRingBufferLogProvider(buffer)

	// Test with limit
	entries, err := provider.GetLogs("info", time.Now().Add(-1*time.Hour), "", 3)
	if err != nil {
		t.Errorf("GetLogs should not return error, got %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("Expected 3 log entries due to limit, got %d", len(entries))
	}
}

// Test RingBufferLogProvider GetTotalLogCount
func TestRingBufferLogProvider_GetTotalLogCount(t *testing.T) {
	buffer := utils.NewRingBuffer(50)
	// Add 42 entries to match expected count
	for i := 0; i < 42; i++ {
		buffer.Add(utils.LogEntry{Level: "info", Message: fmt.Sprintf("Entry %d", i), Timestamp: time.Now()})
	}
	provider := NewRingBufferLogProvider(buffer)

	count := provider.GetTotalLogCount()
	if count != 42 {
		t.Errorf("Expected total count to be 42, got %d", count)
	}
}

// Test NewRingBufferLogProvider constructor
func TestNewRingBufferLogProvider(t *testing.T) {
	buffer := &utils.RingBuffer{} // Assume this exists
	provider := NewRingBufferLogProvider(buffer)

	if provider == nil {
		t.Error("NewRingBufferLogProvider should not return nil")
		return
	}
	if provider.buffer != buffer {
		t.Error("Expected buffer to be set correctly")
	}
}

// Test NewGetLogsTool constructor
func TestNewGetLogsTool(t *testing.T) {
	logger := zerolog.Nop()
	buffer := utils.NewRingBuffer(10)
	provider := NewRingBufferLogProvider(buffer)

	tool := NewGetLogsTool(logger, provider)

	if tool == nil {
		t.Error("NewGetLogsTool should not return nil")
		return
	}
	if tool.logProvider != provider {
		t.Error("Expected logProvider to be set correctly")
	}
}

// Test GetLogsTool Execute with valid args
func TestGetLogsTool_Execute_ValidArgs(t *testing.T) {
	logger := zerolog.Nop()
	buffer := utils.NewRingBuffer(10)
	// Add test log entry
	buffer.Add(utils.LogEntry{Level: "info", Message: "Test message", Timestamp: time.Now()})
	provider := NewRingBufferLogProvider(buffer)

	tool := NewGetLogsTool(logger, provider)

	args := GetLogsArgs{
		BaseToolArgs: types.BaseToolArgs{
			SessionID: "test-session",
		},
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
}

// Test GetLogsTool Execute with invalid args
func TestGetLogsTool_Execute_InvalidArgs(t *testing.T) {
	logger := zerolog.Nop()
	buffer := utils.NewRingBuffer(10)
	provider := NewRingBufferLogProvider(buffer)

	tool := NewGetLogsTool(logger, provider)

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

	if minimalArgs.SessionID != "minimal-session" {
		t.Errorf("Expected SessionID to be 'minimal-session', got '%s'", minimalArgs.SessionID)
	}
	if minimalArgs.Level != "" {
		t.Errorf("Expected Level to be empty by default, got '%s'", minimalArgs.Level)
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
		Level:          "debug",
		TimeRange:      "24h",
		Pattern:        "critical",
		Limit:          200,
		Format:         "text",
		IncludeCallers: false,
	}

	if fullArgs.SessionID != "full-session" {
		t.Errorf("Expected SessionID to be 'full-session', got '%s'", fullArgs.SessionID)
	}
	if !fullArgs.DryRun {
		t.Error("Expected DryRun to be true")
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
	if fullArgs.IncludeCallers {
		t.Error("Expected IncludeCallers to be false")
	}
}

// Test GetLogsTool struct initialization
func TestGetLogsToolStruct(t *testing.T) {
	logger := zerolog.Nop()
	buffer := utils.NewRingBuffer(10)
	// Add 5 test entries to match expected count
	for i := 0; i < 5; i++ {
		buffer.Add(utils.LogEntry{Level: "info", Message: fmt.Sprintf("Test message %d", i), Timestamp: time.Now()})
	}
	provider := NewRingBufferLogProvider(buffer)

	tool := GetLogsTool{
		logger:      logger,
		logProvider: provider,
	}

	if tool.logProvider == nil {
		t.Error("Expected logProvider to be set")
	}
	if tool.logProvider.GetTotalLogCount() != 5 {
		t.Errorf("Expected total log count to be 5, got %d", tool.logProvider.GetTotalLogCount())
	}
}
