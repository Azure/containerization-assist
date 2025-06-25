package types

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadObservabilityConfig_Success(t *testing.T) {
	// Create a temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-observability.yaml")

	configContent := `
version: "1.0"
last_updated: "2025-06-24"
opentelemetry:
  enabled: true
  service:
    name: "test-service"
    version: "1.0.0"
    environment: "test"
  resource:
    attributes:
      service.namespace: "mcp"
      deployment.environment: "test"
  tracing:
    enabled: true
    sampling:
      type: "probabilistic"
      rate: 0.1
    exporters:
      - type: "otlp"
        endpoint: "http://localhost:4317"
        enabled: true
        timeout: "30s"
    attributes:
      include_environment: true
      include_process_info: false
      include_host_info: true
  metrics:
    enabled: true
    exporters:
      - type: "prometheus"
        endpoint: "http://localhost:9090"
        enabled: true
        interval: "60s"
    custom_metrics:
      tool_execution_duration:
        enabled: true
        histogram_buckets: [0.1, 0.5, 1.0, 5.0, 10.0]
  logging:
    enabled: true
    exporters:
      - type: "console"
        enabled: true
    attributes:
      include_trace_context: true
      include_span_context: false
      include_source_location: true
slo:
  enabled: true
  tool_execution:
    availability:
      target: 99.9
      window: "24h"
    latency:
      target: 95.0
      threshold: "300ms"
      window: "1h"
    error_rate:
      target: 1.0
      window: "1h"
  session_management:
    availability:
      target: 99.95
      window: "24h"
alerting:
  enabled: true
  channels:
    - name: "slack"
      type: "webhook"
      webhook_url: "https://hooks.slack.com/services/..."
      enabled: true
    - name: "pagerduty"
      type: "pagerduty"
      integration_key: "test-key"
      enabled: false
  rules:
    - name: "high_error_rate"
      description: "Error rate above threshold"
      condition: "error_rate > 5%"
      severity: "critical"
      channels: ["slack", "pagerduty"]
dashboards:
  enabled: true
  grafana:
    enabled: true
    url: "http://localhost:3000"
    api_key: "test-api-key"
    definitions:
      - name: "MCP Overview"
        file: "dashboards/mcp-overview.json"
health_checks:
  enabled: true
  endpoints:
    liveness:
      path: "/health/live"
      port: 8080
    readiness:
      path: "/health/ready"
      port: 8080
  probes:
    - name: "database"
      type: "tcp"
      target: "localhost:5432"
      timeout: "5s"
      expected_status: 0
performance:
  profiling:
    enabled: true
    endpoint: "localhost:6060"
  sampling:
    type: "rate_limiting"
    rate: 10.0
  limits:
    max_concurrent_tools: 100
    max_session_duration: "24h"
    max_memory_usage: "2GB"
    cpu_profile_rate: 100
    memory_profile_rate: 512
`

	err := os.WriteFile(configPath, []byte(configContent), 0600)
	require.NoError(t, err)

	// Load the config
	config, err := LoadObservabilityConfig(configPath)
	require.NoError(t, err)
	assert.NotNil(t, config)

	// Verify basic fields
	assert.Equal(t, "1.0", config.Version)
	assert.Equal(t, "2025-06-24", config.LastUpdated)

	// Verify OpenTelemetry config
	assert.True(t, config.OpenTelemetry.Enabled)
	assert.Equal(t, "test-service", config.OpenTelemetry.Service.Name)
	assert.Equal(t, "1.0.0", config.OpenTelemetry.Service.Version)
	assert.Equal(t, "test", config.OpenTelemetry.Service.Environment)

	// Verify tracing config
	assert.True(t, config.OpenTelemetry.Tracing.Enabled)
	assert.Equal(t, "probabilistic", config.OpenTelemetry.Tracing.Sampling.Type)
	assert.Equal(t, 0.1, config.OpenTelemetry.Tracing.Sampling.Rate)
	assert.Len(t, config.OpenTelemetry.Tracing.Exporters, 1)
	assert.Equal(t, "otlp", config.OpenTelemetry.Tracing.Exporters[0].Type)
	assert.True(t, config.OpenTelemetry.Tracing.Exporters[0].Enabled)

	// Verify metrics config
	assert.True(t, config.OpenTelemetry.Metrics.Enabled)
	assert.Len(t, config.OpenTelemetry.Metrics.Exporters, 1)
	assert.Equal(t, "prometheus", config.OpenTelemetry.Metrics.Exporters[0].Type)
	assert.Contains(t, config.OpenTelemetry.Metrics.CustomMetrics, "tool_execution_duration")
	assert.True(t, config.OpenTelemetry.Metrics.CustomMetrics["tool_execution_duration"].Enabled)

	// Verify SLO config
	assert.True(t, config.SLO.Enabled)
	assert.Equal(t, 99.9, config.SLO.ToolExecution.Availability.Target)
	assert.Equal(t, "24h", config.SLO.ToolExecution.Availability.Window)

	// Verify alerting config
	assert.True(t, config.Alerting.Enabled)
	assert.Len(t, config.Alerting.Channels, 2)
	assert.Equal(t, "slack", config.Alerting.Channels[0].Name)
	assert.True(t, config.Alerting.Channels[0].Enabled)
	assert.Equal(t, "pagerduty", config.Alerting.Channels[1].Name)
	assert.False(t, config.Alerting.Channels[1].Enabled)

	// Verify health checks config
	assert.True(t, config.HealthChecks.Enabled)
	assert.Contains(t, config.HealthChecks.Endpoints, "liveness")
	assert.Equal(t, "/health/live", config.HealthChecks.Endpoints["liveness"].Path)
	assert.Equal(t, 8080, config.HealthChecks.Endpoints["liveness"].Port)

	// Verify performance config
	assert.True(t, config.Performance.Profiling.Enabled)
	assert.Equal(t, "localhost:6060", config.Performance.Profiling.Endpoint)
	assert.Equal(t, 100, config.Performance.Limits.MaxConcurrentTools)
}

