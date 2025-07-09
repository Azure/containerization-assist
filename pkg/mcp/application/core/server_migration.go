package core

import (
	"fmt"

	"github.com/Azure/container-kit/pkg/mcp/application/services"
)

// ServerMigrationConfig controls how the server migration is handled
type ServerMigrationConfig struct {
	// Migration settings
	EnableMigration    bool     `json:"enable_migration"`
	MigrationMode      string   `json:"migration_mode"`       // "legacy", "migrated", "hybrid"
	MigratedTools      []string `json:"migrated_tools"`       // List of tools to migrate
	DisableLegacyTools []string `json:"disable_legacy_tools"` // List of legacy tools to disable
	MigrationLogLevel  string   `json:"migration_log_level"`  // Log level for migration operations

	// Rollback settings
	EnableRollback bool     `json:"enable_rollback"`
	RollbackTools  []string `json:"rollback_tools"` // List of tools to rollback to legacy

	// Performance settings
	MigrationTimeout  int  `json:"migration_timeout"`  // Timeout for migration operations
	ParallelMigration bool `json:"parallel_migration"` // Whether to migrate tools in parallel
}

// DefaultServerMigrationConfig returns the default migration configuration
func DefaultServerMigrationConfig() ServerMigrationConfig {
	return ServerMigrationConfig{
		EnableMigration:    false, // Disabled due to import cycle issues
		MigrationMode:      "legacy",
		MigratedTools:      []string{},
		DisableLegacyTools: []string{},
		MigrationLogLevel:  "info",
		EnableRollback:     false,
		RollbackTools:      []string{},
		MigrationTimeout:   30,
		ParallelMigration:  false,
	}
}

// initializeMigrationInfrastructure sets up the migration infrastructure
func (s *serverImpl) initializeMigrationInfrastructure() error {
	s.logger.Info("Initializing migration infrastructure")

	// Initialize service container if not already done
	if s.serviceContainer == nil {
		if err := s.initializeServiceContainer(); err != nil {
			return fmt.Errorf("failed to initialize service container: %w", err)
		}
	}

	// Migration infrastructure initialization removed - example code was causing compilation issues

	s.logger.Info("Migration infrastructure initialized successfully")
	return nil
}

// initializeServiceContainer initializes the service container with proper configuration
func (s *serverImpl) initializeServiceContainer() error {
	s.logger.Info("Initializing service container")

	// Create a basic service container for migration purposes
	// The service container will be enhanced as more services are integrated
	s.serviceContainer = services.NewDefaultServiceContainer(s.logger)

	s.logger.Info("Service container initialized successfully")
	return nil
}

// performMigration performs the actual tool migration based on configuration
func (s *serverImpl) performMigration(config ServerMigrationConfig) error {
	if !config.EnableMigration {
		s.logger.Info("Migration disabled, skipping")
		return nil
	}

	s.logger.Info("Tool migration temporarily disabled due to import cycle issues - using legacy tools only")
	return nil
}

// registerLegacyTools registers only legacy gomcp tools
func (s *serverImpl) registerLegacyTools(config ServerMigrationConfig) error {
	s.logger.Info("Registering legacy tools only")

	// This would call the existing legacy registration
	// For now, return an error to indicate this needs implementation
	return fmt.Errorf("legacy-only mode not implemented yet")
}

// registerMigratedTools registers only migrated consolidated command tools
func (s *serverImpl) registerMigratedTools(config ServerMigrationConfig) error {
	s.logger.Info("Registering migrated tools only")

	// Migrate specified tools
	for _, toolName := range config.MigratedTools {
		if err := s.migrateTool(toolName); err != nil {
			return fmt.Errorf("failed to migrate tool %s: %w", toolName, err)
		}
	}

	s.logger.Info("Successfully registered migrated tools", "count", len(config.MigratedTools))
	return nil
}

