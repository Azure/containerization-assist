package cmd

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Azure/containerization-assist/pkg/mcp/api"
	"github.com/Azure/containerization-assist/pkg/mcp/service"
	"github.com/Azure/containerization-assist/pkg/mcp/service/config"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Build-time variables set via ldflags
var (
	// Version is the semantic version of the application
	Version = "dev"
	// GitCommit is the git commit SHA at build time
	GitCommit = "unknown"
	// BuildTime is the time of the build
	BuildTime = "unknown"
)

// Simplified FlagConfig with only essential flags
type FlagConfig struct {
	configFile   *string
	workspaceDir *string
	storePath    *string
	sessionTTL   *string
	maxSessions  *int
	logLevel     *string
	version      *bool
	workflowMode *string
}

// Execute is the main entry point for the MCP server
func Execute() {
	// Parse command line flags
	flags := parseFlags()

	// Handle special flags that exit early
	handleSpecialFlags(flags)

	// Load and configure server
	config, err := loadAndConfigureServer(flags)
	if err != nil {
		log.Error().Err(err).Msg("Failed to configure server")
		os.Exit(1)
	}

	// Create and configure server
	mcpServer, err := createAndConfigureServer(config)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create server")
		os.Exit(1)
	}

	// Run server with graceful shutdown handling
	runServerWithShutdown(mcpServer)
}

// parseFlags parses essential command line flags only
func parseFlags() *FlagConfig {
	flags := &FlagConfig{
		configFile:   flag.String("config", "", "Path to .env configuration file"),
		workspaceDir: flag.String("workspace-dir", "", "Workspace directory"),
		storePath:    flag.String("store-path", "", "Session store path"),
		sessionTTL:   flag.String("session-ttl", "", "Session TTL (e.g., '24h')"),
		maxSessions:  flag.Int("max-sessions", 0, "Maximum number of sessions"),
		logLevel:     flag.String("log-level", "", "Log level (debug, info, warn, error)"),
		version:      flag.Bool("version", false, "Show version information"),
		workflowMode: flag.String("workflow-mode", "", "Workflow mode: 'automated' or 'interactive'"),
	}
	flag.Parse()
	return flags
}

// handleSpecialFlags handles version and schema export flags that exit early
func handleSpecialFlags(flags *FlagConfig) {
	if *flags.version {
		log.Info().Str("version", getVersion()).Msg("Containerization Assist MCP Server version")
		os.Exit(0)
	}
}

// loadAndConfigureServer loads simplified configuration
func loadAndConfigureServer(flags *FlagConfig) (*config.Config, error) {
	// Load configuration from environment variables
	cfg, err := config.Load(*flags.configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Apply flag overrides
	applyFlagOverrides(cfg, flags)

	// Setup structured logging
	setupLogging(cfg.LogLevel)

	return cfg, nil
}

// applyFlagOverrides applies command line flag overrides to configuration
func applyFlagOverrides(cfg *config.Config, flags *FlagConfig) {
	if *flags.workspaceDir != "" {
		cfg.WorkspaceDir = *flags.workspaceDir
	}
	if *flags.storePath != "" {
		cfg.StorePath = *flags.storePath
	}
	if *flags.sessionTTL != "" {
		if ttl, err := time.ParseDuration(*flags.sessionTTL); err == nil {
			cfg.SessionTTL = ttl
		}
	}
	if *flags.maxSessions > 0 {
		cfg.MaxSessions = *flags.maxSessions
	}
	if *flags.logLevel != "" {
		cfg.LogLevel = *flags.logLevel
	}
	if *flags.workflowMode != "" {
		cfg.WorkflowMode = *flags.workflowMode
	}
}

// createAndConfigureServer creates the MCP server with simplified configuration
func createAndConfigureServer(cfg *config.Config) (api.MCPServer, error) {
	log.Info().
		Str("version", getVersion()).
		Str("workspace_dir", cfg.WorkspaceDir).
		Msg("Starting Containerization Assist MCP Server")

	slogLogger := createSlogLogger(cfg.LogLevel)

	// Convert to server config and create server
	serverConfig := cfg.ToServerConfig()
	mcpServer, err := service.InitializeServer(slogLogger, serverConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create MCP server: %w", err)
	}
	return mcpServer, nil
}

// createSlogLogger creates a structured logger for dependency injection
func createSlogLogger(logLevel string) *slog.Logger {
	level := parseSlogLevel(logLevel)

	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	})

	return slog.New(handler)
}

// parseSlogLevel converts string log level to slog.Level
func parseSlogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// runServerWithShutdown runs the server with graceful shutdown handling
func runServerWithShutdown(mcpServer api.MCPServer) {
	ctx := context.Background()

	serverErr := make(chan error, 1)
	go func() {
		if err := mcpServer.Start(ctx); err != nil {
			serverErr <- err
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigChan:
		log.Info().Str("signal", sig.String()).Msg("Received shutdown signal")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := mcpServer.Stop(shutdownCtx); err != nil {
			log.Error().Err(err).Msg("Error during server shutdown")
		}

		// Wait a moment for final logs to be written
		time.Sleep(100 * time.Millisecond)

	case err := <-serverErr:
		log.Error().Err(err).Msg("Server failed")
		os.Exit(1)

	case <-ctx.Done():
		log.Info().Msg("Context cancelled, shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := mcpServer.Stop(shutdownCtx); err != nil {
			log.Error().Err(err).Msg("Error during server shutdown")
		}
	}
}

// setupLogging configures structured logging
func setupLogging(level string) {
	// Parse log level
	logLevel, err := zerolog.ParseLevel(level)
	if err != nil {
		logLevel = zerolog.InfoLevel
	}

	// Configure global logger
	zerolog.SetGlobalLevel(logLevel)

	// Use console writer for better readability
	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.RFC3339,
	})
}

// getVersion returns the version information
func getVersion() string {
	if Version == "dev" {
		return fmt.Sprintf("%s (commit: %s, built: %s)", Version, GitCommit, BuildTime)
	}
	return fmt.Sprintf("v%s (commit: %s, built: %s)", Version, GitCommit, BuildTime)
}