func TestLoadObservabilityConfig_FileNotFound(t *testing.T) {
	config, err := LoadObservabilityConfig("/nonexistent/path/config.yaml")
	assert.Error(t, err)
	assert.Nil(t, config)
	assert.Contains(t, err.Error(), "failed to read observability config file")
}

func TestLoadObservabilityConfig_InvalidYAML(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "invalid.yaml")

	invalidContent := `
version: "1.0"
invalid: yaml: content: [
`

	err := os.WriteFile(configPath, []byte(invalidContent), 0600)
	require.NoError(t, err)

	config, err := LoadObservabilityConfig(configPath)
	assert.Error(t, err)
	assert.Nil(t, config)
	assert.Contains(t, err.Error(), "failed to parse observability config")
}

func TestLoadObservabilityConfig_EnvironmentVariables(t *testing.T) {
	// Set environment variables
	os.Setenv("TEST_SERVICE_NAME", "env-service")
	os.Setenv("TEST_ENDPOINT", "env-endpoint:8080")
	defer func() {
		os.Unsetenv("TEST_SERVICE_NAME")
		os.Unsetenv("TEST_ENDPOINT")
	}()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "env-test.yaml")

	configContent := `
version: "1.0"
opentelemetry:
  service:
    name: "${TEST_SERVICE_NAME}"
  tracing:
    exporters:
      - type: "otlp"
        endpoint: "http://${TEST_ENDPOINT}"
        enabled: true
`

	err := os.WriteFile(configPath, []byte(configContent), 0600)
	require.NoError(t, err)

	config, err := LoadObservabilityConfig(configPath)
	require.NoError(t, err)

	assert.Equal(t, "env-service", config.OpenTelemetry.Service.Name)
	assert.Equal(t, "http://env-endpoint:8080", config.OpenTelemetry.Tracing.Exporters[0].Endpoint)
}

func TestLoadObservabilityConfig_DefaultPath(t *testing.T) {
	// Test with empty config path (should default to "observability.yaml")
	// Save original working directory
	originalWd, err := os.Getwd()
	require.NoError(t, err)

	// Change to a directory without observability.yaml
	tempDir := t.TempDir()
	err = os.Chdir(tempDir)
	require.NoError(t, err)

	defer func() {
		os.Chdir(originalWd) // Restore original directory
	}()

	config, err := LoadObservabilityConfig("")
	// This should fail since there's no observability.yaml in the temp directory
	assert.Error(t, err)
	assert.Nil(t, config)
	assert.Contains(t, err.Error(), "failed to read observability config file")
}

func TestObservabilityConfig_GetTraceExporters(t *testing.T) {
	config := &ObservabilityConfig{
		OpenTelemetry: OpenTelemetryConfig{
			Tracing: TracingConfig{
				Exporters: []ExporterConfig{
					{Type: "otlp", Enabled: true},
					{Type: "jaeger", Enabled: false},
					{Type: "zipkin", Enabled: true},
				},
			},
		},
	}

	exporters := config.GetTraceExporters()
	assert.Len(t, exporters, 2)
	assert.Equal(t, "otlp", exporters[0].Type)
	assert.Equal(t, "zipkin", exporters[1].Type)
}

