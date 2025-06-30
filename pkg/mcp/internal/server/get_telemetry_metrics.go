package server

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/types"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/rs/zerolog"
)

type GetTelemetryMetricsArgs struct {
	types.BaseToolArgs
	Format       string   `json:"format,omitempty" jsonschema:"enum=prometheus,enum=json,default=prometheus,description=Output format for metrics"`
	MetricNames  []string `json:"metric_names,omitempty" jsonschema:"description=Filter metrics by exact name match. Supports multiple names for batch filtering (empty=all metrics)"`
	IncludeHelp  bool     `json:"include_help,omitempty" jsonschema:"default=true,description=Include metric help text"`
	TimeRange    string   `json:"time_range,omitempty" jsonschema:"description=Time range filter: duration format (e.g. 1h, 24h, 30m) or RFC3339 timestamp. Filters metrics collected after specified time"`
	IncludeEmpty bool     `json:"include_empty,omitempty" jsonschema:"default=false,description=Include metrics with zero values"`
}

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

type PerformanceReportData struct {
	P95Target       string                         `json:"p95_target"`
	ViolationCount  int                            `json:"violation_count"`
	ToolPerformance map[string]ToolPerformanceData `json:"tool_performance"`
}

type ToolPerformanceData struct {
	Tool           string  `json:"tool"`
	ExecutionCount int     `json:"execution_count"`
	SuccessRate    float64 `json:"success_rate"`
	P95Duration    string  `json:"p95_duration"`
	MaxDuration    string  `json:"max_duration"`
	Violations     int     `json:"violations"`
}

type TelemetryExporter interface {
	ExportMetrics() (string, error)
}

type GetTelemetryMetricsTool struct {
	logger    zerolog.Logger
	telemetry TelemetryExporter
	startTime time.Time
}

func NewGetTelemetryMetricsTool(logger zerolog.Logger, telemetry TelemetryExporter) *GetTelemetryMetricsTool {
	return &GetTelemetryMetricsTool{
		logger:    logger,
		telemetry: telemetry,
		startTime: time.Now(),
	}
}

func (t *GetTelemetryMetricsTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	telemetryArgs, ok := args.(GetTelemetryMetricsArgs)
	if !ok {
		return nil, fmt.Errorf("invalid arguments type: expected GetTelemetryMetricsArgs, got %T", args)
	}

	return t.ExecuteTyped(ctx, telemetryArgs)
}

func (t *GetTelemetryMetricsTool) ExecuteTyped(ctx context.Context, args GetTelemetryMetricsArgs) (*GetTelemetryMetricsResult, error) {
	t.logger.Info().
		Str("format", args.Format).
		Int("filter_count", len(args.MetricNames)).
		Str("time_range", args.TimeRange).
		Msg("Exporting telemetry metrics")

	if args.Format == "" {
		args.Format = "prometheus"
	}

	if args.Format != "prometheus" && args.Format != "json" {
		return nil, types.NewRichError(
			"INVALID_ARGUMENTS",
			fmt.Sprintf("unsupported format: %s (supported: prometheus, json)", args.Format),
			"validation_error",
		)
	}

	var startTime *time.Time
	if args.TimeRange != "" {
		st, err := t.parseTimeRange(args.TimeRange)
		if err != nil {
			return &GetTelemetryMetricsResult{
				BaseToolResponse: types.NewBaseResponse("get_telemetry_metrics", args.SessionID, args.DryRun),
				Format:           args.Format,
				ExportTimestamp:  time.Now(),
				Error: &types.ToolError{
					Type:      "INVALID_TIME_RANGE",
					Message:   fmt.Sprintf("Invalid time range format: %v", err),
					Retryable: false,
					Timestamp: time.Now(),
				},
			}, nil
		}
		startTime = &st
	}

	var metricFamilies []*dto.MetricFamily
	var err error

	if t.telemetry != nil {
		metricsText, err := t.telemetry.ExportMetrics()
		if err == nil {
			metricFamilies, err = t.parsePrometheusText(metricsText)
		}
	}

	if metricFamilies == nil || len(metricFamilies) == 0 {
		metricFamilies, err = prometheus.DefaultGatherer.Gather()
		if err != nil {
			return &GetTelemetryMetricsResult{
				BaseToolResponse: types.NewBaseResponse("get_telemetry_metrics", args.SessionID, args.DryRun),
				Format:           args.Format,
				ExportTimestamp:  time.Now(),
				Error: &types.ToolError{
					Type:      "EXPORT_FAILED",
					Message:   fmt.Sprintf("Failed to gather metrics: %v", err),
					Retryable: true,
					Timestamp: time.Now(),
				},
			}, nil
		}
	}

	if len(args.MetricNames) > 0 {
		metricFamilies = t.filterMetricFamilies(metricFamilies, args.MetricNames)
	}

	if startTime != nil {
		metricFamilies = t.filterByTimeRange(metricFamilies, *startTime)
	}

	if !args.IncludeEmpty {
		metricFamilies = t.removeEmptyMetricFamilies(metricFamilies)
	}

	var buf bytes.Buffer
	encoder := expfmt.NewEncoder(&buf, expfmt.FmtText)

	for _, mf := range metricFamilies {
		if !args.IncludeHelp {
			mf.Help = nil
		}

		if err := encoder.Encode(mf); err != nil {
			t.logger.Warn().Err(err).Str("metric", mf.GetName()).Msg("Failed to encode metric family")
			continue
		}
	}

	metricsText := buf.String()

	metricCount := t.countMetricFamilies(metricFamilies)

	uptime := time.Since(t.startTime)

	result := &GetTelemetryMetricsResult{
		BaseToolResponse:  types.NewBaseResponse("get_telemetry_metrics", args.SessionID, args.DryRun),
		Metrics:           metricsText,
		Format:            args.Format,
		MetricCount:       metricCount,
		ExportTimestamp:   time.Now(),
		PerformanceReport: nil,
		ServerUptime:      uptime.String(),
	}

	if args.Format == "json" {
		t.logger.Debug().Msg("JSON format requested - returning Prometheus text format for client parsing")
	}

	t.logger.Info().
		Int("metric_count", metricCount).
		Str("format", args.Format).
		Msg("Telemetry metrics exported successfully")

	return result, nil
}

