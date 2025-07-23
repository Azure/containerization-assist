// Package config provides centralized configuration management for the MCP server
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

// Config represents the complete MCP server configuration
type Config struct {
	// Server settings
	WorkspaceDir      string        `env:"MCP_WORKSPACE_DIR" yaml:"workspace_dir"`
	StorePath         string        `env:"MCP_STORE_PATH" yaml:"store_path"`
	SessionTTL        time.Duration `env:"MCP_SESSION_TTL" yaml:"session_ttl"`
	MaxSessions       int           `env:"MCP_MAX_SESSIONS" yaml:"max_sessions"`
	MaxDiskPerSession int64         `env:"MCP_MAX_DISK_PER_SESSION" yaml:"max_disk_per_session"`
	TotalDiskLimit    int64         `env:"MCP_TOTAL_DISK_LIMIT" yaml:"total_disk_limit"`
	CleanupInterval   time.Duration `env:"MCP_CLEANUP_INTERVAL" yaml:"cleanup_interval"`

	// Transport settings
	TransportType string   `env:"MCP_TRANSPORT" yaml:"transport_type"`
	HTTPAddr      string   `env:"MCP_HTTP_ADDR" yaml:"http_addr"`
	HTTPPort      int      `env:"MCP_HTTP_PORT" yaml:"http_port"`
	CORSOrigins   []string `env:"MCP_CORS_ORIGINS" yaml:"cors_origins"`

	// Logging settings
	LogLevel       string `env:"MCP_LOG_LEVEL" yaml:"log_level"`
	LogHTTPBodies  bool   `env:"MCP_LOG_HTTP_BODIES" yaml:"log_http_bodies"`
	MaxBodyLogSize int64  `env:"MCP_MAX_BODY_LOG_SIZE" yaml:"max_body_log_size"`

	// Prompt settings
	PromptTemplateDir   string `env:"MCP_PROMPT_TEMPLATE_DIR" yaml:"prompt_template_dir"`
	PromptHotReload     bool   `env:"MCP_PROMPT_HOT_RELOAD" yaml:"prompt_hot_reload"`
	PromptAllowOverride bool   `env:"MCP_PROMPT_ALLOW_OVERRIDE" yaml:"prompt_allow_override"`

	// Sampling settings
	SamplingEndpoint      string        `env:"MCP_SAMPLING_ENDPOINT" yaml:"sampling_endpoint"`
	SamplingAPIKey        string        `env:"MCP_SAMPLING_API_KEY" yaml:"sampling_api_key"`
	SamplingMaxTokens     int32         `env:"MCP_SAMPLING_MAX_TOKENS" yaml:"sampling_max_tokens"`
	SamplingTemperature   float32       `env:"MCP_SAMPLING_TEMPERATURE" yaml:"sampling_temperature"`
	SamplingRetryAttempts int           `env:"MCP_SAMPLING_RETRY_ATTEMPTS" yaml:"sampling_retry_attempts"`
	SamplingTokenBudget   int           `env:"MCP_SAMPLING_TOKEN_BUDGET" yaml:"sampling_token_budget"`
	SamplingBaseBackoff   time.Duration `env:"MCP_SAMPLING_BASE_BACKOFF" yaml:"sampling_base_backoff"`
	SamplingMaxBackoff    time.Duration `env:"MCP_SAMPLING_MAX_BACKOFF" yaml:"sampling_max_backoff"`
	SamplingStreaming     bool          `env:"MCP_SAMPLING_STREAMING" yaml:"sampling_streaming"`
	SamplingTimeout       time.Duration `env:"MCP_SAMPLING_TIMEOUT" yaml:"sampling_timeout"`

	// Tracing settings
	TracingEnabled     bool    `env:"MCP_TRACING_ENABLED" yaml:"tracing_enabled"`
	TracingEndpoint    string  `env:"MCP_TRACING_ENDPOINT" yaml:"tracing_endpoint"`
	TracingServiceName string  `env:"MCP_TRACING_SERVICE_NAME" yaml:"tracing_service_name"`
	TracingSampleRate  float64 `env:"MCP_TRACING_SAMPLE_RATE" yaml:"tracing_sample_rate"`

	// Security settings
	SecurityScanEnabled    bool     `env:"MCP_SECURITY_SCAN_ENABLED" yaml:"security_scan_enabled"`
	SecurityScanners       []string `env:"MCP_SECURITY_SCANNERS" yaml:"security_scanners"`
	SecurityFailOnHigh     bool     `env:"MCP_SECURITY_FAIL_ON_HIGH" yaml:"security_fail_on_high"`
	SecurityFailOnCritical bool     `env:"MCP_SECURITY_FAIL_ON_CRITICAL" yaml:"security_fail_on_critical"`

	// Registry settings
	RegistryURL      string `env:"MCP_REGISTRY_URL" yaml:"registry_url"`
	RegistryUsername string `env:"MCP_REGISTRY_USERNAME" yaml:"registry_username"`
	RegistryPassword string `env:"MCP_REGISTRY_PASSWORD" yaml:"registry_password"`
	RegistryInsecure bool   `env:"MCP_REGISTRY_INSECURE" yaml:"registry_insecure"`

	// Orchestrator settings
	Orchestrator OrchestratorSettings `env:",prefix=MCP_ORCHESTRATOR_" yaml:"orchestrator"`
}