func TestObservabilityConfig_GetMetricExporters(t *testing.T) {
	config := &ObservabilityConfig{
		OpenTelemetry: OpenTelemetryConfig{
			Metrics: MetricsConfig{
				Exporters: []ExporterConfig{
					{Type: "prometheus", Enabled: true},
					{Type: "otlp", Enabled: false},
					{Type: "custom", Enabled: true},
				},
			},
		},
	}

	exporters := config.GetMetricExporters()
	assert.Len(t, exporters, 2)
	assert.Equal(t, "prometheus", exporters[0].Type)
	assert.Equal(t, "custom", exporters[1].Type)
}

func TestObservabilityConfig_GetAlertChannels(t *testing.T) {
	config := &ObservabilityConfig{
		Alerting: AlertingConfig{
			Channels: []AlertChannel{
				{Name: "slack", Enabled: true},
				{Name: "email", Enabled: false},
				{Name: "pagerduty", Enabled: true},
			},
		},
	}

	channels := config.GetAlertChannels()
	assert.Len(t, channels, 2)
	assert.Equal(t, "slack", channels[0].Name)
	assert.Equal(t, "pagerduty", channels[1].Name)
}

func TestSamplingConfig_GetSamplingTimeout(t *testing.T) {
	tests := []struct {
		name           string
		samplingType   string
		expectedMinDur time.Duration
		expectedMaxDur time.Duration
	}{
		{
			name:           "always_on",
			samplingType:   "always_on",
			expectedMinDur: time.Millisecond,
			expectedMaxDur: time.Millisecond,
		},
		{
			name:           "always_off",
			samplingType:   "always_off",
			expectedMinDur: time.Millisecond,
			expectedMaxDur: time.Millisecond,
		},
		{
			name:           "probabilistic",
			samplingType:   "probabilistic",
			expectedMinDur: 10 * time.Millisecond,
			expectedMaxDur: 10 * time.Millisecond,
		},
		{
			name:           "rate_limiting",
			samplingType:   "rate_limiting",
			expectedMinDur: 100 * time.Millisecond,
			expectedMaxDur: 100 * time.Millisecond,
		},
		{
			name:           "unknown",
			samplingType:   "unknown",
			expectedMinDur: 10 * time.Millisecond,
			expectedMaxDur: 10 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := SamplingConfig{Type: tt.samplingType}
			timeout := config.GetSamplingTimeout()
			assert.GreaterOrEqual(t, timeout, tt.expectedMinDur)
			assert.LessOrEqual(t, timeout, tt.expectedMaxDur)
		})
	}
}

func TestExporterConfig_GetExporterTimeout(t *testing.T) {
	tests := []struct {
		name            string
		timeout         string
		expectedTimeout time.Duration
	}{
		{
			name:            "valid_duration",
			timeout:         "45s",
			expectedTimeout: 45 * time.Second,
		},
		{
			name:            "empty_timeout",
			timeout:         "",
			expectedTimeout: 30 * time.Second,
		},
		{
			name:            "invalid_duration",
			timeout:         "invalid",
			expectedTimeout: 30 * time.Second,
		},
		{
			name:            "minutes",
			timeout:         "2m",
			expectedTimeout: 2 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := ExporterConfig{Timeout: tt.timeout}
			timeout := config.GetExporterTimeout()
			assert.Equal(t, tt.expectedTimeout, timeout)
		})
	}
}

func TestExporterConfig_GetExportInterval(t *testing.T) {
	tests := []struct {
		name             string
		interval         string
		expectedInterval time.Duration
	}{
		{
			name:             "valid_duration",
			interval:         "90s",
			expectedInterval: 90 * time.Second,
		},
		{
			name:             "empty_interval",
			interval:         "",
			expectedInterval: 60 * time.Second,
		},
		{
			name:             "invalid_duration",
			interval:         "invalid",
			expectedInterval: 60 * time.Second,
		},
		{
			name:             "minutes",
			interval:         "5m",
			expectedInterval: 5 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := ExporterConfig{Interval: tt.interval}
			interval := config.GetExportInterval()
			assert.Equal(t, tt.expectedInterval, interval)
		})
	}
}

func TestObservabilityConfig_EmptyConfig(t *testing.T) {
	config := &ObservabilityConfig{}

	// Test that methods work with empty config
	assert.Empty(t, config.GetTraceExporters())
	assert.Empty(t, config.GetMetricExporters())
	assert.Empty(t, config.GetAlertChannels())
}

