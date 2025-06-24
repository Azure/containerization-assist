package tools

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockTelemetryExporter for testing
type MockTelemetryExporter struct {
	metrics     string
	exportError error
}

func NewMockTelemetryExporter() *MockTelemetryExporter {
	return &MockTelemetryExporter{
		metrics: `# HELP mcp_tool_executions_total Total number of tool executions
# TYPE mcp_tool_executions_total counter
mcp_tool_executions_total{dry_run="false",status="success",tool="analyze_repository"} 5
mcp_tool_executions_total{dry_run="false",status="failure",tool="build_image"} 2
mcp_tool_executions_total{dry_run="true",status="success",tool="generate_dockerfile"} 3

# HELP mcp_tool_duration_seconds Tool execution duration in seconds
# TYPE mcp_tool_duration_seconds histogram
mcp_tool_duration_seconds_bucket{dry_run="false",status="success",tool="analyze_repository",le="0.1"} 2
mcp_tool_duration_seconds_bucket{dry_run="false",status="success",tool="analyze_repository",le="0.5"} 4
mcp_tool_duration_seconds_bucket{dry_run="false",status="success",tool="analyze_repository",le="1"} 5
mcp_tool_duration_seconds_bucket{dry_run="false",status="success",tool="analyze_repository",le="+Inf"} 5
mcp_tool_duration_seconds_sum{dry_run="false",status="success",tool="analyze_repository"} 2.5
mcp_tool_duration_seconds_count{dry_run="false",status="success",tool="analyze_repository"} 5

# HELP mcp_active_sessions Number of currently active sessions
# TYPE mcp_active_sessions gauge
mcp_active_sessions 3

# HELP llm_prompt_tokens_total Total number of prompt tokens used
# TYPE llm_prompt_tokens_total counter
llm_prompt_tokens_total{model="gpt-4",tool="chat"} 1500
llm_prompt_tokens_total{model="claude-3",tool="analyze_repository"} 0

# HELP mcp_tokens_used_total Total tokens used by tool (legacy)
# TYPE mcp_tokens_used_total counter
mcp_tokens_used_total{tool="chat"} 0
`,
	}
}

func (m *MockTelemetryExporter) ExportMetrics() (string, error) {
	if m.exportError != nil {
		return "", m.exportError
	}
	return m.metrics, nil
}

