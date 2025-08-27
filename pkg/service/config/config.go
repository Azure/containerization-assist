package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/Azure/containerization-assist/pkg/domain/workflow"
	"github.com/joho/godotenv"
)

// Simplified configuration with only essential fields
type Config struct {
	// Core server settings
	WorkspaceDir string        `env:"MCP_WORKSPACE_DIR"`
	StorePath    string        `env:"MCP_STORE_PATH"`
	SessionTTL   time.Duration `env:"MCP_SESSION_TTL"`

	// Session management (essential for functionality)
	MaxSessions int `env:"MCP_MAX_SESSIONS"`

	// Logging settings
	LogLevel string `env:"MCP_LOG_LEVEL"`

	// Service identification
	ServiceName    string `env:"MCP_SERVICE_NAME"`
	ServiceVersion string `env:"MCP_SERVICE_VERSION"`

	// Container registry
	RegistryURL      string `env:"MCP_REGISTRY_URL"`
	RegistryUsername string `env:"MCP_REGISTRY_USERNAME"`
	RegistryPassword string `env:"MCP_REGISTRY_PASSWORD"`

	// Workflow mode
	WorkflowMode string `env:"MCP_WORKFLOW_MODE"`
}

// No additional config types needed - simplified to single Config struct

// Simple configuration loading - just use defaults + environment variables
func Load(envFile string) (*Config, error) {
	cfg := DefaultConfig()

	// Load environment file if specified
	if envFile != "" {
		if err := godotenv.Load(envFile); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to load .env file: %w", err)
		}
	}

	// Apply environment variables
	loadFromEnv(cfg)

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

func DefaultConfig() *Config {
	// Create process-specific paths to avoid conflicts between multiple instances
	pid := os.Getpid()
	storePath := filepath.Join(os.TempDir(), fmt.Sprintf("sessions-%d.db", pid))
	workspaceDir := filepath.Join(os.TempDir(), fmt.Sprintf("mcp-workspace-%d", pid))

	return &Config{
		WorkspaceDir:     workspaceDir,
		StorePath:        storePath,
		SessionTTL:       24 * time.Hour,
		MaxSessions:      100,
		LogLevel:         "info",
		ServiceName:      "containerization-assist-mcp",
		ServiceVersion:   "dev",
		RegistryURL:      "",
		RegistryUsername: "",
		RegistryPassword: "",
		WorkflowMode:     "interactive",
	}
}

// Simple environment variable loading for essential fields only
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
	if v := os.Getenv("MCP_LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
	if v := os.Getenv("MCP_SERVICE_NAME"); v != "" {
		cfg.ServiceName = v
	}
	if v := os.Getenv("MCP_SERVICE_VERSION"); v != "" {
		cfg.ServiceVersion = v
	}
	if v := os.Getenv("MCP_REGISTRY_URL"); v != "" {
		cfg.RegistryURL = v
	}
	if v := os.Getenv("MCP_REGISTRY_USERNAME"); v != "" {
		cfg.RegistryUsername = v
	}
	if v := os.Getenv("MCP_REGISTRY_PASSWORD"); v != "" {
		cfg.RegistryPassword = v
	}
	if v := os.Getenv("MCP_WORKFLOW_MODE"); v != "" {
		cfg.WorkflowMode = v
	}
}

// Simple validation for essential fields only
func (c *Config) Validate() error {
	if c.WorkspaceDir == "" {
		return fmt.Errorf("workspace_dir is required")
	}
	if c.SessionTTL <= 0 {
		return fmt.Errorf("session_ttl must be positive")
	}
	if c.MaxSessions <= 0 {
		return fmt.Errorf("max_sessions must be positive")
	}
	validLogLevels := []string{"debug", "info", "warn", "error"}
	valid := false
	for _, level := range validLogLevels {
		if c.LogLevel == level {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("log_level must be one of: debug, info, warn, error")
	}
	return nil
}

// Simple direct conversion - no intermediate config types needed
func (c *Config) ToServerConfig() workflow.ServerConfig {
	return workflow.ServerConfig{
		WorkspaceDir:     c.WorkspaceDir,
		StorePath:        c.StorePath,
		SessionTTL:       c.SessionTTL,
		MaxSessions:      c.MaxSessions,
		LogLevel:         c.LogLevel,
		ServiceName:      c.ServiceName,
		ServiceVersion:   c.ServiceVersion,
		RegistryURL:      c.RegistryURL,
		RegistryUsername: c.RegistryUsername,
		RegistryPassword: c.RegistryPassword,
		WorkflowMode:     c.WorkflowMode,
	}
}
