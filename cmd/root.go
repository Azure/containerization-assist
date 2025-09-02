package cmd

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Azure/containerization-assist/pkg/api"
	"github.com/Azure/containerization-assist/pkg/service"
	"github.com/Azure/containerization-assist/pkg/service/config"
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
	// Global log file handle for proper cleanup
	logFileHandle *os.File
)

// Simplified FlagConfig with only essential flags
type FlagConfig struct {
	configFile   *string
	workspaceDir *string
	storePath    *string
	sessionTTL   *string
	maxSessions  *int
	logLevel     *string
	logFile      *string
	version      *bool
	workflowMode *string
}

// Execute is the main entry point for the MCP server
func Execute() {
	// Check if running in tool mode
	if len(os.Args) > 1 && os.Args[1] == "tool" {
		handleToolMode()
		return
	}

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
		logFile:      flag.String("log-file", "", "Path to log file (logs to stderr if not specified)"),
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
	setupLogging(cfg.LogLevel, cfg.LogFile)

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
	if *flags.logFile != "" {
		cfg.LogFile = *flags.logFile
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

	slogLogger := createSlogLogger(cfg.LogLevel, cfg.LogFile)

	// Convert to server config and create server
	serverConfig := cfg.ToServerConfig()
	mcpServer, err := service.InitializeServer(slogLogger, serverConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create MCP server: %w", err)
	}
	return mcpServer, nil
}

// createSlogLogger creates a structured logger for dependency injection
func createSlogLogger(logLevel string, logFile string) *slog.Logger {
	level := parseSlogLevel(logLevel)

	// If no log file specified, use original behavior (stderr only)
	if logFile == "" {
		handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: level,
		})
		return slog.New(handler)
	}

	// Log file specified - setup dual output (stderr + file)
	var writers []io.Writer

	// Always include stderr
	writers = append(writers, os.Stderr)

	// Add file writer if we can open it
	if err := os.MkdirAll(filepath.Dir(logFile), 0755); err == nil {
		if file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
			writers = append(writers, file)
		}
	}

	// Create output writer
	var output io.Writer
	if len(writers) > 1 {
		output = io.MultiWriter(writers...)
	} else {
		output = writers[0]
	}

	// Create slog handler with the combined output
	handler := slog.NewJSONHandler(output, &slog.HandlerOptions{
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

		// Flush and close log file if it exists
		if logFileHandle != nil {
			logFileHandle.Sync()
			logFileHandle.Close()
		}

	case err := <-serverErr:
		log.Error().Err(err).Msg("Server failed")

		// Flush and close log file if it exists
		if logFileHandle != nil {
			logFileHandle.Sync()
			logFileHandle.Close()
		}
		os.Exit(1)

	case <-ctx.Done():
		log.Info().Msg("Context cancelled, shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := mcpServer.Stop(shutdownCtx); err != nil {
			log.Error().Err(err).Msg("Error during server shutdown")
		}

		// Flush and close log file if it exists
		if logFileHandle != nil {
			logFileHandle.Sync()
			logFileHandle.Close()
		}
	}
}

// setupLogging configures structured logging
func setupLogging(level string, logFile string) {
	// Parse log level
	logLevel, err := zerolog.ParseLevel(level)
	if err != nil {
		logLevel = zerolog.InfoLevel
	}

	// Configure global logger
	zerolog.SetGlobalLevel(logLevel)

	// If no log file specified, use original behavior (console writer only)
	if logFile == "" {
		// Use console writer for better readability (original behavior)
		log.Logger = log.Output(zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: time.RFC3339,
		})
		return
	}

	// Log file specified - setup dual output (console + file)
	var writers []io.Writer

	// Always include stderr for console output
	writers = append(writers, zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.RFC3339,
	})

	// Create log file directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(logFile), 0755); err != nil {
		log.Error().Err(err).Str("log_file", logFile).Msg("Failed to create log file directory")
		return
	}

	// Open log file for writing (create if doesn't exist, append if it does)
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Error().Err(err).Str("log_file", logFile).Msg("Failed to open log file")
		return
	}

	logFileHandle = file
	// Add file writer with JSON format for structured logging
	writers = append(writers, file)

	// Configure logger with multiple outputs
	log.Logger = zerolog.New(io.MultiWriter(writers...)).With().Timestamp().Logger()

	// Make sure to sync the file handle and log the success
	logFileHandle.Sync()
	log.Info().Str("log_file", logFile).Msg("Logging to file enabled")
}

// getVersion returns the version information
func getVersion() string {
	if Version == "dev" {
		return fmt.Sprintf("%s (commit: %s, built: %s)", Version, GitCommit, BuildTime)
	}
	return fmt.Sprintf("v%s (commit: %s, built: %s)", Version, GitCommit, BuildTime)
}
