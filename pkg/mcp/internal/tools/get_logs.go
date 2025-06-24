package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	"github.com/Azure/container-copilot/pkg/mcp/internal/utils"
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

// LogProvider interface for accessing logs
type LogProvider interface {
	GetLogs(level string, since time.Time, pattern string, limit int) ([]utils.LogEntry, error)
	GetTotalLogCount() int
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
	logProvider LogProvider
}

// NewGetLogsTool creates a new get logs tool
func NewGetLogsTool(logger zerolog.Logger, logProvider LogProvider) *GetLogsTool {
	return &GetLogsTool{
		logger:      logger,
		logProvider: logProvider,
	}
}

// Execute retrieves server logs based on the provided filters
func (t *GetLogsTool) Execute(ctx context.Context, args GetLogsArgs) (*GetLogsResult, error) {
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
func CreateGlobalLogProvider() LogProvider {
	buffer := utils.GetGlobalLogBuffer()
	if buffer == nil {
		// Initialize if not already done
		utils.InitializeLogCapture(10000) // 10k log entries
		buffer = utils.GetGlobalLogBuffer()
	}
	return NewRingBufferLogProvider(buffer)
}