func TestGetTelemetryMetricsTool_Execute(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("export all metrics in prometheus format", func(t *testing.T) {
		// Setup
		telemetry := NewMockTelemetryExporter()
		tool := NewGetTelemetryMetricsTool(logger, telemetry)

		// Execute
		args := GetTelemetryMetricsArgs{
			Format: "prometheus",
		}
		result, err := tool.Execute(context.Background(), args)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "prometheus", result.Format)
		assert.Contains(t, result.Metrics, "mcp_tool_executions_total")
		assert.Contains(t, result.Metrics, "mcp_active_sessions")
		assert.Contains(t, result.Metrics, "llm_prompt_tokens_total")
		assert.Greater(t, result.MetricCount, 0)
		// Check that uptime is not empty and is a valid duration string
		assert.NotEmpty(t, result.ServerUptime)
		// Ensure it can be parsed as a duration
		_, err = time.ParseDuration(result.ServerUptime)
		assert.NoError(t, err, "ServerUptime should be a valid duration string")
	})

	t.Run("filter specific metrics", func(t *testing.T) {
		// Setup
		telemetry := NewMockTelemetryExporter()
		tool := NewGetTelemetryMetricsTool(logger, telemetry)

		// Execute
		args := GetTelemetryMetricsArgs{
			Format:      "prometheus",
			MetricNames: []string{"mcp_active_sessions", "llm_prompt_tokens_total"},
			IncludeHelp: true,
		}
		result, err := tool.Execute(context.Background(), args)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Contains(t, result.Metrics, "mcp_active_sessions")
		assert.Contains(t, result.Metrics, "llm_prompt_tokens_total")
		assert.NotContains(t, result.Metrics, "mcp_tool_executions_total")
		assert.NotContains(t, result.Metrics, "mcp_tool_duration_seconds")
	})

	t.Run("remove empty metrics", func(t *testing.T) {
		// Setup
		telemetry := NewMockTelemetryExporter()
		tool := NewGetTelemetryMetricsTool(logger, telemetry)

		// Execute
		args := GetTelemetryMetricsArgs{
			Format:       "prometheus",
			IncludeEmpty: false,
		}
		result, err := tool.Execute(context.Background(), args)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		// Should remove metrics with value 0
		assert.NotContains(t, result.Metrics, "llm_prompt_tokens_total{model=\"claude-3\",tool=\"analyze_repository\"} 0")
		assert.NotContains(t, result.Metrics, "mcp_tokens_used_total{tool=\"chat\"} 0")
		// Should keep non-zero metrics
		assert.Contains(t, result.Metrics, "llm_prompt_tokens_total{model=\"gpt-4\",tool=\"chat\"} 1500")
	})

	t.Run("export error handling", func(t *testing.T) {
		// Setup
		telemetry := &MockTelemetryExporter{
			exportError: fmt.Errorf("failed to gather metrics"),
		}
		tool := NewGetTelemetryMetricsTool(logger, telemetry)

		// Execute
		args := GetTelemetryMetricsArgs{}
		result, err := tool.Execute(context.Background(), args)

		// Assert
		require.NoError(t, err) // Execute returns error in result, not as error
		assert.NotNil(t, result)
		// When telemetry export fails, it falls back to DefaultGatherer
		assert.Nil(t, result.Error)
		// Should still have metrics from DefaultGatherer fallback
		assert.NotEmpty(t, result.Metrics)
	})

	t.Run("invalid format", func(t *testing.T) {
		// Setup
		telemetry := NewMockTelemetryExporter()
		tool := NewGetTelemetryMetricsTool(logger, telemetry)

		// Execute
		args := GetTelemetryMetricsArgs{
			Format: "invalid",
		}
		_, err := tool.Execute(context.Background(), args)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported format: invalid")
	})

	t.Run("count metrics correctly", func(t *testing.T) {
		// Setup
		telemetry := NewMockTelemetryExporter()
		tool := NewGetTelemetryMetricsTool(logger, telemetry)

		// Execute
		args := GetTelemetryMetricsArgs{
			Format: "prometheus",
		}
		result, err := tool.Execute(context.Background(), args)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)

		// MetricCount should represent the number of metric samples, not text lines
		// This is implementation-specific but should be consistent
		assert.Greater(t, result.MetricCount, 0, "Should have at least one metric")

		// Verify the metrics text has actual content
		lines := strings.Split(result.Metrics, "\n")
		metricLines := 0
		for _, line := range lines {
			if line != "" && !strings.HasPrefix(line, "#") {
				metricLines++
			}
		}
		assert.Greater(t, metricLines, 0, "Should have metric lines in output")
	})
}

func TestGetTelemetryMetricsTool_TimeRangeParsing(t *testing.T) {
	logger := zerolog.Nop()
	telemetry := NewMockTelemetryExporter()
	tool := NewGetTelemetryMetricsTool(logger, telemetry)

	tests := []struct {
		name           string
		timeRange      string
		wantError      bool
		errorType      string
		validateResult func(t *testing.T, result *GetTelemetryMetricsResult)
	}{
		{
			name:      "valid duration - 1 hour",
			timeRange: "1h",
			wantError: false,
			validateResult: func(t *testing.T, result *GetTelemetryMetricsResult) {
				assert.Nil(t, result.Error)
				assert.Greater(t, result.MetricCount, 0)
			},
		},
		{
			name:      "valid duration - 24 hours",
			timeRange: "24h",
			wantError: false,
			validateResult: func(t *testing.T, result *GetTelemetryMetricsResult) {
				assert.Nil(t, result.Error)
				assert.Greater(t, result.MetricCount, 0)
			},
		},
		{
			name:      "valid RFC3339 timestamp",
			timeRange: "2025-01-01T00:00:00Z",
			wantError: false,
			validateResult: func(t *testing.T, result *GetTelemetryMetricsResult) {
				assert.Nil(t, result.Error)
				assert.Greater(t, result.MetricCount, 0)
			},
		},
		{
			name:      "invalid time range format",
			timeRange: "invalid-format",
			wantError: false, // Execute doesn't return error, but result contains error
			errorType: "INVALID_TIME_RANGE",
			validateResult: func(t *testing.T, result *GetTelemetryMetricsResult) {
				assert.NotNil(t, result.Error)
				assert.Equal(t, "INVALID_TIME_RANGE", result.Error.Type)
				assert.Contains(t, result.Error.Message, "Invalid time range format")
			},
		},
		{
			name:      "empty time range",
			timeRange: "",
			wantError: false,
			validateResult: func(t *testing.T, result *GetTelemetryMetricsResult) {
				assert.Nil(t, result.Error)
				assert.Greater(t, result.MetricCount, 0)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := GetTelemetryMetricsArgs{
				Format:    "prometheus",
				TimeRange: tt.timeRange,
			}

			result, err := tool.Execute(context.Background(), args)

			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				if tt.validateResult != nil {
					tt.validateResult(t, result)
				}
			}
		})
	}
}

