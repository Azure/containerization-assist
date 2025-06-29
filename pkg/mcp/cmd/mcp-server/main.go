package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Azure/container-kit/pkg/mcp"
	"github.com/Azure/container-kit/pkg/mcp/factory"
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

func main() {
	// Parse command line flags
	var (
		configFile        = flag.String("config", "", "Path to configuration file")
		workspaceDir      = flag.String("workspace-dir", "", "Workspace directory")
		storePath         = flag.String("store-path", "", "Session store path")
		maxSessions       = flag.Int("max-sessions", 0, "Maximum number of sessions")
		sessionTTL        = flag.String("session-ttl", "", "Session TTL (e.g., '24h')")
		maxDiskPerSession = flag.String("max-disk-per-session", "", "Max disk per session (bytes)")
		totalDiskLimit    = flag.String("total-disk-limit", "", "Total disk limit (bytes)")
		transportType     = flag.String("transport", "", "Transport type (stdio, http)")
		httpAddr          = flag.String("http-addr", "", "HTTP address")
		httpPort          = flag.Int("http-port", 0, "HTTP port")
		logLevel          = flag.String("log-level", "", "Log level (debug, info, warn, error)")
		logHTTPBodies     = flag.Bool("log-http-bodies", false, "Log HTTP request/response bodies")
		maxBodyLogSize    = flag.String("max-body-log-size", "", "Maximum size of bodies to log (bytes)")
		sandboxEnabled    = flag.Bool("sandbox", false, "Enable sandboxed execution")
		conversationMode  = flag.Bool("conversation", false, "Enable conversation mode (chat tool)")
		telemetryEnabled  = flag.Bool("telemetry", true, "Enable Prometheus metrics")
		telemetryPort     = flag.Int("telemetry-port", 9090, "Port for Prometheus metrics endpoint")
		otelEnabled       = flag.Bool("otel", false, "Enable OpenTelemetry tracing")
		otelEndpoint      = flag.String("otel-endpoint", "", "OpenTelemetry OTLP endpoint (e.g., http://localhost:4318/v1/traces)")
		otelHeaders       = flag.String("otel-headers", "", "OpenTelemetry OTLP headers (comma-separated key=value pairs)")
		serviceName       = flag.String("service-name", "container-kit-mcp", "Service name for OpenTelemetry")
		serviceVersion    = flag.String("service-version", "", "Service version for OpenTelemetry")
		environment       = flag.String("environment", "development", "Environment name for OpenTelemetry")
		traceSampleRate   = flag.Float64("trace-sample-rate", 1.0, "Trace sampling rate (0.0-1.0)")
		version           = flag.Bool("version", false, "Show version information")
		demo              = flag.String("demo", "", "Run demo mode: all, basic, errors, session, performance, metrics")
		exportSchemas     = flag.Bool("export-schemas", false, "Export tool schemas to docs/tools.schema.json and exit")
		schemaOutput      = flag.String("schema-output", "docs/tools.schema.json", "Output path for exported schemas")
	)
	flag.Parse()

	if *version {
		log.Info().Str("version", getVersion()).Msg("Container Kit MCP Server version")
		os.Exit(0)
	}

	if *exportSchemas {
		if err := exportToolSchemas(*schemaOutput); err != nil {
			log.Error().Err(err).Msg("Failed to export schemas")
			os.Exit(1)
		}
		log.Info().Str("output_file", *schemaOutput).Msg("Tool schemas exported successfully")
		os.Exit(0)
	}

	// Load configuration
	config, err := loadConfig(*configFile, telemetryEnabled, telemetryPort)
	if err != nil {
		log.Error().Err(err).Msg("Failed to load configuration")
		os.Exit(1)
	}

	// Override config with command line flags
	if *workspaceDir != "" {
		config.WorkspaceDir = *workspaceDir
	}
	if *storePath != "" {
		config.StorePath = *storePath
	}
	if *maxSessions > 0 {
		config.MaxSessions = *maxSessions
	}
	if *sessionTTL != "" {
		if ttl, err := time.ParseDuration(*sessionTTL); err == nil {
			config.SessionTTL = ttl
		}
	}
	if *maxDiskPerSession != "" {
		if bytes, err := strconv.ParseInt(*maxDiskPerSession, 10, 64); err == nil {
			config.MaxDiskPerSession = bytes
		}
	}
	if *totalDiskLimit != "" {
		if bytes, err := strconv.ParseInt(*totalDiskLimit, 10, 64); err == nil {
			config.TotalDiskLimit = bytes
		}
	}
	if *transportType != "" {
		config.TransportType = *transportType
	}
	if *httpAddr != "" {
		config.HTTPAddr = *httpAddr
	}
	if *httpPort > 0 {
		config.HTTPPort = *httpPort
	}
	if *logHTTPBodies {
		config.LogHTTPBodies = true
	}
	if *maxBodyLogSize != "" {
		if bytes, err := strconv.ParseInt(*maxBodyLogSize, 10, 64); err == nil {
			config.MaxBodyLogSize = bytes
		}
	}
	if *logLevel != "" {
		config.LogLevel = *logLevel
	}
	if *sandboxEnabled {
		config.SandboxEnabled = true
	}

	// Apply OpenTelemetry flag overrides to the base config
	if *otelEnabled {
		config.EnableOTEL = true
	}
	if *otelEndpoint != "" {
		config.OTELEndpoint = *otelEndpoint
		config.EnableOTEL = true
	}
	if *serviceName != "" {
		config.ServiceName = *serviceName
	}
	if *serviceVersion != "" {
		config.ServiceVersion = *serviceVersion
	}
	if *environment != "" {
		config.Environment = *environment
	}
	if *traceSampleRate != 1.0 {
		config.TraceSampleRate = *traceSampleRate
	}

	// Parse OpenTelemetry headers if provided
	if *otelHeaders != "" {
		config.OTELHeaders = make(map[string]string)
		pairs := strings.Split(*otelHeaders, ",")
		for _, pair := range pairs {
			if kv := strings.SplitN(strings.TrimSpace(pair), "=", 2); len(kv) == 2 {
				config.OTELHeaders[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
			}
		}
	}

	// Setup structured logging
	setupLogging(config.LogLevel)

	log.Info().
		Str("version", getVersion()).
		Str("transport", config.TransportType).
		Str("workspace_dir", config.WorkspaceDir).
		Msg("Starting Container Kit MCP Server")

	// Create server using the factory
	server, err := factory.NewServer(context.Background(), config)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create server")
		os.Exit(1)
	}

	// Enable conversation mode if requested
	if *conversationMode {
		// Override OpenTelemetry flags with environment variables if not set
		if !*otelEnabled && os.Getenv("CONTAINER_KIT_OTEL_ENABLED") == "true" {
			*otelEnabled = true
		}
		if *otelEndpoint == "" {
			if val := os.Getenv("CONTAINER_KIT_OTEL_ENDPOINT"); val != "" {
				*otelEndpoint = val
			}
		}
		if *otelHeaders == "" {
			if val := os.Getenv("CONTAINER_KIT_OTEL_HEADERS"); val != "" {
				*otelHeaders = val
			}
		}
		if *serviceName == "container-kit-mcp" {
			if val := os.Getenv("CONTAINER_KIT_SERVICE_NAME"); val != "" {
				*serviceName = val
			}
		}
		if *serviceVersion == "" {
			if val := os.Getenv("CONTAINER_KIT_SERVICE_VERSION"); val != "" {
				*serviceVersion = val
			}
		}
		if *environment == "development" {
			if val := os.Getenv("CONTAINER_KIT_ENVIRONMENT"); val != "" {
				*environment = val
			}
		}
		if *traceSampleRate == 1.0 {
			if val := os.Getenv("CONTAINER_KIT_TRACE_SAMPLE_RATE"); val != "" {
				if parsed, err := strconv.ParseFloat(val, 64); err == nil {
					*traceSampleRate = parsed
				}
			}
		}

		// Parse OpenTelemetry headers if provided
		otelHeadersMap := make(map[string]string)
		if *otelHeaders != "" {
			pairs := strings.Split(*otelHeaders, ",")
			for _, pair := range pairs {
				if kv := strings.SplitN(strings.TrimSpace(pair), "=", 2); len(kv) == 2 {
					otelHeadersMap[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
				}
			}
		}

		// Set service version from build version if not provided
		svcVersion := *serviceVersion
		if svcVersion == "" {
			svcVersion = Version
		}

		conversationConfig := mcp.ConversationConfig{
			EnableTelemetry:   *telemetryEnabled,
			TelemetryPort:     *telemetryPort,
			PreferencesDBPath: "", // Will use default workspace path

			// OpenTelemetry configuration
			EnableOTEL:      *otelEnabled,
			OTELEndpoint:    *otelEndpoint,
			OTELHeaders:     otelHeadersMap,
			ServiceName:     *serviceName,
			ServiceVersion:  svcVersion,
			Environment:     *environment,
			TraceSampleRate: *traceSampleRate,
		}

		if err := server.EnableConversationMode(conversationConfig); err != nil {
			log.Error().Err(err).Msg("Failed to enable conversation mode")
			os.Exit(1)
		}

		log.Info().Msg("Conversation mode enabled - chat tool available")

		if *telemetryEnabled {
			log.Info().
				Int("port", *telemetryPort).
				Bool("otel_enabled", *otelEnabled).
				Str("otel_endpoint", *otelEndpoint).
				Msg("Prometheus metrics and OpenTelemetry enabled")
		}
	}

	// Handle demo mode
	if *demo != "" {
		log.Warn().Msg("Demo mode temporarily disabled due to API restructuring")
		return
	}

	// Create context for server operation
	ctx := context.Background()

	// Start server in a goroutine so we can handle shutdown
	serverErr := make(chan error, 1)
	go func() {
		if err := server.Start(ctx); err != nil {
			serverErr <- err
		}
	}()

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal or server error
	select {
	case sig := <-sigChan:
		log.Info().Str("signal", sig.String()).Msg("Received shutdown signal")

		// Gracefully shutdown the server
		if err := server.Stop(); err != nil {
			log.Error().Err(err).Msg("Error during server shutdown")
		}

		// Wait a moment for final logs to be written
		time.Sleep(100 * time.Millisecond)

	case err := <-serverErr:
		log.Error().Err(err).Msg("Server failed")
		os.Exit(1)

	case <-ctx.Done():
		log.Info().Msg("Context cancelled, shutting down")
		if err := server.Stop(); err != nil {
			log.Error().Err(err).Msg("Error during server shutdown")
		}
	}
}

// loadConfig loads configuration from environment variables and config file
func loadConfig(configFile string, telemetryEnabled *bool, telemetryPort *int) (mcp.ServerConfig, error) {
	// Start with defaults
	config := mcp.DefaultServerConfig()

	// Load .env file if it exists
	if configFile != "" {
		if err := godotenv.Load(configFile); err != nil {
			return config, fmt.Errorf("failed to load config file %s: %w", configFile, err)
		}
	} else {
		// Try to load default .env file
		if _, err := os.Stat(".env"); err == nil {
			godotenv.Load(".env")
		}
	}

	// Override with environment variables
	if val := os.Getenv("CONTAINER_KIT_WORKSPACE_DIR"); val != "" {
		config.WorkspaceDir = val
	}
	if val := os.Getenv("CONTAINER_KIT_STORE_PATH"); val != "" {
		config.StorePath = val
	}
	if val := os.Getenv("CONTAINER_KIT_MAX_SESSIONS"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			config.MaxSessions = parsed
		}
	}
	if val := os.Getenv("CONTAINER_KIT_SESSION_TTL"); val != "" {
		if parsed, err := time.ParseDuration(val); err == nil {
			config.SessionTTL = parsed
		}
	}
	if val := os.Getenv("CONTAINER_KIT_MAX_DISK_PER_SESSION"); val != "" {
		if parsed, err := strconv.ParseInt(val, 10, 64); err == nil {
			config.MaxDiskPerSession = parsed
		}
	}
	if val := os.Getenv("CONTAINER_KIT_TOTAL_DISK_LIMIT"); val != "" {
		if parsed, err := strconv.ParseInt(val, 10, 64); err == nil {
			config.TotalDiskLimit = parsed
		}
	}
	if val := os.Getenv("CONTAINER_KIT_TRANSPORT"); val != "" {
		config.TransportType = val
	}
	if val := os.Getenv("CONTAINER_KIT_HTTP_ADDR"); val != "" {
		config.HTTPAddr = val
	}
	if val := os.Getenv("CONTAINER_KIT_HTTP_PORT"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			config.HTTPPort = parsed
		}
	}
	if val := os.Getenv("CONTAINER_KIT_LOG_LEVEL"); val != "" {
		config.LogLevel = val
	}
	if val := os.Getenv("CONTAINER_KIT_LOG_HTTP_BODIES"); val != "" {
		config.LogHTTPBodies = val == "true" || val == "1"
	}
	if val := os.Getenv("CONTAINER_KIT_MAX_BODY_LOG_SIZE"); val != "" {
		if parsed, err := strconv.ParseInt(val, 10, 64); err == nil {
			config.MaxBodyLogSize = parsed
		}
	}
	if val := os.Getenv("CONTAINER_KIT_SANDBOX_ENABLED"); val != "" {
		config.SandboxEnabled = val == "true" || val == "1"
	}
	if val := os.Getenv("CONTAINER_KIT_CLEANUP_INTERVAL"); val != "" {
		if parsed, err := time.ParseDuration(val); err == nil {
			config.CleanupInterval = parsed
		}
	}

	// Telemetry configuration via environment variables
	if val := os.Getenv("CONTAINER_KIT_TELEMETRY_ENABLED"); val != "" {
		*telemetryEnabled = val == "true" || val == "1"
	}
	if val := os.Getenv("CONTAINER_KIT_TELEMETRY_PORT"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			*telemetryPort = parsed
		}
	}

	// OpenTelemetry configuration via environment variables
	if val := os.Getenv("CONTAINER_KIT_OTEL_ENABLED"); val != "" {
		config.EnableOTEL = val == "true" || val == "1"
	}
	if val := os.Getenv("CONTAINER_KIT_OTEL_ENDPOINT"); val != "" {
		config.OTELEndpoint = val
		config.EnableOTEL = true
	}
	if val := os.Getenv("CONTAINER_KIT_OTEL_HEADERS"); val != "" {
		// Parse headers format: "key1=value1,key2=value2"
		config.OTELHeaders = make(map[string]string)
		pairs := strings.Split(val, ",")
		for _, pair := range pairs {
			if kv := strings.SplitN(strings.TrimSpace(pair), "=", 2); len(kv) == 2 {
				config.OTELHeaders[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
			}
		}
	}
	if val := os.Getenv("CONTAINER_KIT_SERVICE_NAME"); val != "" {
		config.ServiceName = val
	}
	if val := os.Getenv("CONTAINER_KIT_SERVICE_VERSION"); val != "" {
		config.ServiceVersion = val
	}
	if val := os.Getenv("CONTAINER_KIT_ENVIRONMENT"); val != "" {
		config.Environment = val
	}
	if val := os.Getenv("CONTAINER_KIT_TRACE_SAMPLE_RATE"); val != "" {
		if parsed, err := strconv.ParseFloat(val, 64); err == nil {
			config.TraceSampleRate = parsed
		}
	}

	// Ensure directories exist
	if err := os.MkdirAll(config.WorkspaceDir, 0755); err != nil {
		return config, fmt.Errorf("failed to create workspace directory: %w", err)
	}

	if config.StorePath != "" {
		storeDir := filepath.Dir(config.StorePath)
		if err := os.MkdirAll(storeDir, 0755); err != nil {
			return config, fmt.Errorf("failed to create store directory: %w", err)
		}
	}

	return config, nil
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

// runDemo runs the specified demo mode
func runDemo(ctx context.Context, server mcp.Server, demoMode string) error {
	log.Warn().Str("mode", demoMode).Msg("Demo mode temporarily disabled due to API restructuring")
	return nil
}

// runAllDemos runs all demonstration scenarios
func runAllDemos(ctx context.Context, server mcp.Server) error {
	log.Warn().Msg("All demos temporarily disabled due to API restructuring")
	return nil
}

// runBasicWorkflowDemo demonstrates standard containerization workflow
func runBasicWorkflowDemo(ctx context.Context, server mcp.Server) error {
	log.Warn().Msg("Basic workflow demo temporarily disabled due to API restructuring")
	return nil
}

// runErrorHandlingDemo demonstrates error handling and recovery
func runErrorHandlingDemo(ctx context.Context, server mcp.Server) error {
	log.Warn().Msg("Error handling demo temporarily disabled due to API restructuring")
	return nil
}

// runSessionManagementDemo demonstrates session lifecycle
func runSessionManagementDemo(ctx context.Context, server mcp.Server) error {
	log.Warn().Msg("Session management demo temporarily disabled due to API restructuring")
	return nil
}

// runPerformanceDemo demonstrates performance monitoring
func runPerformanceDemo(ctx context.Context, server mcp.Server) error {
	log.Warn().Msg("Performance demo temporarily disabled due to API restructuring")
	return nil
}

// exportToolSchemas exports tool schemas to a file
func exportToolSchemas(outputPath string) error {
	// Define all available tools with their descriptions
	availableTools := []map[string]string{
		// Core Tools
		{"name": "chat", "category": "Core", "description": "Conversational interface for guided containerization workflow"},
		{"name": "server_status", "category": "Core", "description": "[Advanced] Diagnostic tool for debugging server issues"},
		{"name": "list_sessions", "category": "Core", "description": "List all active containerization sessions with their metadata and status"},
		{"name": "delete_session", "category": "Core", "description": "Delete a containerization session and clean up its resources"},

		// Repository Analysis
		{"name": "analyze_repository_atomic", "category": "Analysis", "description": "Analyze a repository to determine language, framework, and containerization requirements"},

		// Dockerfile Operations
		{"name": "generate_dockerfile_atomic", "category": "Dockerfile", "description": "Generate a Dockerfile using AI or templates based on repository analysis"},
		{"name": "validate_dockerfile_atomic", "category": "Dockerfile", "description": "Validate a Dockerfile for syntax errors and best practices"},

		// Container Image Operations
		{"name": "build_image_atomic", "category": "Image", "description": "Build a Docker image from a Dockerfile with automatic error fixing"},
		{"name": "pull_image_atomic", "category": "Image", "description": "Pull a Docker image from a registry"},
		{"name": "push_image_atomic", "category": "Image", "description": "Push a Docker image to a registry with authentication"},
		{"name": "tag_image_atomic", "category": "Image", "description": "Tag a Docker image with a new name or version"},

		// Security Scanning
		{"name": "scan_image_security_atomic", "category": "Security", "description": "Scan a Docker image for security vulnerabilities using Trivy"},
		{"name": "scan_secrets_atomic", "category": "Security", "description": "Scan code and configuration files for exposed secrets"},

		// Kubernetes Operations
		{"name": "generate_manifests_atomic", "category": "Kubernetes", "description": "Generate Kubernetes manifests (Deployment, Service, etc.) for an application"},
		{"name": "deploy_kubernetes_atomic", "category": "Kubernetes", "description": "Deploy an application to Kubernetes with automatic error fixing"},

		// Health Checks
		{"name": "check_health_atomic", "category": "Health", "description": "Check the health and readiness of deployed applications"},
	}

	// Group tools by category
	toolsByCategory := make(map[string][]map[string]string)
	for _, tool := range availableTools {
		category := tool["category"]
		toolsByCategory[category] = append(toolsByCategory[category], tool)
	}

	// Use GoMCP automatic schema generation instead of manual schema export
	// GoMCP automatically generates schemas for all registered tools and resources
	schemas := map[string]interface{}{
		"schema_version": "1.0.0",
		"generated_at":   time.Now().Format(time.RFC3339),
		"generator":      "gomcp-automatic",
		"description":    "Container Kit MCP Server - AI-powered containerization toolkit",
		"note":           "GoMCP provides automatic JSON schema generation via reflection",
		"source":         "github.com/localrivet/gomcp",
		"access_methods": []string{
			"Tools/Resources are automatically introspected by GoMCP",
			"Schemas available via MCP protocol introspection",
			"No manual schema maintenance required",
		},
		"tools": map[string]interface{}{
			"total_count": len(availableTools),
			"by_category": toolsByCategory,
			"categories":  []string{"Core", "Analysis", "Dockerfile", "Image", "Security", "Kubernetes", "Health"},
			"note":        "All tools registered with GoMCP automatically have schemas generated from their argument and result struct types",
			"automatic_features": []string{
				"JSON Schema generation from Go struct types",
				"Automatic validation based on struct tags",
				"Type-safe parameter handling",
				"Documentation from field tags",
				"AI-powered error fixing for build and deploy operations",
			},
		},
		"resources": map[string]interface{}{
			"note": "All resources registered with GoMCP automatically have schemas generated",
			"available_resources": []string{
				"logs/{level} - Server logs filtered by level (debug, info, warn, error)",
				"logs - All server logs with default filtering",
				"telemetry/metrics - Prometheus metrics endpoint",
				"telemetry/metrics/{name} - Specific metrics by name",
				"sessions - Active containerization sessions",
				"workflow/status - Current workflow execution status",
			},
		},
		"capabilities": map[string]interface{}{
			"workflow_modes": []string{
				"Guided conversation mode via 'chat' tool",
				"Direct atomic tool execution",
				"Parallel stage execution in workflows",
			},
			"ai_features": []string{
				"Repository analysis with language/framework detection",
				"Intelligent Dockerfile generation",
				"Automatic build error fixing",
				"Kubernetes deployment error resolution",
			},
			"security": []string{
				"Container image vulnerability scanning (Trivy)",
				"Secret detection in code and configs",
				"Registry authentication support",
			},
		},
		"migration_complete": true,
		"gomcp_version":      "v1.6.3",
	}

	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Write to file
	data, err := json.MarshalIndent(schemas, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}
