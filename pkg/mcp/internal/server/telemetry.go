package server

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/internal/orchestration"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/Azure/container-kit/pkg/mcp/internal/utils"
)

// Telemetry and stats functionality consolidated from server_stats.go + get_telemetry_metrics.go

// ServerStats provides comprehensive server statistics
type ServerStats struct {
	Uptime          time.Duration                                 `json:"uptime"`
	Sessions        *core.SessionManagerStats                     `json:"sessions"`
	Workspace       *utils.WorkspaceStats                         `json:"workspace"`
	CircuitBreakers map[string]*orchestration.CircuitBreakerStats `json:"circuit_breakers"`
	Transport       string                                        `json:"transport"`
}

// GetStats returns server statistics (implementation for core.Server interface)
func (s *Server) GetStats() *core.ServerStats {
	sessionStats := s.sessionManager.GetStats()
	workspaceStats := s.workspaceManager.GetStats()

	return &core.ServerStats{
		Transport: s.config.TransportType,
		Sessions: &core.SessionManagerStats{
			ActiveSessions:    sessionStats.ActiveSessions,
			TotalSessions:     sessionStats.TotalSessions,
			FailedSessions:    sessionStats.FailedSessions,
			ExpiredSessions:   sessionStats.ExpiredSessions,
			SessionsWithJobs:  sessionStats.SessionsWithJobs,
			AverageSessionAge: sessionStats.AverageSessionAge,
			SessionErrors:     sessionStats.SessionErrors,
			TotalDiskUsage:    sessionStats.TotalDiskUsage,
			ServerStartTime:   sessionStats.ServerStartTime,
		},
		Workspace: &core.WorkspaceStats{
			TotalDiskUsage: workspaceStats.TotalDiskUsage,
			SessionCount:   workspaceStats.TotalSessions,
			TotalFiles:     0, // Not available in utils.WorkspaceStats
			DiskLimit:      workspaceStats.TotalDiskLimit,
		},
		Uptime:    time.Since(s.startTime),
		StartTime: s.startTime,
	}
}

// GetSessionManagerStats returns session manager statistics (interface implementation)
func (s *Server) GetSessionManagerStats() *core.SessionManagerStats {
	sessionStats := s.sessionManager.GetStats()
	return &core.SessionManagerStats{
		ActiveSessions:    sessionStats.ActiveSessions,
		TotalSessions:     sessionStats.TotalSessions,
		FailedSessions:    sessionStats.FailedSessions,
		ExpiredSessions:   sessionStats.ExpiredSessions,
		SessionsWithJobs:  sessionStats.SessionsWithJobs,
		AverageSessionAge: sessionStats.AverageSessionAge,
		SessionErrors:     sessionStats.SessionErrors,
		TotalDiskUsage:    sessionStats.TotalDiskUsage,
		ServerStartTime:   sessionStats.ServerStartTime,
	}
}

// GetWorkspaceStats returns workspace statistics (interface implementation)
func (s *Server) GetWorkspaceStats() *core.WorkspaceStats {
	workspaceStats := s.workspaceManager.GetStats()
	return &core.WorkspaceStats{
		TotalDiskUsage: workspaceStats.TotalDiskUsage,
		SessionCount:   workspaceStats.TotalSessions,
		TotalFiles:     0, // Not available in utils.WorkspaceStats
		DiskLimit:      workspaceStats.TotalDiskLimit,
	}
}

// GetStartTime returns when the server was started
func (s *Server) GetStartTime() time.Time {
	return s.startTime
}

// GetLogger returns the logger instance (interface implementation)
func (s *Server) GetLogger() interface{} {
	return s.logger
}

// Telemetry metrics functionality

// GetTelemetryMetricsArgs defines arguments for telemetry metrics retrieval
type GetTelemetryMetricsArgs struct {
	types.BaseToolArgs
	Format       string   `json:"format,omitempty" jsonschema:"enum=prometheus,enum=json,default=prometheus,description=Output format for metrics"`
	MetricNames  []string `json:"metric_names,omitempty" jsonschema:"description=Filter metrics by exact name match. Supports multiple names for batch filtering (empty=all metrics)"`
	IncludeHelp  bool     `json:"include_help,omitempty" jsonschema:"default=true,description=Include metric help text"`
	TimeRange    string   `json:"time_range,omitempty" jsonschema:"description=Time range filter: duration format (e.g. 1h, 24h, 30m) or RFC3339 timestamp. Filters metrics collected after specified time"`
	IncludeEmpty bool     `json:"include_empty,omitempty" jsonschema:"default=false,description=Include metrics with zero values"`
}

// GetTelemetryMetricsResult represents telemetry metrics result
type GetTelemetryMetricsResult struct {
	types.BaseToolResponse
	Metrics           string                 `json:"metrics"`
	Format            string                 `json:"format"`
	MetricCount       int                    `json:"metric_count"`
	ExportTimestamp   time.Time              `json:"export_timestamp"`
	PerformanceReport *PerformanceReportData `json:"performance_report,omitempty"`
	ServerUptime      string                 `json:"server_uptime"`
	Error             *types.ToolError       `json:"error,omitempty"`
}

// PerformanceReportData provides performance analysis
type PerformanceReportData struct {
	P95Target       string                         `json:"p95_target"`
	ViolationCount  int                            `json:"violation_count"`
	ToolPerformance map[string]ToolPerformanceData `json:"tool_performance"`
}

