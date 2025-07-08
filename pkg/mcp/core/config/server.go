// Package config - Server configuration consolidation
// This file consolidates server configuration from multiple scattered locations
package config

import (
	"os"
	"path/filepath"
	"time"
)

// ServerConfig holds comprehensive configuration for the MCP server
// Consolidated from:
// - pkg/mcp/internal/server/config.go
// - pkg/mcp/internal/core/server_config.go
type ServerConfig struct {
	// Session management
	WorkspaceDir      string        `json:"workspace_dir" yaml:"workspace_dir" env:"MCP_WORKSPACE_DIR"`
	MaxSessions       int           `json:"max_sessions" yaml:"max_sessions" env:"MCP_MAX_SESSIONS" default:"10"`
	SessionTTL        time.Duration `json:"session_ttl" yaml:"session_ttl" env:"MCP_SESSION_TTL" default:"24h"`
	MaxDiskPerSession int64         `json:"max_disk_per_session" yaml:"max_disk_per_session" env:"MCP_MAX_DISK_PER_SESSION" default:"1073741824"` // 1GB
	TotalDiskLimit    int64         `json:"total_disk_limit" yaml:"total_disk_limit" env:"MCP_TOTAL_DISK_LIMIT" default:"10737418240"`            // 10GB

	// Storage
	StorePath string `json:"store_path" yaml:"store_path" env:"MCP_STORE_PATH"`

	// Transport configuration
	TransportType string   `json:"transport_type" yaml:"transport_type" env:"MCP_TRANSPORT_TYPE" default:"stdio"`
	HTTPAddr      string   `json:"http_addr" yaml:"http_addr" env:"MCP_HTTP_ADDR" default:"localhost"`
	HTTPPort      int      `json:"http_port" yaml:"http_port" env:"MCP_HTTP_PORT" default:"8090"`
	CORSOrigins   []string `json:"cors_origins" yaml:"cors_origins" env:"MCP_CORS_ORIGINS"`
	APIKey        string   `json:"api_key" yaml:"api_key" env:"MCP_API_KEY"`
	RateLimit     int      `json:"rate_limit" yaml:"rate_limit" env:"MCP_RATE_LIMIT" default:"100"` // Requests per minute per IP

	// Features
	SandboxEnabled bool `json:"sandbox_enabled" yaml:"sandbox_enabled" env:"MCP_SANDBOX_ENABLED" default:"false"`

	// Logging configuration
	LogLevel       string `json:"log_level" yaml:"log_level" env:"MCP_LOG_LEVEL" default:"info"`
	LogHTTPBodies  bool   `json:"log_http_bodies" yaml:"log_http_bodies" env:"MCP_LOG_HTTP_BODIES" default:"false"`
	MaxBodyLogSize int64  `json:"max_body_log_size" yaml:"max_body_log_size" env:"MCP_MAX_BODY_LOG_SIZE" default:"4096"`

	// Cleanup and maintenance
	CleanupInterval time.Duration `json:"cleanup_interval" yaml:"cleanup_interval" env:"MCP_CLEANUP_INTERVAL" default:"1h"`

	// Job management
	MaxWorkers int           `json:"max_workers" yaml:"max_workers" env:"MCP_MAX_WORKERS" default:"5"`
	JobTTL     time.Duration `json:"job_ttl" yaml:"job_ttl" env:"MCP_JOB_TTL" default:"1h"`

	// Service identification
	ServiceName    string `json:"service_name" yaml:"service_name" env:"MCP_SERVICE_NAME" default:"container-kit-mcp"`
	ServiceVersion string `json:"service_version" yaml:"service_version" env:"MCP_SERVICE_VERSION" default:"dev"`
	Environment    string `json:"environment" yaml:"environment" env:"MCP_ENVIRONMENT" default:"development"`

	// Security configuration
	TrustedProxies []string `json:"trusted_proxies" yaml:"trusted_proxies" env:"MCP_TRUSTED_PROXIES"`

	// Performance tuning
	ReadTimeout      time.Duration `json:"read_timeout" yaml:"read_timeout" env:"MCP_READ_TIMEOUT" default:"30s"`
	WriteTimeout     time.Duration `json:"write_timeout" yaml:"write_timeout" env:"MCP_WRITE_TIMEOUT" default:"30s"`
	IdleTimeout      time.Duration `json:"idle_timeout" yaml:"idle_timeout" env:"MCP_IDLE_TIMEOUT" default:"60s"`
	MaxHeaderSize    int           `json:"max_header_size" yaml:"max_header_size" env:"MCP_MAX_HEADER_SIZE" default:"8192"`
	MaxRequestSize   int64         `json:"max_request_size" yaml:"max_request_size" env:"MCP_MAX_REQUEST_SIZE" default:"10485760"` // 10MB
	ShutdownTimeout  time.Duration `json:"shutdown_timeout" yaml:"shutdown_timeout" env:"MCP_SHUTDOWN_TIMEOUT" default:"30s"`
	KeepAliveTimeout time.Duration `json:"keep_alive_timeout" yaml:"keep_alive_timeout" env:"MCP_KEEP_ALIVE_TIMEOUT" default:"3m"`

	// Resource limits
	MaxConcurrentRequests int   `json:"max_concurrent_requests" yaml:"max_concurrent_requests" env:"MCP_MAX_CONCURRENT_REQUESTS" default:"100"`
	MemoryLimit           int64 `json:"memory_limit" yaml:"memory_limit" env:"MCP_MEMORY_LIMIT"` // in bytes
	CPULimit              int   `json:"cpu_limit" yaml:"cpu_limit" env:"MCP_CPU_LIMIT"`          // CPU cores

	// Health check configuration
	HealthCheckEnabled  bool          `json:"health_check_enabled" yaml:"health_check_enabled" env:"MCP_HEALTH_CHECK_ENABLED" default:"true"`
	HealthCheckInterval time.Duration `json:"health_check_interval" yaml:"health_check_interval" env:"MCP_HEALTH_CHECK_INTERVAL" default:"30s"`
	HealthCheckTimeout  time.Duration `json:"health_check_timeout" yaml:"health_check_timeout" env:"MCP_HEALTH_CHECK_TIMEOUT" default:"5s"`

	// Metrics configuration
	MetricsEnabled   bool   `json:"metrics_enabled" yaml:"metrics_enabled" env:"MCP_METRICS_ENABLED" default:"true"`
	MetricsPath      string `json:"metrics_path" yaml:"metrics_path" env:"MCP_METRICS_PATH" default:"/metrics"`
	MetricsPort      int    `json:"metrics_port" yaml:"metrics_port" env:"MCP_METRICS_PORT" default:"9090"`
	ProfilingPort    int    `json:"profiling_port" yaml:"profiling_port" env:"MCP_PROFILING_PORT" default:"6060"`
	ProfilingEnabled bool   `json:"profiling_enabled" yaml:"profiling_enabled" env:"MCP_PROFILING_ENABLED" default:"false"`
}