// SamplingConfig holds configuration for the sampling client
type SamplingConfig struct {
	MaxTokens        int32         `json:"max_tokens"`
	Temperature      float32       `json:"temperature"`
	RetryAttempts    int           `json:"retry_attempts"`
	TokenBudget      int           `json:"token_budget"`
	BaseBackoff      time.Duration `json:"base_backoff"`
	MaxBackoff       time.Duration `json:"max_backoff"`
	StreamingEnabled bool          `json:"streaming_enabled"`
	RequestTimeout   time.Duration `json:"request_timeout"`
	Endpoint         string        `json:"endpoint"`
	APIKey           string        `json:"api_key"`
}

// PromptConfig holds configuration for the prompt manager
type PromptConfig struct {
	TemplateDir     string `json:"template_dir"`
	EnableHotReload bool   `json:"enable_hot_reload"`
	AllowOverride   bool   `json:"allow_override"`
}

// TracingConfig holds configuration for distributed tracing
type TracingConfig struct {
	Enabled     bool    `json:"enabled"`
	Endpoint    string  `json:"endpoint"`
	ServiceName string  `json:"service_name"`
	SampleRate  float64 `json:"sample_rate"`
}

// SecurityConfig holds configuration for security scanning
type SecurityConfig struct {
	ScanEnabled    bool     `json:"scan_enabled"`
	Scanners       []string `json:"scanners"`
	FailOnHigh     bool     `json:"fail_on_high"`
	FailOnCritical bool     `json:"fail_on_critical"`
}

// RegistryConfig holds configuration for container registry access
type RegistryConfig struct {
	URL      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
	Insecure bool   `json:"insecure"`
}

// OrchestratorSettings holds configuration for the unified orchestrator
type OrchestratorSettings struct {
	Mode                     string        `env:"MODE" yaml:"mode" default:"sequential"`
	ParallelExecutionEnabled bool          `env:"PARALLEL_ENABLED" yaml:"parallel_enabled" default:"false"`
	MaxParallelSteps         int           `env:"MAX_PARALLEL" yaml:"max_parallel_steps" default:"3"`
	AdaptiveLearningEnabled  bool          `env:"ADAPTIVE_ENABLED" yaml:"adaptive_enabled" default:"false"`
	EventsEnabled            bool          `env:"EVENTS_ENABLED" yaml:"events_enabled" default:"false"`
	DefaultTimeout           time.Duration `env:"DEFAULT_TIMEOUT" yaml:"default_timeout" default:"5m"`

	// Middleware settings
	LoggingLevel       string `env:"LOGGING_LEVEL" yaml:"logging_level" default:"standard"`
	ProgressMode       string `env:"PROGRESS_MODE" yaml:"progress_mode" default:"simple"`
	TracingEnabled     bool   `env:"TRACING_ENABLED" yaml:"tracing_enabled" default:"false"`
	EnhancementEnabled bool   `env:"ENHANCEMENT_ENABLED" yaml:"enhancement_enabled" default:"false"`

	// Retry policy settings
	RetryBaseBackoff       time.Duration `env:"RETRY_BASE_BACKOFF" yaml:"retry_base_backoff" default:"1s"`
	RetryMaxBackoff        time.Duration `env:"RETRY_MAX_BACKOFF" yaml:"retry_max_backoff" default:"30s"`
	RetryBackoffMultiplier float64       `env:"RETRY_BACKOFF_MULTIPLIER" yaml:"retry_backoff_multiplier" default:"2.0"`
	RetryJitter            bool          `env:"RETRY_JITTER" yaml:"retry_jitter" default:"true"`
	RetryMaxJitter         float64       `env:"RETRY_MAX_JITTER" yaml:"retry_max_jitter" default:"0.1"`

	// Timeout settings
	TimeoutAdaptive bool          `env:"TIMEOUT_ADAPTIVE" yaml:"timeout_adaptive" default:"false"`
	TimeoutMax      time.Duration `env:"TIMEOUT_MAX" yaml:"timeout_max" default:"15m"`
	TimeoutMin      time.Duration `env:"TIMEOUT_MIN" yaml:"timeout_min" default:"1s"`
}

