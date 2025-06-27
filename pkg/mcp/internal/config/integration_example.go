package config

import (
	"fmt"

	"github.com/rs/zerolog"
)

// IntegrationExample demonstrates how to integrate the centralized configuration
// with existing MCP components

// Example: How to initialize configuration in main.go
func ExampleInitializeInMain() error {
	// This would typically be called in main.go
	if err := Initialize(""); err != nil {
		return fmt.Errorf("failed to initialize configuration: %w", err)
	}

	// Configuration is now available globally
	serverConfig := MustGetServer()
	fmt.Printf("Server will run on %s:%d\n", serverConfig.Host, serverConfig.Port)

	return nil
}

// Example: How to use configuration in a component
func ExampleComponentUsage(logger zerolog.Logger) error {
	// Get configuration for this component
	analyzerConfig, err := GetAnalyzer()
	if err != nil {
		return fmt.Errorf("failed to get analyzer config: %w", err)
	}

	// Use the configuration
	if analyzerConfig.EnableAI {
		logger.Info().Msg("AI analyzer is enabled")
	} else {
		logger.Info().Msg("AI analyzer is disabled")
	}

	// Configure component based on settings
	logger = logger.Level(parseLogLevel(analyzerConfig.AIAnalyzerLogLevel))

	return nil
}

// Example: How to migrate existing scattered configuration
func ExampleMigrateExistingCode() {
	// Before: Scattered environment variable loading
	// enableAI := os.Getenv("MCP_ENABLE_AI_ANALYZER") == "true"
	// logLevel := os.Getenv("MCP_ANALYZER_LOG_LEVEL")
	// if logLevel == "" {
	//     logLevel = "info"
	// }

	// After: Centralized configuration
	analyzerConfig := MustGetAnalyzer()
	enableAI := analyzerConfig.EnableAI
	logLevel := analyzerConfig.AIAnalyzerLogLevel

	// Use the values
	fmt.Printf("AI enabled: %t, Log level: %s\n", enableAI, logLevel)
}

// Example: How to handle Docker credentials migration
func ExampleDockerCredentialsMigration() {
	// Before: Direct environment variable access
	// username := os.Getenv("DOCKER_USERNAME")
	// password := os.Getenv("DOCKER_PASSWORD")
	// registry := os.Getenv("DOCKER_REGISTRY")
	// if registry == "" {
	//     registry = "docker.io"
	// }

	// After: Centralized configuration with migration support
	dockerConfig := MustGetDocker()

	// Check for legacy environment variables and issue warnings
	helper := NewMigrationHelper(zerolog.Nop())
	helper.BackwardCompatibilityWarnings()

	// Use the standardized configuration
	username := dockerConfig.Username
	password := dockerConfig.Password
	registry := dockerConfig.Registry

	fmt.Printf("Docker config - Registry: %s, User: %s, Has Password: %t\n",
		registry, username, password != "")
}

// Example: How to use configuration in server initialization
func ExampleServerInitialization() error {
	serverConfig, err := GetServer()
	if err != nil {
		return err
	}

	transportConfig, err := GetTransport()
	if err != nil {
		return err
	}

	observabilityConfig, err := GetObservability()
	if err != nil {
		return err
	}

	// Use configurations to set up server
	fmt.Printf("Starting server on %s:%d\n", serverConfig.Host, serverConfig.Port)
	fmt.Printf("Transport type: %s\n", transportConfig.Type)
	fmt.Printf("Observability enabled: tracing=%t, metrics=%t\n",
		observabilityConfig.EnableTracing, observabilityConfig.EnableMetrics)

	return nil
}

// Example: How to handle configuration in tests
func ExampleTestConfiguration() {
	// Save original config
	originalConfig, _ := Get()

	// Set up test configuration
	testConfig := NewConfigManager()
	testConfig.Server.Port = 9999
	testConfig.Analyzer.EnableAI = true
	SetTestConfig(testConfig)

	// Run tests with test configuration
	serverConfig := MustGetServer()
	fmt.Printf("Test server port: %d\n", serverConfig.Port)

	// Restore original configuration (in real tests, use defer)
	if originalConfig != nil {
		SetTestConfig(originalConfig)
	} else {
		Reset()
	}
}

// Helper function to parse log level
func parseLogLevel(level string) zerolog.Level {
	switch level {
	case "debug":
		return zerolog.DebugLevel
	case "info":
		return zerolog.InfoLevel
	case "warn":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	default:
		return zerolog.InfoLevel
	}
}