func TestGetTelemetryMetricsTool_MetricFiltering(t *testing.T) {
	logger := zerolog.Nop()
	telemetry := NewMockTelemetryExporter()
	tool := NewGetTelemetryMetricsTool(logger, telemetry)

	tests := []struct {
		name             string
		metricNames      []string
		includeEmpty     bool
		expectedCount    int
		shouldContain    []string
		shouldNotContain []string
	}{
		{
			name:             "filter single metric",
			metricNames:      []string{"mcp_active_sessions"},
			includeEmpty:     true,
			expectedCount:    1,
			shouldContain:    []string{"mcp_active_sessions"},
			shouldNotContain: []string{"mcp_tool_executions_total", "llm_prompt_tokens_total"},
		},
		{
			name:             "filter multiple metrics",
			metricNames:      []string{"mcp_active_sessions", "llm_prompt_tokens_total"},
			includeEmpty:     true,
			expectedCount:    3, // 1 gauge + 2 counter entries
			shouldContain:    []string{"mcp_active_sessions", "llm_prompt_tokens_total"},
			shouldNotContain: []string{"mcp_tool_executions_total"},
		},
		{
			name:             "filter with exclude empty",
			metricNames:      []string{"llm_prompt_tokens_total", "mcp_tokens_used_total"},
			includeEmpty:     false,
			expectedCount:    1, // Only llm_prompt_tokens_total with non-zero value
			shouldContain:    []string{"llm_prompt_tokens_total{model=\"gpt-4\",tool=\"chat\"}"},
			shouldNotContain: []string{"llm_prompt_tokens_total{model=\"claude-3\",tool=\"analyze_repository\"}", "mcp_tokens_used_total"},
		},
		{
			name:          "no filter - all metrics",
			metricNames:   []string{},
			includeEmpty:  true,
			expectedCount: 8, // Count all metric samples in mock data
			shouldContain: []string{"mcp_tool_executions_total", "mcp_active_sessions", "llm_prompt_tokens_total"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := GetTelemetryMetricsArgs{
				Format:       "prometheus",
				MetricNames:  tt.metricNames,
				IncludeEmpty: tt.includeEmpty,
				IncludeHelp:  false,
			}

			result, err := tool.Execute(context.Background(), args)
			require.NoError(t, err)
			require.NotNil(t, result)

			// Check metric count
			assert.Equal(t, tt.expectedCount, result.MetricCount, "Unexpected metric count")

			// Check contains
			for _, expected := range tt.shouldContain {
				assert.Contains(t, result.Metrics, expected, "Expected metric not found: %s", expected)
			}

			// Check not contains
			for _, unexpected := range tt.shouldNotContain {
				assert.NotContains(t, result.Metrics, unexpected, "Unexpected metric found: %s", unexpected)
			}
		})
	}
}

func TestGetTelemetryMetricsTool_EdgeCases(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("handle export error gracefully", func(t *testing.T) {
		telemetry := &MockTelemetryExporter{
			exportError: fmt.Errorf("telemetry system unavailable"),
		}
		tool := NewGetTelemetryMetricsTool(logger, telemetry)

		args := GetTelemetryMetricsArgs{
			Format: "prometheus",
		}

		result, err := tool.Execute(context.Background(), args)

		// Should not return error, falls back to DefaultGatherer
		assert.NoError(t, err)
		assert.NotNil(t, result)
		// Falls back to DefaultGatherer when telemetry export fails
		assert.Nil(t, result.Error)
		assert.NotEmpty(t, result.Metrics)
	})

	t.Run("handle nil telemetry exporter", func(t *testing.T) {
		tool := NewGetTelemetryMetricsTool(logger, nil)

		args := GetTelemetryMetricsArgs{
			Format: "prometheus",
		}

		// Should use prometheus.DefaultGatherer as fallback
		result, err := tool.Execute(context.Background(), args)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		// Will likely have some default Go runtime metrics
		assert.NotEmpty(t, result.Metrics)
	})

	t.Run("json format request", func(t *testing.T) {
		telemetry := NewMockTelemetryExporter()
		tool := NewGetTelemetryMetricsTool(logger, telemetry)

		args := GetTelemetryMetricsArgs{
			Format: "json",
		}

		result, err := tool.Execute(context.Background(), args)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "json", result.Format)
		// Currently returns prometheus format even for JSON
		assert.Contains(t, result.Metrics, "# TYPE")
	})
}