// ToOrchestratorConfig converts OrchestratorSettings to workflow.OrchestratorConfig
func (o *OrchestratorSettings) ToOrchestratorConfig() workflow.OrchestratorConfig {
	return workflow.OrchestratorConfig{
		ExecutionMode: workflow.ExecutionMode(o.Mode),
		ParallelConfig: workflow.ParallelConfig{
			Enabled:          o.ParallelExecutionEnabled,
			MaxParallelSteps: o.MaxParallelSteps,
			DependencyAware:  true, // Enable by default
		},
		AdaptiveConfig: workflow.AdaptiveConfig{
			Enabled:            o.AdaptiveLearningEnabled,
			PatternRecognition: o.AdaptiveLearningEnabled,
			StrategyLearning:   o.AdaptiveLearningEnabled,
			MinConfidence:      0.7, // Default confidence threshold
		},
		EventsEnabled:  o.EventsEnabled,
		MaxConcurrency: o.MaxParallelSteps,
		DefaultTimeout: o.DefaultTimeout,
		MiddlewareConfig: workflow.MiddlewareConfig{
			LoggingLevel:       o.LoggingLevel,
			ProgressMode:       o.ProgressMode,
			TracingEnabled:     o.TracingEnabled,
			EnhancementEnabled: o.EnhancementEnabled,
			RetryPolicy: workflow.RetryPolicy{
				BaseBackoff:             o.RetryBaseBackoff,
				MaxBackoff:              o.RetryMaxBackoff,
				BackoffMultiplier:       o.RetryBackoffMultiplier,
				Jitter:                  o.RetryJitter,
				MaxJitter:               o.RetryMaxJitter,
				ErrorPatternRecognition: o.AdaptiveLearningEnabled,
			},
			TimeoutConfig: workflow.TimeoutConfig{
				DefaultTimeout:   o.DefaultTimeout,
				AdaptiveTimeouts: o.TimeoutAdaptive,
				MaxTimeout:       o.TimeoutMax,
				MinTimeout:       o.TimeoutMin,
			},
		},
	}
}

// LoadOption is a functional option for loading configuration
type LoadOption func(*loadOptions)

type loadOptions struct {
	envFile    string
	configFile string
	useEnv     bool
	useFlags   bool
}

// FromFile specifies a YAML config file to load
func FromFile(path string) LoadOption {
	return func(o *loadOptions) {
		o.configFile = path
	}
}

// FromEnv enables loading from environment variables
func FromEnv() LoadOption {
	return func(o *loadOptions) {
		o.useEnv = true
	}
}

// FromEnvFile specifies a .env file to load
func FromEnvFile(path string) LoadOption {
	return func(o *loadOptions) {
		o.envFile = path
	}
}

