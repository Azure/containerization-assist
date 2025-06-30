package server

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/Azure/container-kit/pkg/mcp/internal/utils"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/types"
	"github.com/rs/zerolog"
)

// GetLogsArgs represents the arguments for getting server logs
type GetLogsArgs struct {
	types.BaseToolArgs
	Level          string `json:"level,omitempty" jsonschema:"enum=trace,enum=debug,enum=info,enum=warn,enum=error,default=info,description=Minimum log level to include"`
	TimeRange      string `json:"time_range,omitempty" jsonschema:"description=Time range filter (e.g. '5m', '1h', '24h')"`
	Pattern        string `json:"pattern,omitempty" jsonschema:"description=Pattern to search for in logs"`
	Limit          int    `json:"limit,omitempty" jsonschema:"default=100,description=Maximum number of log entries to return"`
	Format         string `json:"format,omitempty" jsonschema:"enum=json,enum=text,default=json,description=Output format"`
	IncludeCallers bool   `json:"include_callers,omitempty" jsonschema:"default=false,description=Include caller information"`
}

// GetLogsResult represents the result of getting server logs
type GetLogsResult struct {
	types.BaseToolResponse
	Logs          []utils.LogEntry `json:"logs"`
	TotalCount    int              `json:"total_count"`
	FilteredCount int              `json:"filtered_count"`
	TimeRange     string           `json:"time_range,omitempty"`
	OldestEntry   *time.Time       `json:"oldest_entry,omitempty"`
	NewestEntry   *time.Time       `json:"newest_entry,omitempty"`
	Format        string           `json:"format"`
	LogText       string           `json:"log_text,omitempty"` // For text format
	Error         *types.ToolError `json:"error,omitempty"`
}

// RingBufferLogProvider implements LogProvider using a ring buffer
type RingBufferLogProvider struct {
	buffer *utils.RingBuffer
}

// NewRingBufferLogProvider creates a new ring buffer log provider
func NewRingBufferLogProvider(buffer *utils.RingBuffer) *RingBufferLogProvider {
	return &RingBufferLogProvider{
		buffer: buffer,
	}
}

// GetLogs retrieves logs from the ring buffer
func (p *RingBufferLogProvider) GetLogs(level string, since time.Time, pattern string, limit int) ([]utils.LogEntry, error) {
	entries := p.buffer.GetEntriesFiltered(level, since, pattern)

	// Apply limit
	if limit > 0 && len(entries) > limit {
		// Return the most recent entries
		entries = entries[len(entries)-limit:]
	}

	return entries, nil
}

// GetTotalLogCount returns the total number of logs in the buffer
func (p *RingBufferLogProvider) GetTotalLogCount() int {
	return p.buffer.Size()
}

// GetLogsTool implements the get_logs MCP tool
type GetLogsTool struct {
	logger      zerolog.Logger
	logProvider *RingBufferLogProvider
}

// NewGetLogsTool creates a new get logs tool
func NewGetLogsTool(logger zerolog.Logger, logProvider *RingBufferLogProvider) *GetLogsTool {
	return &GetLogsTool{
		logger:      logger,
		logProvider: logProvider,
	}
}

// Execute implements the unified Tool interface
func (t *GetLogsTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	// Type assertion to get proper args
	logsArgs, ok := args.(GetLogsArgs)
	if !ok {
		return nil, fmt.Errorf("invalid arguments type: expected GetLogsArgs, got %T", args)
	}

	return t.ExecuteTyped(ctx, logsArgs)
}

// ExecuteTyped provides typed execution for backward compatibility
func (t *GetLogsTool) ExecuteTyped(ctx context.Context, args GetLogsArgs) (*GetLogsResult, error) {
	t.logger.Info().
		Str("level", args.Level).
		Str("time_range", args.TimeRange).
		Str("pattern", args.Pattern).
		Int("limit", args.Limit).
		Msg("Retrieving server logs")

	// Set defaults
	if args.Level == "" {
		args.Level = "info"
	}
	if args.Format == "" {
		args.Format = "json"
	}
	if args.Limit == 0 {
		args.Limit = 100
	}

	// Parse time range
	var since time.Time
	if args.TimeRange != "" {
		duration, err := time.ParseDuration(args.TimeRange)
		if err != nil {
			return &GetLogsResult{
				BaseToolResponse: types.NewBaseResponse("get_logs", args.SessionID, args.DryRun),
				Format:           args.Format,
				Error: &types.ToolError{
					Type:      "INVALID_TIME_RANGE",
					Message:   fmt.Sprintf("Invalid time range format: %v", err),
					Retryable: false,
					Timestamp: time.Now(),
				},
			}, nil
		}
		since = time.Now().Add(-duration)
	}

	// Get logs from provider
	logs, err := t.logProvider.GetLogs(args.Level, since, args.Pattern, args.Limit)
	if err != nil {
		return &GetLogsResult{
			BaseToolResponse: types.NewBaseResponse("get_logs", args.SessionID, args.DryRun),
			Format:           args.Format,
			Error: &types.ToolError{
				Type:      "LOG_RETRIEVAL_FAILED",
				Message:   fmt.Sprintf("Failed to retrieve logs: %v", err),
				Retryable: true,
				Timestamp: time.Now(),
			},
		}, nil
	}

	// Calculate time range info
	var oldestEntry, newestEntry *time.Time
	if len(logs) > 0 {
		oldest := logs[0].Timestamp
		newest := logs[len(logs)-1].Timestamp
		oldestEntry = &oldest
		newestEntry = &newest
	}

	result := &GetLogsResult{
		BaseToolResponse: types.NewBaseResponse("get_logs", args.SessionID, args.DryRun),
		Logs:             logs,
		TotalCount:       t.logProvider.GetTotalLogCount(),
		FilteredCount:    len(logs),
		TimeRange:        args.TimeRange,
		OldestEntry:      oldestEntry,
		NewestEntry:      newestEntry,
		Format:           args.Format,
	}

	// Format as text if requested
	if args.Format == "text" {
		var lines []string
		for _, entry := range logs {
			line := utils.FormatLogEntry(entry)
			if !args.IncludeCallers && entry.Caller != "" {
				// Remove caller info if not requested
				line = strings.Replace(line, fmt.Sprintf(" caller=%s", entry.Caller), "", 1)
			}
			lines = append(lines, line)
		}
		result.LogText = strings.Join(lines, "\n")
		// Clear logs array for text format to reduce response size
		result.Logs = nil
	}

	t.logger.Info().
		Int("total_logs", result.TotalCount).
		Int("filtered_logs", result.FilteredCount).
		Str("format", args.Format).
		Msg("Successfully retrieved server logs")

	return result, nil
}