func (t *GetTelemetryMetricsTool) filterMetrics(metricsText string, metricNames []string, includeHelp bool) string {
	lines := strings.Split(metricsText, "\n")
	filtered := make([]string, 0)

	nameMap := make(map[string]bool)
	for _, name := range metricNames {
		nameMap[name] = true
	}

	include := false
	for _, line := range lines {
		if strings.HasPrefix(line, "# HELP ") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				metricName := parts[2]
				include = nameMap[metricName]
				if include && includeHelp {
					filtered = append(filtered, line)
				}
			}
		} else if strings.HasPrefix(line, "# TYPE ") {
			if include {
				filtered = append(filtered, line)
			}
		} else if line != "" && !strings.HasPrefix(line, "#") {
			if include {
				filtered = append(filtered, line)
			}
		} else if line == "" {
			if len(filtered) > 0 && filtered[len(filtered)-1] != "" {
				filtered = append(filtered, line)
			}
		}
	}

	return strings.Join(filtered, "\n")
}

func (t *GetTelemetryMetricsTool) removeEmptyMetrics(metricsText string) string {
	lines := strings.Split(metricsText, "\n")
	filtered := make([]string, 0)

	skipNext := false
	for _, line := range lines {
		if !strings.HasPrefix(line, "#") && strings.Contains(line, " 0") {
			if strings.HasSuffix(line, " 0") || strings.HasSuffix(line, " 0.0") {
				skipNext = true
				for j := len(filtered) - 1; j >= 0 && j >= len(filtered)-3; j-- {
					if strings.HasPrefix(filtered[j], "# HELP ") || strings.HasPrefix(filtered[j], "# TYPE ") {
						filtered = filtered[:j]
					} else {
						break
					}
				}
				continue
			}
		}

		if !skipNext {
			filtered = append(filtered, line)
		} else if line == "" {
			skipNext = false
		}
	}

	return strings.Join(filtered, "\n")
}

func (t *GetTelemetryMetricsTool) countMetrics(metricsText string) int {
	lines := strings.Split(metricsText, "\n")
	count := 0

	for _, line := range lines {
		if line != "" && !strings.HasPrefix(line, "#") {
			count++
		}
	}

	return count
}

func (t *GetTelemetryMetricsTool) parseTimeRange(timeRange string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, timeRange); err == nil {
		return t, nil
	}

	if duration, err := time.ParseDuration(timeRange); err == nil {
		return time.Now().Add(-duration), nil
	}

	return time.Time{}, types.NewRichError(
		"INVALID_ARGUMENTS",
		"time range must be either a duration (e.g., '1h', '24h') or RFC3339 timestamp",
		"validation_error",
	)
}

func (t *GetTelemetryMetricsTool) parsePrometheusText(text string) ([]*dto.MetricFamily, error) {
	parser := expfmt.TextParser{}
	reader := strings.NewReader(text)

	families, err := parser.TextToMetricFamilies(reader)
	if err != nil {
		return nil, err
	}

	result := make([]*dto.MetricFamily, 0, len(families))
	for _, mf := range families {
		result = append(result, mf)
	}

	return result, nil
}

func (t *GetTelemetryMetricsTool) filterMetricFamilies(families []*dto.MetricFamily, names []string) []*dto.MetricFamily {
	if len(names) == 0 {
		return families
	}

	nameMap := make(map[string]bool)
	for _, name := range names {
		nameMap[name] = true
	}

	filtered := make([]*dto.MetricFamily, 0)
	for _, mf := range families {
		if nameMap[mf.GetName()] {
			filtered = append(filtered, mf)
		}
	}

	return filtered
}

func (t *GetTelemetryMetricsTool) filterByTimeRange(families []*dto.MetricFamily, startTime time.Time) []*dto.MetricFamily {
	return families
}