// Load loads configuration from multiple sources
func Load(opts ...LoadOption) (*Config, error) {
	options := &loadOptions{
		useEnv: true, // Default to using env vars
	}
	for _, opt := range opts {
		opt(options)
	}

	// Start with defaults
	cfg := DefaultConfig()

	// Load .env file if specified
	if options.envFile != "" {
		if err := godotenv.Load(options.envFile); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to load .env file: %w", err)
		}
	}

	// Load from config file if specified
	if options.configFile != "" {
		if err := loadFromFile(cfg, options.configFile); err != nil {
			return nil, err
		}
	}

	// Override with environment variables
	if options.useEnv {
		loadFromEnv(cfg)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		// Server settings
		WorkspaceDir:      "/tmp/mcp-workspace",
		StorePath:         "/tmp/mcp-store/sessions.db",
		SessionTTL:        24 * time.Hour,
		MaxSessions:       100,
		MaxDiskPerSession: 100 * 1024 * 1024,  // 100MB
		TotalDiskLimit:    1024 * 1024 * 1024, // 1GB
		CleanupInterval:   1 * time.Hour,

		// Transport settings
		TransportType: "stdio",
		HTTPAddr:      "0.0.0.0",
		HTTPPort:      8080,
		CORSOrigins:   []string{"*"},

		// Logging settings
		LogLevel:       "info",
		LogHTTPBodies:  false,
		MaxBodyLogSize: 1024 * 1024, // 1MB

		// Prompt settings
		PromptTemplateDir:   "",
		PromptHotReload:     false,
		PromptAllowOverride: false,

		// Sampling settings
		SamplingEndpoint:      "",
		SamplingAPIKey:        "",
		SamplingMaxTokens:     4096,
		SamplingTemperature:   0.7,
		SamplingRetryAttempts: 3,
		SamplingTokenBudget:   5000,
		SamplingBaseBackoff:   200 * time.Millisecond,
		SamplingMaxBackoff:    10 * time.Second,
		SamplingStreaming:     false,
		SamplingTimeout:       30 * time.Second,

		// Tracing settings
		TracingEnabled:     false,
		TracingEndpoint:    "",
		TracingServiceName: "container-kit-mcp",
		TracingSampleRate:  0.1,

		// Security settings
		SecurityScanEnabled:    true,
		SecurityScanners:       []string{"trivy"},
		SecurityFailOnHigh:     false,
		SecurityFailOnCritical: true,

		// Registry settings
		RegistryURL:      "",
		RegistryUsername: "",
		RegistryPassword: "",
		RegistryInsecure: false,
	}
}

// loadFromFile loads configuration from a YAML file
func loadFromFile(cfg *Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	return nil
}