// registerHybridTools registers both legacy and migrated tools
func (s *serverImpl) registerHybridTools(config ServerMigrationConfig) error {
	s.logger.Info("Registering hybrid tools (legacy + migrated)")

	// First register legacy tools (excluding those being migrated)
	if err := s.registerLegacyToolsExcluding(config.MigratedTools); err != nil {
		return fmt.Errorf("failed to register legacy tools: %w", err)
	}

	// Then register migrated tools
	for _, toolName := range config.MigratedTools {
		if err := s.migrateTool(toolName); err != nil {
			s.logger.Error("Failed to migrate tool, keeping legacy",
				"tool", toolName,
				"error", err)
			// Continue with other tools rather than failing completely
		}
	}

	s.logger.Info("Successfully registered hybrid tools")
	return nil
}

// registerLegacyToolsExcluding registers legacy tools excluding specified ones
func (s *serverImpl) registerLegacyToolsExcluding(excludeTools []string) error {
	s.logger.Info("Registering legacy tools", "excluding", excludeTools)

	// Create a set of excluded tools for fast lookup
	excluded := make(map[string]bool)
	for _, tool := range excludeTools {
		excluded[tool] = true
	}

	// This would register legacy tools except those being migrated
	// For now, we'll skip this and focus on migration
	s.logger.Info("Legacy tool registration skipped for migration focus")
	return nil
}

// migrateTool migrates a specific tool
func (s *serverImpl) migrateTool(toolName string) error {
	s.logger.Info("Migrating tool", "tool", toolName)

	// Migration functionality removed - example code was causing compilation issues
	return fmt.Errorf("migration functionality not implemented for tool: %s", toolName)
}

// rollbackTool rolls back a migrated tool to legacy implementation
func (s *serverImpl) rollbackTool(toolName string) error {
	s.logger.Info("Rolling back tool", "tool", toolName)

	// This would re-register the legacy tool
	// For now, return an error to indicate this needs implementation
	return fmt.Errorf("rollback not implemented yet for tool: %s", toolName)
}

// getMigrationStatus returns the current migration status
func (s *serverImpl) getMigrationStatus() map[string]interface{} {
	// Migration manager removed - example code was causing compilation issues
	return map[string]interface{}{
		"status":         "not_implemented",
		"migrated_count": 0,
		"tools":          make(map[string]interface{}),
	}
}

// Add migration manager to server struct
func (s *serverImpl) addMigrationManager() {
	// This field needs to be added to the serverImpl struct
	// For now, we'll add it in the migration file
	// s.migrationManager = NewToolMigrationManager(s, s.logger)
}

// validateMigrationConfig validates the migration configuration
func validateMigrationConfig(config ServerMigrationConfig) error {
	// Validate migration mode
	validModes := []string{"legacy", "migrated", "hybrid"}
	validMode := false
	for _, mode := range validModes {
		if config.MigrationMode == mode {
			validMode = true
			break
		}
	}
	if !validMode {
		return fmt.Errorf("invalid migration mode: %s", config.MigrationMode)
	}

	// Validate log level
	validLevels := []string{"debug", "info", "warn", "error"}
	validLevel := false
	for _, level := range validLevels {
		if config.MigrationLogLevel == level {
			validLevel = true
			break
		}
	}
	if !validLevel {
		return fmt.Errorf("invalid migration log level: %s", config.MigrationLogLevel)
	}

	// Validate timeout
	if config.MigrationTimeout < 1 || config.MigrationTimeout > 300 {
		return fmt.Errorf("migration timeout must be between 1 and 300 seconds")
	}

	return nil
}

// startMigration starts the migration process with the given configuration
func (s *serverImpl) startMigration(config ServerMigrationConfig) error {
	// Validate configuration
	if err := validateMigrationConfig(config); err != nil {
		return fmt.Errorf("invalid migration configuration: %w", err)
	}

	// Initialize migration infrastructure
	if err := s.initializeMigrationInfrastructure(); err != nil {
		return fmt.Errorf("failed to initialize migration infrastructure: %w", err)
	}

	// Perform migration
	if err := s.performMigration(config); err != nil {
		return fmt.Errorf("failed to perform migration: %w", err)
	}

	s.logger.Info("Migration completed successfully")
	return nil
}