func TestComplexConfigStructure(t *testing.T) {
	// Test a complex configuration to ensure all nested structures work
	config := ObservabilityConfig{
		Version:     "2.0",
		LastUpdated: "2025-06-24T10:00:00Z",
		OpenTelemetry: OpenTelemetryConfig{
			Enabled: true,
			Service: ServiceConfig{
				Name:        "complex-service",
				Version:     "2.1.0",
				Environment: "production",
			},
			Resource: ResourceConfig{
				Attributes: map[string]string{
					"service.namespace":      "mcp",
					"deployment.environment": "prod",
					"k8s.cluster.name":       "prod-cluster",
				},
			},
			Tracing: TracingConfig{
				Enabled: true,
				Sampling: SamplingConfig{
					Type: "probabilistic",
					Rate: 0.01,
				},
				Exporters: []ExporterConfig{
					{
						Type:     "otlp",
						Endpoint: "https://traces.example.com:4317",
						Headers: map[string]string{
							"authorization": "Bearer token123",
							"x-api-key":     "key456",
						},
						Timeout: "60s",
						Enabled: true,
					},
				},
				Attributes: AttributesConfig{
					IncludeEnvironment: true,
					IncludeProcessInfo: true,
					IncludeHostInfo:    false,
				},
			},
			Metrics: MetricsConfig{
				Enabled: true,
				Exporters: []ExporterConfig{
					{
						Type:     "prometheus",
						Endpoint: "http://prometheus:9090",
						Interval: "30s",
						Enabled:  true,
					},
				},
				CustomMetrics: map[string]CustomMetricConfig{
					"request_duration": {
						Enabled:          true,
						HistogramBuckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
					},
					"active_sessions": {
						Enabled:     true,
						GaugeLabels: []string{"environment", "region"},
					},
				},
			},
		},
		SLO: SLOConfig{
			Enabled: true,
			ToolExecution: SLOTargetConfig{
				Availability: AvailabilitySLO{
					Target: 99.95,
					Window: "7d",
				},
				Latency: LatencySLO{
					Target:    99.0,
					Threshold: "200ms",
					Window:    "1h",
				},
				ErrorRate: ErrorRateSLO{
					Target: 0.5,
					Window: "1h",
				},
			},
		},
	}

	// Verify complex nested access works
	assert.Equal(t, "complex-service", config.OpenTelemetry.Service.Name)
	assert.Equal(t, "prod-cluster", config.OpenTelemetry.Resource.Attributes["k8s.cluster.name"])
	assert.Equal(t, 0.01, config.OpenTelemetry.Tracing.Sampling.Rate)
	assert.Len(t, config.OpenTelemetry.Metrics.CustomMetrics["request_duration"].HistogramBuckets, 11)
	assert.Equal(t, 99.95, config.SLO.ToolExecution.Availability.Target)

	// Test methods on complex config
	traceExporters := config.GetTraceExporters()
	assert.Len(t, traceExporters, 1)
	assert.Equal(t, "Bearer token123", traceExporters[0].Headers["authorization"])

	metricExporters := config.GetMetricExporters()
	assert.Len(t, metricExporters, 1)
	assert.Equal(t, 30*time.Second, metricExporters[0].GetExportInterval())
}

// Benchmark tests
func BenchmarkLoadObservabilityConfig(b *testing.B) {
	tempDir := b.TempDir()
	configPath := filepath.Join(tempDir, "bench-config.yaml")

	configContent := `
version: "1.0"
opentelemetry:
  enabled: true
  service:
    name: "bench-service"
    version: "1.0.0"
  tracing:
    enabled: true
    sampling:
      type: "probabilistic"
      rate: 0.1
`

	err := os.WriteFile(configPath, []byte(configContent), 0600)
	if err != nil {
		b.Fatalf("Failed to create config file: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := LoadObservabilityConfig(configPath)
		if err != nil {
			b.Fatalf("Failed to load config: %v", err)
		}
	}
}

func BenchmarkGetTraceExporters(b *testing.B) {
	config := &ObservabilityConfig{
		OpenTelemetry: OpenTelemetryConfig{
			Tracing: TracingConfig{
				Exporters: []ExporterConfig{
					{Type: "otlp", Enabled: true},
					{Type: "jaeger", Enabled: false},
					{Type: "zipkin", Enabled: true},
					{Type: "stdout", Enabled: false},
					{Type: "custom", Enabled: true},
				},
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = config.GetTraceExporters()
	}
}
