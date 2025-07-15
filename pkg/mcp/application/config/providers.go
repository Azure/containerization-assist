package config

import (
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/observability/tracing"
	"github.com/google/wire"
)

// Providers exports all configuration providers
var Providers = wire.NewSet(
	ProvideConfig,
	ProvideConfigFromServerConfig,
	ProvideTracingConfig,
	ProvideSecurityConfig,
	ProvideRegistryConfig,
)

// ProvideConfig creates a config instance from environment variables
func ProvideConfig() (*Config, error) {
	return Load(FromEnv())
}

// ProvideConfigFromServerConfig converts workflow.ServerConfig to Config
func ProvideConfigFromServerConfig(serverConfig workflow.ServerConfig) *Config {
	// Create a config with values from ServerConfig
	cfg := DefaultConfig()

	// Map the ServerConfig fields to Config
	cfg.WorkspaceDir = serverConfig.WorkspaceDir
	cfg.StorePath = serverConfig.StorePath
	cfg.SessionTTL = serverConfig.SessionTTL
	cfg.MaxSessions = serverConfig.MaxSessions
	cfg.MaxDiskPerSession = serverConfig.MaxDiskPerSession
	cfg.TotalDiskLimit = serverConfig.TotalDiskLimit
	cfg.CleanupInterval = serverConfig.CleanupInterval
	cfg.TransportType = serverConfig.TransportType
	cfg.HTTPAddr = serverConfig.HTTPAddr
	cfg.HTTPPort = serverConfig.HTTPPort
	cfg.CORSOrigins = serverConfig.CORSOrigins
	cfg.LogLevel = serverConfig.LogLevel
	cfg.LogHTTPBodies = serverConfig.LogHTTPBodies
	cfg.MaxBodyLogSize = serverConfig.MaxBodyLogSize

	// Load any additional values from environment
	loadFromEnv(cfg)

	return cfg
}

// ProvideTracingConfig extracts tracing configuration
func ProvideTracingConfig(cfg *Config) *tracing.Config {
	return &tracing.Config{
		Enabled:     cfg.TracingEnabled,
		Endpoint:    cfg.TracingEndpoint,
		ServiceName: cfg.TracingServiceName,
		SampleRate:  cfg.TracingSampleRate,
	}
}

// ProvideSecurityConfig extracts security configuration
func ProvideSecurityConfig(cfg *Config) *SecurityConfig {
	sc := cfg.ToSecurityConfig()
	return &sc
}

// ProvideRegistryConfig extracts registry configuration
func ProvideRegistryConfig(cfg *Config) *RegistryConfig {
	rc := cfg.ToRegistryConfig()
	return &rc
}