// loadFromEnv loads configuration from environment variables
func loadFromEnv(cfg *Config) {
	// Server settings
	if v := os.Getenv("MCP_WORKSPACE_DIR"); v != "" {
		cfg.WorkspaceDir = v
	}
	if v := os.Getenv("MCP_STORE_PATH"); v != "" {
		cfg.StorePath = v
	}
	if v := os.Getenv("MCP_SESSION_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.SessionTTL = d
		}
	}
	if v := os.Getenv("MCP_MAX_SESSIONS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.MaxSessions = n
		}
	}
	if v := os.Getenv("MCP_MAX_DISK_PER_SESSION"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			cfg.MaxDiskPerSession = n
		}
	}
	if v := os.Getenv("MCP_TOTAL_DISK_LIMIT"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			cfg.TotalDiskLimit = n
		}
	}
	if v := os.Getenv("MCP_CLEANUP_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.CleanupInterval = d
		}
	}

	// Transport settings
	if v := os.Getenv("MCP_TRANSPORT"); v != "" {
		cfg.TransportType = v
	}
	if v := os.Getenv("MCP_HTTP_ADDR"); v != "" {
		cfg.HTTPAddr = v
	}
	if v := os.Getenv("MCP_HTTP_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.HTTPPort = n
		}
	}
	if v := os.Getenv("MCP_CORS_ORIGINS"); v != "" {
		cfg.CORSOrigins = strings.Split(v, ",")
	}

	// Logging settings
	if v := os.Getenv("MCP_LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
	if v := os.Getenv("MCP_LOG_HTTP_BODIES"); v != "" {
		cfg.LogHTTPBodies = v == "true" || v == "1"
	}
	if v := os.Getenv("MCP_MAX_BODY_LOG_SIZE"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			cfg.MaxBodyLogSize = n
		}
	}

	// Prompt settings
	if v := os.Getenv("MCP_PROMPT_TEMPLATE_DIR"); v != "" {
		cfg.PromptTemplateDir = v
	}
	if v := os.Getenv("MCP_PROMPT_HOT_RELOAD"); v != "" {
		cfg.PromptHotReload = v == "true" || v == "1"
	}
	if v := os.Getenv("MCP_PROMPT_ALLOW_OVERRIDE"); v != "" {
		cfg.PromptAllowOverride = v == "true" || v == "1"
	}

	// Sampling settings
	if v := os.Getenv("MCP_SAMPLING_ENDPOINT"); v != "" {
		cfg.SamplingEndpoint = v
	}
	if v := os.Getenv("MCP_SAMPLING_API_KEY"); v != "" {
		cfg.SamplingAPIKey = v
	}
	if v := os.Getenv("MCP_SAMPLING_MAX_TOKENS"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 32); err == nil {
			cfg.SamplingMaxTokens = int32(n)
		}
	}
	if v := os.Getenv("MCP_SAMPLING_TEMPERATURE"); v != "" {
		if f, err := strconv.ParseFloat(v, 32); err == nil {
			cfg.SamplingTemperature = float32(f)
		}
	}
	if v := os.Getenv("MCP_SAMPLING_RETRY_ATTEMPTS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.SamplingRetryAttempts = n
		}
	}
	if v := os.Getenv("MCP_SAMPLING_TOKEN_BUDGET"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.SamplingTokenBudget = n
		}
	}
	if v := os.Getenv("MCP_SAMPLING_BASE_BACKOFF"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.SamplingBaseBackoff = d
		}
	}
	if v := os.Getenv("MCP_SAMPLING_MAX_BACKOFF"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.SamplingMaxBackoff = d
		}
	}
	if v := os.Getenv("MCP_SAMPLING_STREAMING"); v != "" {
		cfg.SamplingStreaming = v == "true" || v == "1"
	}
	if v := os.Getenv("MCP_SAMPLING_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.SamplingTimeout = d
		}
	}

	// Tracing settings
	if v := os.Getenv("MCP_TRACING_ENABLED"); v != "" {
		cfg.TracingEnabled = v == "true" || v == "1"
	}
	if v := os.Getenv("MCP_TRACING_ENDPOINT"); v != "" {
		cfg.TracingEndpoint = v
	}
	if v := os.Getenv("MCP_TRACING_SERVICE_NAME"); v != "" {
		cfg.TracingServiceName = v
	}
	if v := os.Getenv("MCP_TRACING_SAMPLE_RATE"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.TracingSampleRate = f
		}
	}

	// Security settings
	if v := os.Getenv("MCP_SECURITY_SCAN_ENABLED"); v != "" {
		cfg.SecurityScanEnabled = v == "true" || v == "1"
	}
	if v := os.Getenv("MCP_SECURITY_SCANNERS"); v != "" {
		cfg.SecurityScanners = strings.Split(v, ",")
	}
	if v := os.Getenv("MCP_SECURITY_FAIL_ON_HIGH"); v != "" {
		cfg.SecurityFailOnHigh = v == "true" || v == "1"
	}
	if v := os.Getenv("MCP_SECURITY_FAIL_ON_CRITICAL"); v != "" {
		cfg.SecurityFailOnCritical = v == "true" || v == "1"
	}

	// Registry settings
	if v := os.Getenv("MCP_REGISTRY_URL"); v != "" {
		cfg.RegistryURL = v
	}
	if v := os.Getenv("MCP_REGISTRY_USERNAME"); v != "" {
		cfg.RegistryUsername = v
	}
	if v := os.Getenv("MCP_REGISTRY_PASSWORD"); v != "" {
		cfg.RegistryPassword = v
	}
	if v := os.Getenv("MCP_REGISTRY_INSECURE"); v != "" {
		cfg.RegistryInsecure = v == "true" || v == "1"
	}
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	// Server settings validation
	if c.WorkspaceDir == "" {
		return fmt.Errorf("workspace_dir is required")
	}
	if c.MaxSessions <= 0 {
		return fmt.Errorf("max_sessions must be positive")
	}
	if c.SessionTTL <= 0 {
		return fmt.Errorf("session_ttl must be positive")
	}
	if c.CleanupInterval <= 0 {
		return fmt.Errorf("cleanup_interval must be positive")
	}

	// Transport settings validation
	if c.TransportType != "stdio" && c.TransportType != "http" {
		return fmt.Errorf("transport_type must be 'stdio' or 'http'")
	}
	if c.TransportType == "http" && c.HTTPPort <= 0 {
		return fmt.Errorf("http_port must be positive when using http transport")
	}

	// Logging settings validation
	validLogLevels := []string{"debug", "info", "warn", "error"}
	valid := false
	for _, level := range validLogLevels {
		if c.LogLevel == level {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("log_level must be one of: %s", strings.Join(validLogLevels, ", "))
	}
	if c.MaxBodyLogSize < 0 {
		return fmt.Errorf("max_body_log_size must be non-negative")
	}

	// Sampling settings validation
	if c.SamplingTemperature < 0 || c.SamplingTemperature > 2 {
		return fmt.Errorf("sampling_temperature must be between 0 and 2")
	}
	if c.SamplingMaxTokens <= 0 {
		return fmt.Errorf("sampling_max_tokens must be positive")
	}
	if c.SamplingRetryAttempts < 0 {
		return fmt.Errorf("sampling_retry_attempts must be non-negative")
	}
	if c.SamplingTokenBudget <= 0 {
		return fmt.Errorf("sampling_token_budget must be positive")
	}
	if c.SamplingBaseBackoff <= 0 {
		return fmt.Errorf("sampling_base_backoff must be positive")
	}
	if c.SamplingMaxBackoff <= c.SamplingBaseBackoff {
		return fmt.Errorf("sampling_max_backoff must be greater than sampling_base_backoff")
	}
	if c.SamplingTimeout <= 0 {
		return fmt.Errorf("sampling_timeout must be positive")
	}

	// Tracing settings validation
	if c.TracingSampleRate < 0 || c.TracingSampleRate > 1 {
		return fmt.Errorf("tracing_sample_rate must be between 0 and 1")
	}

	// Security settings validation
	if len(c.SecurityScanners) == 0 && c.SecurityScanEnabled {
		return fmt.Errorf("security_scanners cannot be empty when security scanning is enabled")
	}

	return nil
}

