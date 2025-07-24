package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/Azure/container-kit/pkg/mcp/service" // Direct dependency injection
	"github.com/joho/godotenv"
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

// FlagConfig holds all command line flags
type FlagConfig struct {
	configFile        *string
	workspaceDir      *string
	storePath         *string
	maxSessions       *int
	sessionTTL        *string
	maxDiskPerSession *string
	totalDiskLimit    *string
	transportType     *string
	httpAddr          *string
	httpPort          *int
	logLevel          *string
	logHTTPBodies     *bool
	maxBodyLogSize    *string
	sandboxEnabled    *bool
	telemetryEnabled  *bool
	telemetryPort     *int
	otelEnabled       *bool
	otelEndpoint      *string
	otelHeaders       *string
	serviceName       *string
	serviceVersion    *string
	environment       *string
	traceSampleRate   *float64
	version           *bool
	demo              *string
	workflowMode      *string
}

// parseFlags parses command line flags and returns configuration
func parseFlags() *FlagConfig {
	flags := &FlagConfig{
		configFile:        flag.String("config", "", "Path to configuration file"),
		workspaceDir:      flag.String("workspace-dir", "", "Workspace directory"),
		storePath:         flag.String("store-path", "", "Session store path"),
		maxSessions:       flag.Int("max-sessions", 0, "Maximum number of sessions"),
		sessionTTL:        flag.String("session-ttl", "", "Session TTL (e.g., '24h')"),
		maxDiskPerSession: flag.String("max-disk-per-session", "", "Max disk per session (bytes)"),
		totalDiskLimit:    flag.String("total-disk-limit", "", "Total disk limit (bytes)"),
		transportType:     flag.String("transport", "", "Transport type (stdio, http)"),
		httpAddr:          flag.String("http-addr", "", "HTTP address"),
		httpPort:          flag.Int("http-port", 0, "HTTP port"),
		logLevel:          flag.String("log-level", "", "Log level (debug, info, warn, error)"),
		logHTTPBodies:     flag.Bool("log-http-bodies", false, "Log HTTP request/response bodies"),
		maxBodyLogSize:    flag.String("max-body-log-size", "", "Maximum size of bodies to log (bytes)"),
		sandboxEnabled:    flag.Bool("sandbox", false, "Enable sandboxed execution"),
		telemetryEnabled:  flag.Bool("telemetry", true, "Enable Prometheus metrics"),
		telemetryPort:     flag.Int("telemetry-port", 9090, "Port for Prometheus metrics endpoint"),
		otelEnabled:       flag.Bool("otel", false, "Enable OpenTelemetry tracing"),
		otelEndpoint:      flag.String("otel-endpoint", "", "OpenTelemetry OTLP endpoint (e.g., http://localhost:4318/v1/traces)"),
		otelHeaders:       flag.String("otel-headers", "", "OpenTelemetry OTLP headers (comma-separated key=value pairs)"),
		serviceName:       flag.String("service-name", "container-kit-mcp", "Service name for OpenTelemetry"),
		serviceVersion:    flag.String("service-version", "", "Service version for OpenTelemetry"),
		environment:       flag.String("environment", "development", "Environment name for OpenTelemetry"),
		traceSampleRate:   flag.Float64("trace-sample-rate", 1.0, "Trace sampling rate (0.0-1.0)"),
		version:           flag.Bool("version", false, "Show version information"),
		demo:              flag.String("demo", "", "Run demo mode: all, basic, errors, session, performance, metrics"),
		workflowMode:      flag.String("workflow-mode", "", "Workflow mode: 'automated' (promotes start_workflow) or 'interactive' (deprecates start_workflow)"),
	}
	flag.Parse()
	return flags
}

// handleSpecialFlags handles version and schema export flags that exit early
func handleSpecialFlags(flags *FlagConfig) {
	if *flags.version {
		log.Info().Str("version", getVersion()).Msg("Container Kit MCP Server version")
		os.Exit(0)
	}

}

func main() {
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
	mcpServer, err := createAndConfigureServer(config, flags)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create server")
		os.Exit(1)
	}

	// Handle demo mode
	if *flags.demo != "" {
		log.Warn().Msg("Demo mode temporarily disabled due to API restructuring")
		return
	}

	// Run server with graceful shutdown handling
	runServerWithShutdown(mcpServer)
}

