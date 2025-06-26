package types

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// ObservabilityConfig represents the complete observability configuration
type ObservabilityConfig struct {
	Version       string              `yaml:"version"`
	LastUpdated   string              `yaml:"last_updated"`
	OpenTelemetry OpenTelemetryConfig `yaml:"opentelemetry"`
	SLO           SLOConfig           `yaml:"slo"`
	Alerting      AlertingConfig      `yaml:"alerting"`
	Dashboards    DashboardsConfig    `yaml:"dashboards"`
	HealthChecks  HealthChecksConfig  `yaml:"health_checks"`
	Performance   PerformanceConfig   `yaml:"performance"`
}

// OpenTelemetryConfig contains OpenTelemetry configuration
type OpenTelemetryConfig struct {
	Enabled  bool           `yaml:"enabled"`
	Service  ServiceConfig  `yaml:"service"`
	Resource ResourceConfig `yaml:"resource"`
	Tracing  TracingConfig  `yaml:"tracing"`
	Metrics  MetricsConfig  `yaml:"metrics"`
	Logging  LoggingConfig  `yaml:"logging"`
}

// ServiceConfig contains service identification
type ServiceConfig struct {
	Name        string `yaml:"name"`
	Version     string `yaml:"version"`
	Environment string `yaml:"environment"`
}

// ResourceConfig contains resource attributes
type ResourceConfig struct {
	Attributes map[string]string `yaml:"attributes"`
}

// TracingConfig contains tracing configuration
type TracingConfig struct {
	Enabled    bool             `yaml:"enabled"`
	Sampling   SamplingConfig   `yaml:"sampling"`
	Exporters  []ExporterConfig `yaml:"exporters"`
	Attributes AttributesConfig `yaml:"attributes"`
}

// SamplingConfig contains sampling configuration
type SamplingConfig struct {
	Type string  `yaml:"type"`
	Rate float64 `yaml:"rate"`
}

// ExporterConfig contains exporter configuration
type ExporterConfig struct {
	Type     string            `yaml:"type"`
	Endpoint string            `yaml:"endpoint"`
	Headers  map[string]string `yaml:"headers"`
	Timeout  string            `yaml:"timeout"`
	Interval string            `yaml:"interval,omitempty"`
	Enabled  bool              `yaml:"enabled,omitempty"`
}

// AttributesConfig contains attributes configuration
type AttributesConfig struct {
	IncludeEnvironment bool `yaml:"include_environment"`
	IncludeProcessInfo bool `yaml:"include_process_info"`
	IncludeHostInfo    bool `yaml:"include_host_info"`
}

// MetricsConfig contains metrics configuration
type MetricsConfig struct {
	Enabled       bool                          `yaml:"enabled"`
	Exporters     []ExporterConfig              `yaml:"exporters"`
	CustomMetrics map[string]CustomMetricConfig `yaml:"custom_metrics"`
}

// CustomMetricConfig contains custom metric configuration
type CustomMetricConfig struct {
	Enabled          bool      `yaml:"enabled"`
	HistogramBuckets []float64 `yaml:"histogram_buckets,omitempty"`
	CounterLabels    []string  `yaml:"counter_labels,omitempty"`
	GaugeLabels      []string  `yaml:"gauge_labels,omitempty"`
}

// LoggingConfig contains logging configuration
type LoggingConfig struct {
	Enabled    bool                `yaml:"enabled"`
	Exporters  []ExporterConfig    `yaml:"exporters"`
	Attributes LogAttributesConfig `yaml:"attributes"`
}

// LogAttributesConfig contains log attributes configuration
type LogAttributesConfig struct {
	IncludeTraceContext   bool `yaml:"include_trace_context"`
	IncludeSpanContext    bool `yaml:"include_span_context"`
	IncludeSourceLocation bool `yaml:"include_source_location"`
}

// SLOConfig contains SLO configuration
type SLOConfig struct {
	Enabled           bool            `yaml:"enabled"`
	ToolExecution     SLOTargetConfig `yaml:"tool_execution"`
	SessionManagement SLOTargetConfig `yaml:"session_management"`
}

// SLOTargetConfig contains SLO target configuration
type SLOTargetConfig struct {
	Availability AvailabilitySLO `yaml:"availability"`
	Latency      LatencySLO      `yaml:"latency,omitempty"`
	ResponseTime LatencySLO      `yaml:"response_time,omitempty"`
	ErrorRate    ErrorRateSLO    `yaml:"error_rate,omitempty"`
}

// AvailabilitySLO contains availability SLO configuration
type AvailabilitySLO struct {
	Target float64 `yaml:"target"`
	Window string  `yaml:"window"`
}

// LatencySLO contains latency SLO configuration
type LatencySLO struct {
	Target    float64 `yaml:"target"`
	Threshold string  `yaml:"threshold"`
	Window    string  `yaml:"window"`
}

// ErrorRateSLO contains error rate SLO configuration
type ErrorRateSLO struct {
	Target float64 `yaml:"target"`
	Window string  `yaml:"window"`
}

// AlertingConfig contains alerting configuration
type AlertingConfig struct {
	Enabled  bool           `yaml:"enabled"`
	Channels []AlertChannel `yaml:"channels"`
	Rules    []AlertRule    `yaml:"rules"`
}

// AlertChannel contains alert channel configuration
type AlertChannel struct {
	Name           string `yaml:"name"`
	Type           string `yaml:"type"`
	WebhookURL     string `yaml:"webhook_url,omitempty"`
	IntegrationKey string `yaml:"integration_key,omitempty"`
	Enabled        bool   `yaml:"enabled"`
}