// ToServerConfig converts to the legacy ServerConfig format
func (c *Config) ToServerConfig() workflow.ServerConfig {
	return workflow.ServerConfig{
		WorkspaceDir:      c.WorkspaceDir,
		StorePath:         c.StorePath,
		SessionTTL:        c.SessionTTL,
		MaxSessions:       c.MaxSessions,
		MaxDiskPerSession: c.MaxDiskPerSession,
		TotalDiskLimit:    c.TotalDiskLimit,
		CleanupInterval:   c.CleanupInterval,
		TransportType:     c.TransportType,
		HTTPAddr:          c.HTTPAddr,
		HTTPPort:          c.HTTPPort,
		CORSOrigins:       c.CORSOrigins,
		LogLevel:          c.LogLevel,
		LogHTTPBodies:     c.LogHTTPBodies,
		MaxBodyLogSize:    c.MaxBodyLogSize,
	}
}

// ToSamplingConfig creates a sampling configuration from the unified config
func (c *Config) ToSamplingConfig() SamplingConfig {
	return SamplingConfig{
		MaxTokens:        c.SamplingMaxTokens,
		Temperature:      c.SamplingTemperature,
		RetryAttempts:    c.SamplingRetryAttempts,
		TokenBudget:      c.SamplingTokenBudget,
		BaseBackoff:      c.SamplingBaseBackoff,
		MaxBackoff:       c.SamplingMaxBackoff,
		StreamingEnabled: c.SamplingStreaming,
		RequestTimeout:   c.SamplingTimeout,
		Endpoint:         c.SamplingEndpoint,
		APIKey:           c.SamplingAPIKey,
	}
}

// ToPromptConfig creates a prompt configuration from the unified config
func (c *Config) ToPromptConfig() PromptConfig {
	return PromptConfig{
		TemplateDir:     c.PromptTemplateDir,
		EnableHotReload: c.PromptHotReload,
		AllowOverride:   c.PromptAllowOverride,
	}
}

// ToTracingConfig creates a tracing configuration from the unified config
func (c *Config) ToTracingConfig() TracingConfig {
	return TracingConfig{
		Enabled:     c.TracingEnabled,
		Endpoint:    c.TracingEndpoint,
		ServiceName: c.TracingServiceName,
		SampleRate:  c.TracingSampleRate,
	}
}

// ToSecurityConfig creates a security configuration from the unified config
func (c *Config) ToSecurityConfig() SecurityConfig {
	return SecurityConfig{
		ScanEnabled:    c.SecurityScanEnabled,
		Scanners:       c.SecurityScanners,
		FailOnHigh:     c.SecurityFailOnHigh,
		FailOnCritical: c.SecurityFailOnCritical,
	}
}

// ToRegistryConfig creates a registry configuration from the unified config
func (c *Config) ToRegistryConfig() RegistryConfig {
	return RegistryConfig{
		URL:      c.RegistryURL,
		Username: c.RegistryUsername,
		Password: c.RegistryPassword,
		Insecure: c.RegistryInsecure,
	}
}
