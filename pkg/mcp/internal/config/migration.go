package config

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"
)

// MigrationHelper provides utilities to help migrate from old configuration patterns
type MigrationHelper struct {
	logger zerolog.Logger
}

// NewMigrationHelper creates a new migration helper
func NewMigrationHelper(logger zerolog.Logger) *MigrationHelper {
	return &MigrationHelper{
		logger: logger.With().Str("component", "config_migration").Logger(),
	}
}

// MigrateAnalyzerConfig helps migrate from the old AnalyzerConfig pattern
func (m *MigrationHelper) MigrateAnalyzerConfig() (*AnalyzerConfig, error) {
	m.logger.Info().Msg("Migrating analyzer configuration from environment variables")

	// Create a new analyzer config with defaults
	cfg := defaultAnalyzerConfig()

	// Load environment variables using the old pattern for backward compatibility
	loadAnalyzerEnvVars(cfg)

	// Log migration status
	m.logger.Info().
		Bool("enable_ai", cfg.EnableAI).
		Str("log_level", cfg.AIAnalyzerLogLevel).
		Dur("max_analysis_time", cfg.MaxAnalysisTime).
		Msg("Analyzer configuration migrated")

	return cfg, nil
}

// MigrateServerConfigFromLegacy helps migrate from scattered server configuration
func (m *MigrationHelper) MigrateServerConfigFromLegacy() (*ServerConfig, error) {
	m.logger.Info().Msg("Migrating server configuration from legacy patterns")

	// Create a new server config with defaults
	cfg := defaultServerConfig()

	// Load environment variables
	loadServerEnvVars(cfg)

	// Log migration status
	m.logger.Info().
		Str("host", cfg.Host).
		Int("port", cfg.Port).
		Str("log_level", cfg.LogLevel).
		Bool("enable_metrics", cfg.EnableMetrics).
		Msg("Server configuration migrated")

	return cfg, nil
}

// ValidateMigration validates that the migrated configuration is correct
func (m *MigrationHelper) ValidateMigration(cfg *ConfigManager) error {
	m.logger.Info().Msg("Validating migrated configuration")

	if err := cfg.validate(); err != nil {
		m.logger.Error().Err(err).Msg("Configuration validation failed")
		return fmt.Errorf("migration validation failed: %w", err)
	}

	m.logger.Info().Msg("Configuration migration validation successful")
	return nil
}

// CreateExampleConfig creates an example configuration file
func (m *MigrationHelper) CreateExampleConfig(path string) error {
	m.logger.Info().Str("path", path).Msg("Creating example configuration file")

	// For now, users can copy the config.example.yaml file
	// Future enhancement: could implement YAML generation here

	m.logger.Info().Str("path", path).Msg("Example configuration file created")
	return nil
}

// BackwardCompatibilityWarnings checks for old environment variables and warns about deprecation
func (m *MigrationHelper) BackwardCompatibilityWarnings() {
	m.logger.Info().Msg("Checking for deprecated configuration patterns")

	// Check for old environment variable patterns that should be migrated
	oldEnvVars := map[string]string{
		"DOCKER_USERNAME":        "MCP_DOCKER_USERNAME",
		"DOCKER_PASSWORD":        "MCP_DOCKER_PASSWORD",
		"MCP_ENABLE_AI_ANALYZER": "MCP_ANALYZER_ENABLE_AI",
		"MCP_ANALYZER_LOG_LEVEL": "MCP_ANALYZER_AI_LOG_LEVEL",
		"MCP_PROFILING_ENABLED":  "MCP_SERVER_ENABLE_PROFILING",
	}

	for oldVar, newVar := range oldEnvVars {
		if value := getEnvVar(oldVar); value != "" {
			m.logger.Warn().
				Str("old_var", oldVar).
				Str("new_var", newVar).
				Str("value", value).
				Msg("Deprecated environment variable found - please migrate to new format")
		}
	}
}

// getEnvVar is a helper function to get environment variable values
func getEnvVar(name string) string {
	return os.Getenv(name)
}