// loadAndConfigureServer loads configuration and applies flag overrides
func loadAndConfigureServer(flags *FlagConfig) (workflow.ServerConfig, error) {
	// Load configuration
	config, err := loadConfig(*flags.configFile, flags.telemetryEnabled, flags.telemetryPort)
	if err != nil {
		return config, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Apply basic configuration overrides
	applyBasicConfigOverrides(&config, flags)

	// Apply OpenTelemetry configuration overrides
	applyOTELConfigOverrides(&config, flags)

	// Setup structured logging
	setupLogging(config.LogLevel)

	return config, nil
}

// applyBasicConfigOverrides applies basic flag overrides to configuration
func applyBasicConfigOverrides(config *workflow.ServerConfig, flags *FlagConfig) {
	if *flags.workspaceDir != "" {
		config.WorkspaceDir = *flags.workspaceDir
	}
	if *flags.storePath != "" {
		config.StorePath = *flags.storePath
	}
	if *flags.maxSessions > 0 {
		config.MaxSessions = *flags.maxSessions
	}
	if *flags.sessionTTL != "" {
		if ttl, err := time.ParseDuration(*flags.sessionTTL); err == nil {
			config.SessionTTL = ttl
		}
	}
	if *flags.maxDiskPerSession != "" {
		if bytes, err := strconv.ParseInt(*flags.maxDiskPerSession, 10, 64); err == nil {
			config.MaxDiskPerSession = bytes
		}
	}
	if *flags.totalDiskLimit != "" {
		if bytes, err := strconv.ParseInt(*flags.totalDiskLimit, 10, 64); err == nil {
			config.TotalDiskLimit = bytes
		}
	}
	if *flags.transportType != "" {
		config.TransportType = *flags.transportType
	}
	if *flags.httpAddr != "" {
		config.HTTPAddr = *flags.httpAddr
	}
	if *flags.httpPort > 0 {
		config.HTTPPort = *flags.httpPort
	}
	if *flags.logHTTPBodies {
		config.LogHTTPBodies = true
	}
	if *flags.maxBodyLogSize != "" {
		if bytes, err := strconv.ParseInt(*flags.maxBodyLogSize, 10, 64); err == nil {
			config.MaxBodyLogSize = bytes
		}
	}
	if *flags.logLevel != "" {
		config.LogLevel = *flags.logLevel
	}
	if *flags.sandboxEnabled {
		config.SandboxEnabled = true
	}
	if *flags.workflowMode != "" {
		config.WorkflowMode = *flags.workflowMode
	}
}

// applyOTELConfigOverrides applies OpenTelemetry flag overrides to configuration
func applyOTELConfigOverrides(config *workflow.ServerConfig, flags *FlagConfig) {
	if *flags.serviceName != "" {
		config.ServiceName = *flags.serviceName
	}
	if *flags.serviceVersion != "" {
		config.ServiceVersion = *flags.serviceVersion
	}
	if *flags.environment != "" {
		config.Environment = *flags.environment
	}
	// Note: OTEL fields removed as part of DELTA observability cleanup
}

// parseOTELHeaders parses comma-separated key=value pairs into a map
func parseOTELHeaders(headers string) map[string]string {
	headerMap := make(map[string]string)
	pairs := strings.Split(headers, ",")
	for _, pair := range pairs {
		if kv := strings.SplitN(strings.TrimSpace(pair), "=", 2); len(kv) == 2 {
			headerMap[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}
	return headerMap
}

// createAndConfigureServer creates the MCP server for workflow-only operation
func createAndConfigureServer(config workflow.ServerConfig, flags *FlagConfig) (api.MCPServer, error) {
	log.Info().
		Str("version", getVersion()).
		Str("transport", config.TransportType).
		Str("workspace_dir", config.WorkspaceDir).
		Msg("Starting Container Kit MCP Server")

	slogLogger := createSlogLogger(config.LogLevel)

	// Create server using direct dependency injection
	mcpServer, err := service.InitializeServer(slogLogger, config)
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

// EnvConfigMapping defines how environment variables map to configuration fields
type EnvConfigMapping struct {
	EnvKey string
	Type   string
	Setter func(config *workflow.ServerConfig, value string) error
}

// buildEnvMappings creates the environment variable to config field mappings
func buildEnvMappings() []EnvConfigMapping {
	return []EnvConfigMapping{
		{"CONTAINER_KIT_WORKSPACE_DIR", "string", func(config *workflow.ServerConfig, value string) error {
			config.WorkspaceDir = value
			return nil
		}},
		{"CONTAINER_KIT_STORE_PATH", "string", func(config *workflow.ServerConfig, value string) error {
			config.StorePath = value
			return nil
		}},
		{"CONTAINER_KIT_MAX_SESSIONS", "int", func(config *workflow.ServerConfig, value string) error {
			if parsed, err := strconv.Atoi(value); err == nil {
				config.MaxSessions = parsed
			}
			return nil
		}},
		{"CONTAINER_KIT_SESSION_TTL", "duration", func(config *workflow.ServerConfig, value string) error {
			if parsed, err := time.ParseDuration(value); err == nil {
				config.SessionTTL = parsed
			}
			return nil
		}},
		{"CONTAINER_KIT_MAX_DISK_PER_SESSION", "int64", func(config *workflow.ServerConfig, value string) error {
			if parsed, err := strconv.ParseInt(value, 10, 64); err == nil {
				config.MaxDiskPerSession = parsed
			}
			return nil
		}},
		{"CONTAINER_KIT_TOTAL_DISK_LIMIT", "int64", func(config *workflow.ServerConfig, value string) error {
			if parsed, err := strconv.ParseInt(value, 10, 64); err == nil {
				config.TotalDiskLimit = parsed
			}
			return nil
		}},
		{"CONTAINER_KIT_TRANSPORT", "string", func(config *workflow.ServerConfig, value string) error {
			config.TransportType = value
			return nil
		}},
		{"CONTAINER_KIT_HTTP_ADDR", "string", func(config *workflow.ServerConfig, value string) error {
			config.HTTPAddr = value
			return nil
		}},
		{"CONTAINER_KIT_HTTP_PORT", "int", func(config *workflow.ServerConfig, value string) error {
			if parsed, err := strconv.Atoi(value); err == nil {
				config.HTTPPort = parsed
			}
			return nil
		}},
		{"CONTAINER_KIT_LOG_LEVEL", "string", func(config *workflow.ServerConfig, value string) error {
			config.LogLevel = value
			return nil
		}},
		{"CONTAINER_KIT_LOG_HTTP_BODIES", "bool", func(config *workflow.ServerConfig, value string) error {
			config.LogHTTPBodies = value == "true" || value == "1"
			return nil
		}},
		{"CONTAINER_KIT_MAX_BODY_LOG_SIZE", "int64", func(config *workflow.ServerConfig, value string) error {
			if parsed, err := strconv.ParseInt(value, 10, 64); err == nil {
				config.MaxBodyLogSize = parsed
			}
			return nil
		}},
		{"CONTAINER_KIT_SANDBOX_ENABLED", "bool", func(config *workflow.ServerConfig, value string) error {
			config.SandboxEnabled = value == "true" || value == "1"
			return nil
		}},
		{"CONTAINER_KIT_CLEANUP_INTERVAL", "duration", func(config *workflow.ServerConfig, value string) error {
			if parsed, err := time.ParseDuration(value); err == nil {
				config.CleanupInterval = parsed
			}
			return nil
		}},
		{"CONTAINER_KIT_SERVICE_NAME", "string", func(config *workflow.ServerConfig, value string) error {
			config.ServiceName = value
			return nil
		}},
		{"CONTAINER_KIT_SERVICE_VERSION", "string", func(config *workflow.ServerConfig, value string) error {
			config.ServiceVersion = value
			return nil
		}},
		{"CONTAINER_KIT_ENVIRONMENT", "string", func(config *workflow.ServerConfig, value string) error {
			config.Environment = value
			return nil
		}},
		{"CONTAINER_KIT_TRACE_SAMPLE_RATE", "float64", func(config *workflow.ServerConfig, value string) error {
			if parsed, err := strconv.ParseFloat(value, 64); err == nil {
				config.TraceSampleRate = parsed
			}
			return nil
		}},
		{"CONTAINER_KIT_WORKFLOW_MODE", "string", func(config *workflow.ServerConfig, value string) error {
			config.WorkflowMode = value
			return nil
		}},
	}
}

// loadConfig loads configuration from environment variables and config file
func loadConfig(configFile string, telemetryEnabled *bool, telemetryPort *int) (workflow.ServerConfig, error) {
	config := workflow.DefaultServerConfig()

	if err := loadEnvFile(configFile); err != nil {
		return config, err
	}

	if err := applyEnvMappings(&config); err != nil {
		return config, err
	}

	applyTelemetryConfig(telemetryEnabled, telemetryPort)

	if err := ensureDirectoriesExist(config); err != nil {
		return config, err
	}

	return config, nil
}

// loadEnvFile loads environment variables from file
func loadEnvFile(configFile string) error {
	if configFile != "" {
		if err := godotenv.Load(configFile); err != nil {
			return fmt.Errorf("failed to load config file %s: %w", configFile, err)
		}
	} else {
		if _, err := os.Stat(".env"); err == nil {
			godotenv.Load(".env")
		}
	}
	return nil
}

// applyEnvMappings applies environment variable mappings to configuration
func applyEnvMappings(config *workflow.ServerConfig) error {
	mappings := buildEnvMappings()
	for _, mapping := range mappings {
		if val := os.Getenv(mapping.EnvKey); val != "" {
			if err := mapping.Setter(config, val); err != nil {
				return fmt.Errorf("failed to set %s: %w", mapping.EnvKey, err)
			}
		}
	}
	return nil
}

// applyTelemetryConfig applies telemetry-specific environment variables
func applyTelemetryConfig(telemetryEnabled *bool, telemetryPort *int) {
	if val := os.Getenv("CONTAINER_KIT_TELEMETRY_ENABLED"); val != "" {
		*telemetryEnabled = val == "true" || val == "1"
	}
	if val := os.Getenv("CONTAINER_KIT_TELEMETRY_PORT"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			*telemetryPort = parsed
		}
	}
}

// ensureDirectoriesExist creates required directories
func ensureDirectoriesExist(config workflow.ServerConfig) error {
	if err := os.MkdirAll(config.WorkspaceDir, 0755); err != nil {
		return fmt.Errorf("failed to create workspace directory: %w", err)
	}

	if config.StorePath != "" {
		storeDir := filepath.Dir(config.StorePath)
		if err := os.MkdirAll(storeDir, 0755); err != nil {
			return fmt.Errorf("failed to create store directory: %w", err)
		}
	}

	return nil
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