// DefaultServerConfig returns a server configuration with sensible defaults
func DefaultServerConfig() *ServerConfig {
	homeDir, _ := os.UserHomeDir()
	defaultWorkspaceDir := filepath.Join(homeDir, ".container-kit", "workspaces")
	defaultStorePath := filepath.Join(homeDir, ".container-kit", "store")

	return &ServerConfig{
		WorkspaceDir:          defaultWorkspaceDir,
		MaxSessions:           10,
		SessionTTL:            24 * time.Hour,
		MaxDiskPerSession:     1024 * 1024 * 1024,      // 1GB
		TotalDiskLimit:        10 * 1024 * 1024 * 1024, // 10GB
		StorePath:             defaultStorePath,
		TransportType:         "stdio",
		HTTPAddr:              "localhost",
		HTTPPort:              8090,
		CORSOrigins:           []string{},
		RateLimit:             100,
		SandboxEnabled:        false,
		LogLevel:              "info",
		LogHTTPBodies:         false,
		MaxBodyLogSize:        4096,
		CleanupInterval:       time.Hour,
		MaxWorkers:            5,
		JobTTL:                time.Hour,
		ServiceName:           "container-kit-mcp",
		ServiceVersion:        "dev",
		Environment:           "development",
		TrustedProxies:        []string{},
		ReadTimeout:           30 * time.Second,
		WriteTimeout:          30 * time.Second,
		IdleTimeout:           60 * time.Second,
		MaxHeaderSize:         8192,
		MaxRequestSize:        10 * 1024 * 1024, // 10MB
		ShutdownTimeout:       30 * time.Second,
		KeepAliveTimeout:      3 * time.Minute,
		MaxConcurrentRequests: 100,
		HealthCheckEnabled:    true,
		HealthCheckInterval:   30 * time.Second,
		HealthCheckTimeout:    5 * time.Second,
		MetricsEnabled:        true,
		MetricsPath:           "/metrics",
		MetricsPort:           9090,
		ProfilingPort:         6060,
		ProfilingEnabled:      false,
	}
}

