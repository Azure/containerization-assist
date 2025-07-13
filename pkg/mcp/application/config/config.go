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

	// Transport settings
	TransportType string   `env:"MCP_TRANSPORT" yaml:"transport_type"`
	HTTPAddr      string   `env:"MCP_HTTP_ADDR" yaml:"http_addr"`
	HTTPPort      int      `env:"MCP_HTTP_PORT" yaml:"http_port"`
	CORSOrigins   []string `env:"MCP_CORS_ORIGINS" yaml:"cors_origins"`

	// Logging settings
	LogLevel string `env:"MCP_LOG_LEVEL" yaml:"log_level"`

	// Prompt settings
	PromptTemplateDir   string `env:"MCP_PROMPT_TEMPLATE_DIR" yaml:"prompt_template_dir"`
	PromptHotReload     bool   `env:"MCP_PROMPT_HOT_RELOAD" yaml:"prompt_hot_reload"`
	PromptAllowOverride bool   `env:"MCP_PROMPT_ALLOW_OVERRIDE" yaml:"prompt_allow_override"`

	// Sampling settings
	SamplingEndpoint    string  `env:"MCP_SAMPLING_ENDPOINT" yaml:"sampling_endpoint"`
	SamplingAPIKey      string  `env:"MCP_SAMPLING_API_KEY" yaml:"sampling_api_key"`
	SamplingMaxTokens   int32   `env:"MCP_SAMPLING_MAX_TOKENS" yaml:"sampling_max_tokens"`
	SamplingTemperature float32 `env:"MCP_SAMPLING_TEMPERATURE" yaml:"sampling_temperature"`
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
		WorkspaceDir:        "/tmp/mcp-workspace",
		StorePath:           "/tmp/mcp-store/sessions.db",
		SessionTTL:          24 * time.Hour,
		MaxSessions:         100,
		MaxDiskPerSession:   100 * 1024 * 1024,  // 100MB
		TotalDiskLimit:      1024 * 1024 * 1024, // 1GB
		TransportType:       "stdio",
		HTTPAddr:            "0.0.0.0",
		HTTPPort:            8080,
		CORSOrigins:         []string{"*"},
		LogLevel:            "info",
		PromptTemplateDir:   "",
		PromptHotReload:     false,
		PromptAllowOverride: false,
		SamplingEndpoint:    "",
		SamplingAPIKey:      "",
		SamplingMaxTokens:   4096,
		SamplingTemperature: 0.7,
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
	if v := os.Getenv("MCP_LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
	if v := os.Getenv("MCP_PROMPT_TEMPLATE_DIR"); v != "" {
		cfg.PromptTemplateDir = v
	}
	if v := os.Getenv("MCP_PROMPT_HOT_RELOAD"); v != "" {
		cfg.PromptHotReload = v == "true" || v == "1"
	}
	if v := os.Getenv("MCP_PROMPT_ALLOW_OVERRIDE"); v != "" {
		cfg.PromptAllowOverride = v == "true" || v == "1"
	}
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
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.WorkspaceDir == "" {
		return fmt.Errorf("workspace_dir is required")
	}
	if c.MaxSessions <= 0 {
		return fmt.Errorf("max_sessions must be positive")
	}
	if c.SessionTTL <= 0 {
		return fmt.Errorf("session_ttl must be positive")
	}
	if c.TransportType != "stdio" && c.TransportType != "http" {
		return fmt.Errorf("transport_type must be 'stdio' or 'http'")
	}
	if c.TransportType == "http" && c.HTTPPort <= 0 {
		return fmt.Errorf("http_port must be positive when using http transport")
	}
	if c.SamplingTemperature < 0 || c.SamplingTemperature > 2 {
		return fmt.Errorf("sampling_temperature must be between 0 and 2")
	}
	if c.SamplingMaxTokens <= 0 {
		return fmt.Errorf("sampling_max_tokens must be positive")
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
		TransportType:     c.TransportType,
		HTTPAddr:          c.HTTPAddr,
		HTTPPort:          c.HTTPPort,
		CORSOrigins:       c.CORSOrigins,
		LogLevel:          c.LogLevel,
	}
}
