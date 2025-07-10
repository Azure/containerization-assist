package telemetry

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds telemetry configuration
type Config struct {
	// Service identification
	ServiceName    string
	ServiceVersion string
	Environment    string

	// Tracing configuration
	TracingEnabled  bool
	TracingEndpoint string
	TraceSampleRate float64

	// Metrics configuration
	MetricsEnabled  bool
	MetricsEndpoint string
	MetricsInterval time.Duration

	// Resource attributes
	ResourceAttributes map[string]string
}

// DefaultConfig returns a default telemetry configuration
func DefaultConfig() *Config {
	return &Config{
		ServiceName:     "container-kit",
		ServiceVersion:  getEnvWithDefault("CONTAINER_KIT_VERSION", "dev"),
		Environment:     getEnvWithDefault("CONTAINER_KIT_ENV", "development"),
		TracingEnabled:  getBoolEnvWithDefault("CONTAINER_KIT_TRACING_ENABLED", true),
		TracingEndpoint: getEnvWithDefault("OTEL_EXPORTER_JAEGER_ENDPOINT", "http://localhost:14268/api/traces"),
		TraceSampleRate: getFloatEnvWithDefault("CONTAINER_KIT_TRACE_SAMPLE_RATE", 1.0),
		MetricsEnabled:  getBoolEnvWithDefault("CONTAINER_KIT_METRICS_ENABLED", true),
		MetricsEndpoint: getEnvWithDefault("OTEL_EXPORTER_PROMETHEUS_ENDPOINT", "http://localhost:9090"),
		MetricsInterval: getDurationEnvWithDefault("CONTAINER_KIT_METRICS_INTERVAL", 15*time.Second),
		ResourceAttributes: map[string]string{
			"service.name":    "container-kit",
			"service.version": getEnvWithDefault("CONTAINER_KIT_VERSION", "dev"),
		},
	}
}

// LoadFromEnv loads configuration from environment variables
func (c *Config) LoadFromEnv() {
	if val := os.Getenv("CONTAINER_KIT_SERVICE_NAME"); val != "" {
		c.ServiceName = val
	}
	if val := os.Getenv("CONTAINER_KIT_VERSION"); val != "" {
		c.ServiceVersion = val
	}
	if val := os.Getenv("CONTAINER_KIT_ENV"); val != "" {
		c.Environment = val
	}

	c.TracingEnabled = getBoolEnvWithDefault("CONTAINER_KIT_TRACING_ENABLED", c.TracingEnabled)
	c.MetricsEnabled = getBoolEnvWithDefault("CONTAINER_KIT_METRICS_ENABLED", c.MetricsEnabled)

	if val := os.Getenv("OTEL_EXPORTER_JAEGER_ENDPOINT"); val != "" {
		c.TracingEndpoint = val
	}
	if val := os.Getenv("OTEL_EXPORTER_PROMETHEUS_ENDPOINT"); val != "" {
		c.MetricsEndpoint = val
	}

	c.TraceSampleRate = getFloatEnvWithDefault("CONTAINER_KIT_TRACE_SAMPLE_RATE", c.TraceSampleRate)
	c.MetricsInterval = getDurationEnvWithDefault("CONTAINER_KIT_METRICS_INTERVAL", c.MetricsInterval)
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.ServiceName == "" {
		return fmt.Errorf("service name is required")
	}
	if c.ServiceVersion == "" {
		return fmt.Errorf("service version is required")
	}
	if c.TraceSampleRate < 0 || c.TraceSampleRate > 1 {
		return fmt.Errorf("trace sample rate must be between 0 and 1")
	}
	if c.MetricsInterval <= 0 {
		return fmt.Errorf("metrics interval must be positive")
	}
	return nil
}

// Helper functions for environment variable parsing
func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getBoolEnvWithDefault(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return defaultValue
	}
	return parsed
}

func getFloatEnvWithDefault(key string, defaultValue float64) float64 {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return defaultValue
	}
	return parsed
}

func getDurationEnvWithDefault(key string, defaultValue time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return defaultValue
	}
	return parsed
}