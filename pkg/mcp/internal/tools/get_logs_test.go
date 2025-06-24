package tools

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/utils"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockLogProvider for testing
type MockLogProvider struct {
	logs  []utils.LogEntry
	err   error
	total int
}

func NewMockLogProvider() *MockLogProvider {
	now := time.Now()
	return &MockLogProvider{
		logs: []utils.LogEntry{
			{
				Timestamp: now.Add(-5 * time.Minute),
				Level:     "info",
				Message:   "Server started successfully",
				Fields:    map[string]interface{}{"port": 8080},
			},
			{
				Timestamp: now.Add(-4 * time.Minute),
				Level:     "debug",
				Message:   "Processing request",
				Fields:    map[string]interface{}{"method": "GET", "path": "/health"},
			},
			{
				Timestamp: now.Add(-3 * time.Minute),
				Level:     "warn",
				Message:   "High memory usage detected",
				Fields:    map[string]interface{}{"memory_mb": 512},
			},
			{
				Timestamp: now.Add(-2 * time.Minute),
				Level:     "error",
				Message:   "Failed to connect to database",
				Fields:    map[string]interface{}{"error": "connection timeout"},
				Caller:    "database.go:123",
			},
			{
				Timestamp: now.Add(-1 * time.Minute),
				Level:     "info",
				Message:   "Request completed",
				Fields:    map[string]interface{}{"duration_ms": 150},
			},
		},
		total: 100, // Simulate more logs in buffer
	}
}