// Validate validates the server configuration
func (c *ServerConfig) Validate() error {
	if c.WorkspaceDir == "" {
		return NewRichConfigValidationError("workspace_dir", "workspace directory is required")
	}

	if c.StorePath == "" {
		return NewRichConfigValidationError("store_path", "store path is required")
	}

	if c.MaxSessions < 1 {
		return NewRichConfigValidationError("max_sessions", "max sessions must be at least 1")
	}

	if c.SessionTTL <= 0 {
		return NewRichConfigValidationError("session_ttl", "session TTL must be positive")
	}

	if c.TransportType != "stdio" && c.TransportType != "http" {
		return NewRichConfigValidationError("transport_type", "transport type must be 'stdio' or 'http'")
	}

	if c.TransportType == "http" {
		if c.HTTPPort < 1 || c.HTTPPort > 65535 {
			return NewRichConfigValidationError("http_port", "HTTP port must be between 1 and 65535")
		}
	}

	if c.MaxWorkers < 1 {
		return NewRichConfigValidationError("max_workers", "max workers must be at least 1")
	}

	if c.CleanupInterval <= 0 {
		return NewRichConfigValidationError("cleanup_interval", "cleanup interval must be positive")
	}

	if c.MaxConcurrentRequests < 1 {
		return NewRichConfigValidationError("max_concurrent_requests", "max concurrent requests must be at least 1")
	}

	return nil
}

// GetWorkspaceDir returns the workspace directory, creating it if it doesn't exist
func (c *ServerConfig) GetWorkspaceDir() (string, error) {
	if err := os.MkdirAll(c.WorkspaceDir, 0755); err != nil {
		return "", NewRichConfigValidationError("workspace_dir", "failed to create workspace directory: "+err.Error())
	}
	return c.WorkspaceDir, nil
}

// GetStorePath returns the store path, creating it if it doesn't exist
func (c *ServerConfig) GetStorePath() (string, error) {
	if err := os.MkdirAll(c.StorePath, 0755); err != nil {
		return "", NewRichConfigValidationError("store_path", "failed to create store directory: "+err.Error())
	}
	return c.StorePath, nil
}

// IsHTTPTransport returns true if the transport type is HTTP
func (c *ServerConfig) IsHTTPTransport() bool {
	return c.TransportType == "http"
}

// IsStdioTransport returns true if the transport type is stdio
func (c *ServerConfig) IsStdioTransport() bool {
	return c.TransportType == "stdio"
}

// GetHTTPAddress returns the full HTTP address
func (c *ServerConfig) GetHTTPAddress() string {
	if c.HTTPAddr == "" {
		return "localhost:" + string(rune(c.HTTPPort))
	}
	return c.HTTPAddr + ":" + string(rune(c.HTTPPort))
}

// IsDevelopment returns true if the environment is development
func (c *ServerConfig) IsDevelopment() bool {
	return c.Environment == "development" || c.Environment == "dev"
}

// IsProduction returns true if the environment is production
func (c *ServerConfig) IsProduction() bool {
	return c.Environment == "production" || c.Environment == "prod"
}

// WorkerConfig holds configuration for background workers
type WorkerConfig struct {
	ShutdownTimeout   time.Duration `json:"shutdown_timeout" yaml:"shutdown_timeout" env:"MCP_WORKER_SHUTDOWN_TIMEOUT" default:"30s"`
	HealthCheckPeriod time.Duration `json:"health_check_period" yaml:"health_check_period" env:"MCP_WORKER_HEALTH_CHECK_PERIOD" default:"10s"`
	MaxRetries        int           `json:"max_retries" yaml:"max_retries" env:"MCP_WORKER_MAX_RETRIES" default:"3"`
}

// CircuitBreakerStats holds statistics for circuit breaker monitoring
type CircuitBreakerStats struct {
	Name           string    `json:"name"`
	State          string    `json:"state"`
	TotalRequests  int64     `json:"total_requests"`
	FailedRequests int64     `json:"failed_requests"`
	SuccessRate    float64   `json:"success_rate"`
	LastFailure    time.Time `json:"last_failure,omitempty"`
}

// TimeoutConfig holds timeout configuration for various operations
type TimeoutConfig struct {
	Read     time.Duration `json:"read"`
	Write    time.Duration `json:"write"`
	Idle     time.Duration `json:"idle"`
	Shutdown time.Duration `json:"shutdown"`
}

// CircuitBreakerConfig holds circuit breaker configuration
type CircuitBreakerConfig struct {
	Enabled       bool          `json:"enabled"`
	FailureRate   float64       `json:"failure_rate"`
	RequestVolume int           `json:"request_volume"`
	SleepWindow   time.Duration `json:"sleep_window"`
}