// AlertRule contains alert rule configuration
type AlertRule struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Condition   string   `yaml:"condition"`
	Severity    string   `yaml:"severity"`
	Channels    []string `yaml:"channels"`
}

// DashboardsConfig contains dashboards configuration
type DashboardsConfig struct {
	Enabled bool          `yaml:"enabled"`
	Grafana GrafanaConfig `yaml:"grafana"`
}

// GrafanaConfig contains Grafana configuration
type GrafanaConfig struct {
	Enabled     bool                  `yaml:"enabled"`
	URL         string                `yaml:"url"`
	APIKey      string                `yaml:"api_key"`
	Definitions []DashboardDefinition `yaml:"definitions"`
}

// DashboardDefinition contains dashboard definition
type DashboardDefinition struct {
	Name string `yaml:"name"`
	File string `yaml:"file"`
}

// HealthChecksConfig contains health checks configuration
type HealthChecksConfig struct {
	Enabled   bool                      `yaml:"enabled"`
	Endpoints map[string]HealthEndpoint `yaml:"endpoints"`
	Probes    []HealthProbe             `yaml:"probes"`
}

// HealthEndpoint contains health endpoint configuration
type HealthEndpoint struct {
	Path string `yaml:"path"`
	Port int    `yaml:"port"`
}

// HealthProbe contains health probe configuration
type HealthProbe struct {
	Name           string `yaml:"name"`
	Type           string `yaml:"type"`
	Target         string `yaml:"target"`
	Timeout        string `yaml:"timeout"`
	ExpectedStatus int    `yaml:"expected_status,omitempty"`
}

// PerformanceConfig contains performance configuration
type PerformanceConfig struct {
	Profiling ProfilingConfig `yaml:"profiling"`
	Sampling  SamplingConfig  `yaml:"sampling"`
	Limits    LimitsConfig    `yaml:"limits"`
}

// ProfilingConfig contains profiling configuration
type ProfilingConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Endpoint string `yaml:"endpoint"`
}

// LimitsConfig contains limits configuration
type LimitsConfig struct {
	MaxConcurrentTools int    `yaml:"max_concurrent_tools"`
	MaxSessionDuration string `yaml:"max_session_duration"`
	MaxMemoryUsage     string `yaml:"max_memory_usage"`
	CPUProfileRate     int    `yaml:"cpu_profile_rate,omitempty"`
	MemoryProfileRate  int    `yaml:"memory_profile_rate,omitempty"`
}

// LoadObservabilityConfig loads observability configuration from file
func LoadObservabilityConfig(configPath string) (*ObservabilityConfig, error) {
	if configPath == "" {
		configPath = "observability.yaml"
	}

	// Clean the path to prevent directory traversal
	cleanPath := filepath.Clean(configPath)

	// Ensure we're not going outside the current directory for relative paths
	if !filepath.IsAbs(cleanPath) && (filepath.Dir(cleanPath) != "." && filepath.Dir(cleanPath) != "") {
		return nil, fmt.Errorf("invalid config path: relative paths must be in current directory")
	}

	data, err := os.ReadFile(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read observability config file: %w", err)
	}

	// Expand environment variables
	expandedData := os.ExpandEnv(string(data))

	var config ObservabilityConfig
	if err := yaml.Unmarshal([]byte(expandedData), &config); err != nil {
		return nil, fmt.Errorf("failed to parse observability config: %w", err)
	}

	return &config, nil
}

// GetTraceExporters returns enabled trace exporters
func (c *ObservabilityConfig) GetTraceExporters() []ExporterConfig {
	var exporters []ExporterConfig
	for _, exporter := range c.OpenTelemetry.Tracing.Exporters {
		if exporter.Enabled {
			exporters = append(exporters, exporter)
		}
	}
	return exporters
}

// GetMetricExporters returns enabled metric exporters
func (c *ObservabilityConfig) GetMetricExporters() []ExporterConfig {
	var exporters []ExporterConfig
	for _, exporter := range c.OpenTelemetry.Metrics.Exporters {
		if exporter.Enabled {
			exporters = append(exporters, exporter)
		}
	}
	return exporters
}

// GetAlertChannels returns enabled alert channels
func (c *ObservabilityConfig) GetAlertChannels() []AlertChannel {
	var channels []AlertChannel
	for _, channel := range c.Alerting.Channels {
		if channel.Enabled {
			channels = append(channels, channel)
		}
	}
	return channels
}

// GetSamplingTimeout returns sampling timeout as duration
func (s *SamplingConfig) GetSamplingTimeout() time.Duration {
	// Default timeout values based on sampling type
	switch s.Type {
	case "always_on", "always_off":
		return time.Millisecond // Very fast
	case "probabilistic":
		return 10 * time.Millisecond
	case "rate_limiting":
		return 100 * time.Millisecond
	default:
		return 10 * time.Millisecond
	}
}

// GetExporterTimeout returns exporter timeout as duration
func (e *ExporterConfig) GetExporterTimeout() time.Duration {
	if e.Timeout == "" {
		return 30 * time.Second // Default timeout
	}

	if duration, err := time.ParseDuration(e.Timeout); err == nil {
		return duration
	}

	return 30 * time.Second // Fallback
}

// GetExportInterval returns export interval as duration
func (e *ExporterConfig) GetExportInterval() time.Duration {
	if e.Interval == "" {
		return 60 * time.Second // Default interval
	}

	if duration, err := time.ParseDuration(e.Interval); err == nil {
		return duration
	}

	return 60 * time.Second // Fallback
}