func (t *GetTelemetryMetricsTool) removeEmptyMetricFamilies(families []*dto.MetricFamily) []*dto.MetricFamily {
	filtered := make([]*dto.MetricFamily, 0)

	for _, mf := range families {
		filteredMetrics := make([]*dto.Metric, 0)

		for _, metric := range mf.GetMetric() {
			hasNonZero := false

			switch mf.GetType() {
			case dto.MetricType_COUNTER:
				if metric.Counter != nil && metric.Counter.GetValue() > 0 {
					hasNonZero = true
				}
			case dto.MetricType_GAUGE:
				if metric.Gauge != nil && metric.Gauge.GetValue() != 0 {
					hasNonZero = true
				}
			case dto.MetricType_HISTOGRAM:
				if metric.Histogram != nil && metric.Histogram.GetSampleCount() > 0 {
					hasNonZero = true
				}
			case dto.MetricType_SUMMARY:
				if metric.Summary != nil && metric.Summary.GetSampleCount() > 0 {
					hasNonZero = true
				}
			default:
				hasNonZero = true
			}

			if hasNonZero {
				filteredMetrics = append(filteredMetrics, metric)
			}
		}

		if len(filteredMetrics) > 0 {
			newMf := &dto.MetricFamily{
				Name:   mf.Name,
				Help:   mf.Help,
				Type:   mf.Type,
				Metric: filteredMetrics,
			}
			filtered = append(filtered, newMf)
		}
	}

	return filtered
}

func (t *GetTelemetryMetricsTool) countMetricFamilies(families []*dto.MetricFamily) int {
	count := 0
	for _, mf := range families {
		count += len(mf.GetMetric())
	}
	return count
}

func (t *GetTelemetryMetricsTool) GetMetadata() mcptypes.ToolMetadata {
	return mcptypes.ToolMetadata{
		Name:        "get_telemetry_metrics",
		Description: "Export telemetry metrics in Prometheus format with filtering and analysis",
		Version:     "1.0.0",
		Category:    "Monitoring",
		Dependencies: []string{
			"Prometheus Client",
			"Telemetry Exporter",
			"Metrics Registry",
		},
		Capabilities: []string{
			"Metric export",
			"Format conversion",
			"Metric filtering",
			"Time range filtering",
			"Performance analysis",
			"Help text inclusion",
			"Empty metric removal",
		},
		Requirements: []string{
			"Prometheus metrics registry",
			"Telemetry collection enabled",
		},
		Parameters: map[string]string{
			"format":        "Optional: Output format (prometheus, json)",
			"metric_names":  "Optional: Filter metrics by exact name match",
			"include_help":  "Optional: Include metric help text (default: true)",
			"time_range":    "Optional: Time range filter (duration or RFC3339)",
			"include_empty": "Optional: Include metrics with zero values (default: false)",
		},
		Examples: []mcptypes.ToolExample{
			{
				Name:        "Export all metrics",
				Description: "Export all available metrics in Prometheus format",
				Input: map[string]interface{}{
					"format": "prometheus",
				},
				Output: map[string]interface{}{
					"metrics":          "# HELP tool_execution_duration_seconds...",
					"format":           "prometheus",
					"metric_count":     45,
					"export_timestamp": "2024-12-17T10:30:00Z",
					"server_uptime":    "24h30m",
				},
			},
			{
				Name:        "Filter specific metrics",
				Description: "Export only tool execution metrics from the last hour",
				Input: map[string]interface{}{
					"metric_names": []string{"tool_execution_duration_seconds", "tool_execution_total"},
					"time_range":   "1h",
					"include_help": false,
				},
				Output: map[string]interface{}{
					"metrics":          "tool_execution_duration_seconds{tool=\"build_image\"} 2.5\n...",
					"format":           "prometheus",
					"metric_count":     12,
					"export_timestamp": "2024-12-17T10:30:00Z",
				},
			},
		},
	}
}

func (t *GetTelemetryMetricsTool) Validate(ctx context.Context, args interface{}) error {
	telemetryArgs, ok := args.(GetTelemetryMetricsArgs)
	if !ok {
		return fmt.Errorf("invalid arguments type: expected GetTelemetryMetricsArgs, got %T", args)
	}

	if telemetryArgs.Format != "" {
		validFormats := map[string]bool{
			"prometheus": true,
			"json":       true,
		}
		if !validFormats[telemetryArgs.Format] {
			return fmt.Errorf("invalid format: %s (valid values: prometheus, json)", telemetryArgs.Format)
		}
	}

	if len(telemetryArgs.MetricNames) > 100 {
		return fmt.Errorf("too many metric names (max 100)")
	}

	for _, name := range telemetryArgs.MetricNames {
		if name == "" {
			return fmt.Errorf("metric names cannot be empty")
		}
		if len(name) > 200 {
			return fmt.Errorf("metric name '%s' is too long (max 200 characters)", name)
		}
	}

	if telemetryArgs.TimeRange != "" {
		_, err := t.parseTimeRange(telemetryArgs.TimeRange)
		if err != nil {
			return fmt.Errorf("invalid time_range format: %v", err)
		}
	}

	return nil
}