// CreateGlobalLogProvider creates a log provider using the global log buffer
func CreateGlobalLogProvider() *RingBufferLogProvider {
	buffer := utils.GetGlobalLogBuffer()
	if buffer == nil {
		// Initialize if not already done
		utils.InitializeLogCapture(10000) // 10k log entries
		buffer = utils.GetGlobalLogBuffer()
	}
	return NewRingBufferLogProvider(buffer)
}

// GetMetadata returns comprehensive metadata about the get logs tool
func (t *GetLogsTool) GetMetadata() mcptypes.ToolMetadata {
	return mcptypes.ToolMetadata{
		Name:        "get_logs",
		Description: "Retrieve server logs with filtering, pattern matching, and format options",
		Version:     "1.0.0",
		Category:    "Monitoring",
		Dependencies: []string{
			"Log Provider",
			"Ring Buffer",
			"Log Capture System",
		},
		Capabilities: []string{
			"Log retrieval",
			"Level filtering",
			"Time range filtering",
			"Pattern matching",
			"Format conversion",
			"Entry limiting",
			"Caller information",
		},
		Requirements: []string{
			"Log provider instance",
			"Log capture enabled",
		},
		Parameters: map[string]string{
			"level":           "Optional: Minimum log level (trace, debug, info, warn, error)",
			"time_range":      "Optional: Time range filter (e.g. '5m', '1h', '24h')",
			"pattern":         "Optional: Pattern to search for in logs",
			"limit":           "Optional: Maximum number of log entries (default: 100)",
			"format":          "Optional: Output format (json, text)",
			"include_callers": "Optional: Include caller information (default: false)",
		},
		Examples: []mcptypes.ToolExample{
			{
				Name:        "Get recent error logs",
				Description: "Retrieve error-level logs from the last hour",
				Input: map[string]interface{}{
					"level":      "error",
					"time_range": "1h",
					"limit":      50,
				},
				Output: map[string]interface{}{
					"logs": []map[string]interface{}{
						{
							"timestamp": "2024-12-17T10:30:00Z",
							"level":     "error",
							"message":   "Failed to connect to Docker daemon",
							"component": "docker_client",
						},
					},
					"total_count":    1000,
					"filtered_count": 15,
					"time_range":     "1h",
					"format":         "json",
				},
			},
			{
				Name:        "Search for specific pattern in text format",
				Description: "Find logs containing 'build_image' pattern in text format",
				Input: map[string]interface{}{
					"pattern":         "build_image",
					"format":          "text",
					"include_callers": true,
					"limit":           25,
				},
				Output: map[string]interface{}{
					"log_text":       "2024-12-17T10:30:00Z INFO Starting build_image operation caller=tools/build_image.go:45\n...",
					"total_count":    1000,
					"filtered_count": 8,
					"format":         "text",
				},
			},
		},
	}
}

// Validate checks if the provided arguments are valid for the get logs tool
func (t *GetLogsTool) Validate(ctx context.Context, args interface{}) error {
	logsArgs, ok := args.(GetLogsArgs)
	if !ok {
		return fmt.Errorf("invalid arguments type: expected GetLogsArgs, got %T", args)
	}

	// Validate log level
	if logsArgs.Level != "" {
		validLevels := map[string]bool{
			"trace": true,
			"debug": true,
			"info":  true,
			"warn":  true,
			"error": true,
		}
		if !validLevels[logsArgs.Level] {
			return fmt.Errorf("invalid level: %s (valid values: trace, debug, info, warn, error)", logsArgs.Level)
		}
	}

	// Validate format
	if logsArgs.Format != "" {
		validFormats := map[string]bool{
			"json": true,
			"text": true,
		}
		if !validFormats[logsArgs.Format] {
			return fmt.Errorf("invalid format: %s (valid values: json, text)", logsArgs.Format)
		}
	}

	// Validate limit
	if logsArgs.Limit < 0 {
		return fmt.Errorf("limit cannot be negative")
	}
	if logsArgs.Limit > 10000 {
		return fmt.Errorf("limit cannot exceed 10,000 entries")
	}

	// Validate time range format if provided
	if logsArgs.TimeRange != "" {
		_, err := time.ParseDuration(logsArgs.TimeRange)
		if err != nil {
			return fmt.Errorf("invalid time_range format: %v (use duration format like '5m', '1h', '24h')", err)
		}
	}

	// Validate pattern length
	if len(logsArgs.Pattern) > 500 {
		return fmt.Errorf("pattern is too long (max 500 characters)")
	}

	// Validate log provider is available
	if t.logProvider == nil {
		return fmt.Errorf("log provider is not configured")
	}

	return nil
}