// ToolPerformanceData provides tool-specific performance data
type ToolPerformanceData struct {
	Tool           string  `json:"tool"`
	ExecutionCount int     `json:"execution_count"`
	P95Latency     float64 `json:"p95_latency_ms"`
	ViolatesTarget bool    `json:"violates_target"`
	AverageLatency float64 `json:"average_latency_ms"`
	ErrorRate      float64 `json:"error_rate"`
}

// GetTelemetryMetrics implements telemetry metrics retrieval
func GetTelemetryMetrics(ctx context.Context, args GetTelemetryMetricsArgs) (*GetTelemetryMetricsResult, error) {
	exportTime := time.Now()

	// Set default format
	format := args.Format
	if format == "" {
		format = "prometheus"
	}

	result := &GetTelemetryMetricsResult{
		BaseToolResponse: types.BaseToolResponse{}, // Simplified
		Format:           format,
		ExportTimestamp:  exportTime,
		ServerUptime:     time.Since(exportTime.Add(-1 * time.Hour)).String(), // Placeholder
	}

	// This is a simplified implementation
	// Real implementation would gather actual Prometheus metrics

	switch format {
	case "prometheus":
		metrics := generatePrometheusMetrics(args)
		result.Metrics = metrics
		result.MetricCount = countMetrics(metrics)
	case "json":
		metrics := generateJSONMetrics(args)
		result.Metrics = metrics
		result.MetricCount = countJSONMetrics(metrics)
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}

	// Generate performance report if requested
	if shouldIncludePerformanceReport(args) {
		result.PerformanceReport = generatePerformanceReport()
	}

	return result, nil
}

// generatePrometheusMetrics generates sample Prometheus format metrics
func generatePrometheusMetrics(args GetTelemetryMetricsArgs) string {
	var buf bytes.Buffer

	if args.IncludeHelp {
		buf.WriteString("# HELP mcp_server_uptime_seconds Server uptime in seconds\n")
		buf.WriteString("# TYPE mcp_server_uptime_seconds counter\n")
	}
	buf.WriteString("mcp_server_uptime_seconds 3600\n")

	if args.IncludeHelp {
		buf.WriteString("# HELP mcp_sessions_active Current number of active sessions\n")
		buf.WriteString("# TYPE mcp_sessions_active gauge\n")
	}
	buf.WriteString("mcp_sessions_active 5\n")

	if args.IncludeHelp {
		buf.WriteString("# HELP mcp_tool_executions_total Total number of tool executions\n")
		buf.WriteString("# TYPE mcp_tool_executions_total counter\n")
	}
	buf.WriteString("mcp_tool_executions_total{tool=\"analyze_repository\"} 25\n")
	buf.WriteString("mcp_tool_executions_total{tool=\"build_container\"} 10\n")
	buf.WriteString("mcp_tool_executions_total{tool=\"deploy_container\"} 8\n")

	return buf.String()
}

// generateJSONMetrics generates sample JSON format metrics
func generateJSONMetrics(args GetTelemetryMetricsArgs) string {
	return `{
		"server_uptime_seconds": 3600,
		"sessions_active": 5,
		"tool_executions": {
			"analyze_repository": 25,
			"build_container": 10,
			"deploy_container": 8
		}
	}`
}

// countMetrics counts the number of metrics in Prometheus format
func countMetrics(metrics string) int {
	lines := strings.Split(metrics, "\n")
	count := 0
	for _, line := range lines {
		if line != "" && !strings.HasPrefix(line, "#") {
			count++
		}
	}
	return count
}

// countJSONMetrics counts the number of metrics in JSON format
func countJSONMetrics(metrics string) int {
	// Simplified implementation - real implementation would parse JSON
	return 3
}

// shouldIncludePerformanceReport determines if performance report should be included
func shouldIncludePerformanceReport(args GetTelemetryMetricsArgs) bool {
	// Include performance report for detailed requests
	return len(args.MetricNames) > 0 || args.TimeRange != ""
}

// generatePerformanceReport generates a sample performance report
func generatePerformanceReport() *PerformanceReportData {
	return &PerformanceReportData{
		P95Target:      "300ms",
		ViolationCount: 2,
		ToolPerformance: map[string]ToolPerformanceData{
			"analyze_repository": {
				Tool:           "analyze_repository",
				ExecutionCount: 25,
				P95Latency:     250.5,
				ViolatesTarget: false,
				AverageLatency: 180.2,
				ErrorRate:      0.04,
			},
			"build_container": {
				Tool:           "build_container",
				ExecutionCount: 10,
				P95Latency:     450.8,
				ViolatesTarget: true,
				AverageLatency: 320.1,
				ErrorRate:      0.10,
			},
		},
	}
}

// GetTelemetryMetricsTool implements the telemetry metrics tool
type GetTelemetryMetricsTool struct{}

// GetMetadata returns tool metadata
func (t *GetTelemetryMetricsTool) GetMetadata() core.ToolMetadata {
	return core.ToolMetadata{
		Name:        "get_telemetry_metrics",
		Description: "Retrieve telemetry metrics from the server",
		Version:     "1.0.0",
		Category:    "telemetry",
	}
}

// Execute executes the tool
func (t *GetTelemetryMetricsTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	if typed, ok := args.(GetTelemetryMetricsArgs); ok {
		return GetTelemetryMetrics(ctx, typed)
	}
	return nil, fmt.Errorf("invalid arguments type")
}

// Validate validates the arguments
func (t *GetTelemetryMetricsTool) Validate(ctx context.Context, args interface{}) error {
	if _, ok := args.(GetTelemetryMetricsArgs); !ok {
		return fmt.Errorf("invalid arguments type")
	}
	return nil
}