func (m *MockLogProvider) GetLogs(level string, since time.Time, pattern string, limit int) ([]utils.LogEntry, error) {
	if m.err != nil {
		return nil, m.err
	}

	var filtered []utils.LogEntry

	// Filter by level
	levelPriority := getLogLevelPriority(level)

	for _, log := range m.logs {
		// Time filter
		if !since.IsZero() && log.Timestamp.Before(since) {
			continue
		}

		// Level filter
		if getLogLevelPriority(log.Level) < levelPriority {
			continue
		}

		// Pattern filter
		if pattern != "" && !strings.Contains(strings.ToLower(log.Message), strings.ToLower(pattern)) {
			// Also check fields
			found := false
			for _, v := range log.Fields {
				if str, ok := v.(string); ok && strings.Contains(strings.ToLower(str), strings.ToLower(pattern)) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		filtered = append(filtered, log)
	}

	// Apply limit
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[len(filtered)-limit:]
	}

	return filtered, nil
}

func (m *MockLogProvider) GetTotalLogCount() int {
	return m.total
}

func getLogLevelPriority(level string) int {
	priorities := map[string]int{
		"trace": 0,
		"debug": 1,
		"info":  2,
		"warn":  3,
		"error": 4,
		"fatal": 5,
		"panic": 6,
	}

	if p, ok := priorities[level]; ok {
		return p
	}
	return 2 // default to info
}

func TestGetLogsTool_Execute(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("get all logs with default filters", func(t *testing.T) {
		// Setup
		provider := NewMockLogProvider()
		tool := NewGetLogsTool(logger, provider)

		// Execute
		args := GetLogsArgs{
			Format: "json",
		}
		result, err := tool.Execute(context.Background(), args)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "json", result.Format)
		assert.Equal(t, 100, result.TotalCount)
		assert.Equal(t, 4, result.FilteredCount) // info, warn, error, info (2 info logs)
		assert.NotNil(t, result.OldestEntry)
		assert.NotNil(t, result.NewestEntry)
	})

	t.Run("filter by log level", func(t *testing.T) {
		// Setup
		provider := NewMockLogProvider()
		tool := NewGetLogsTool(logger, provider)

		// Execute
		args := GetLogsArgs{
			Level:  "warn",
			Format: "json",
		}
		result, err := tool.Execute(context.Background(), args)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, 2, result.FilteredCount) // warn and error

		// Check that we only get warn and error logs
		for _, log := range result.Logs {
			assert.Contains(t, []string{"warn", "error"}, log.Level)
		}
	})

	t.Run("filter by time range", func(t *testing.T) {
		// Setup
		provider := NewMockLogProvider()
		tool := NewGetLogsTool(logger, provider)

		// Execute
		args := GetLogsArgs{
			Level:     "debug", // Include all levels
			TimeRange: "2m30s", // Last 2.5 minutes
			Format:    "json",
		}
		result, err := tool.Execute(context.Background(), args)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, 2, result.FilteredCount) // Only last 2 logs
	})

	t.Run("filter by pattern", func(t *testing.T) {
		// Setup
		provider := NewMockLogProvider()
		tool := NewGetLogsTool(logger, provider)

		// Execute
		args := GetLogsArgs{
			Level:   "debug",
			Pattern: "request",
			Format:  "json",
		}
		result, err := tool.Execute(context.Background(), args)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, 2, result.FilteredCount) // "Processing request" and "Request completed"

		for _, log := range result.Logs {
			assert.Contains(t, strings.ToLower(log.Message), "request")
		}
	})

	t.Run("apply limit", func(t *testing.T) {
		// Setup
		provider := NewMockLogProvider()
		tool := NewGetLogsTool(logger, provider)

		// Execute
		args := GetLogsArgs{
			Level:  "debug",
			Limit:  2,
			Format: "json",
		}
		result, err := tool.Execute(context.Background(), args)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, 2, result.FilteredCount)
		assert.Len(t, result.Logs, 2)
		// Should get the most recent 2 logs
		assert.Equal(t, "Request completed", result.Logs[1].Message)
	})

	t.Run("text format output", func(t *testing.T) {
		// Setup
		provider := NewMockLogProvider()
		tool := NewGetLogsTool(logger, provider)

		// Execute
		args := GetLogsArgs{
			Level:          "error",
			Format:         "text",
			IncludeCallers: true,
		}
		result, err := tool.Execute(context.Background(), args)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "text", result.Format)
		assert.NotEmpty(t, result.LogText)
		assert.Contains(t, result.LogText, "ERROR Failed to connect to database")
		assert.Contains(t, result.LogText, "caller=database.go:123")
		assert.Nil(t, result.Logs) // Should be nil for text format
	})

	t.Run("text format without callers", func(t *testing.T) {
		// Setup
		provider := NewMockLogProvider()
		tool := NewGetLogsTool(logger, provider)

		// Execute
		args := GetLogsArgs{
			Level:          "error",
			Format:         "text",
			IncludeCallers: false,
		}
		result, err := tool.Execute(context.Background(), args)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.NotContains(t, result.LogText, "caller=")
	})

	t.Run("invalid time range", func(t *testing.T) {
		// Setup
		provider := NewMockLogProvider()
		tool := NewGetLogsTool(logger, provider)

		// Execute
		args := GetLogsArgs{
			TimeRange: "invalid",
		}
		result, err := tool.Execute(context.Background(), args)

		// Assert
		require.NoError(t, err) // Execute returns error in result, not as error
		assert.NotNil(t, result)
		assert.NotNil(t, result.Error)
		assert.Equal(t, "INVALID_TIME_RANGE", result.Error.Type)
		assert.False(t, result.Error.Retryable)
	})

	t.Run("log retrieval error", func(t *testing.T) {
		// Setup
		provider := &MockLogProvider{
			err: fmt.Errorf("failed to read logs"),
		}
		tool := NewGetLogsTool(logger, provider)

		// Execute
		args := GetLogsArgs{}
		result, err := tool.Execute(context.Background(), args)

		// Assert
		require.NoError(t, err) // Execute returns error in result, not as error
		assert.NotNil(t, result)
		assert.NotNil(t, result.Error)
		assert.Equal(t, "LOG_RETRIEVAL_FAILED", result.Error.Type)
		assert.True(t, result.Error.Retryable)
	})

	t.Run("empty logs", func(t *testing.T) {
		// Setup
		provider := &MockLogProvider{
			logs:  []utils.LogEntry{},
			total: 0,
		}
		tool := NewGetLogsTool(logger, provider)

		// Execute
		args := GetLogsArgs{}
		result, err := tool.Execute(context.Background(), args)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, 0, result.FilteredCount)
		assert.Nil(t, result.OldestEntry)
		assert.Nil(t, result.NewestEntry)
	})
}
