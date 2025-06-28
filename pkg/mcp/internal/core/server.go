package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/errors"
	"github.com/Azure/container-kit/pkg/mcp/internal/observability"
	"github.com/Azure/container-kit/pkg/mcp/internal/orchestration"
	"github.com/Azure/container-kit/pkg/mcp/internal/session"
	"github.com/Azure/container-kit/pkg/mcp/internal/transport"
	"github.com/Azure/container-kit/pkg/mcp/internal/utils"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/types"
	"github.com/rs/zerolog"
)

// sessionManagerAdapterImpl adapts the core session manager to orchestration.SessionManager interface
type sessionManagerAdapterImpl struct {
	sessionManager *session.SessionManager
}

func (s *sessionManagerAdapterImpl) GetSession(sessionID string) (interface{}, error) {
	return s.sessionManager.GetSession(sessionID)
}

func (s *sessionManagerAdapterImpl) UpdateSession(session interface{}) error {
	// Convert interface{} back to the concrete session type and update
	switch sess := session.(type) {
	case *mcptypes.SessionState:
		if sess.SessionID == "" {
			return errors.Validation("core/server", "session ID is required for updates")
		}
		return s.sessionManager.UpdateSession(sess.SessionID, func(existing interface{}) {
			if existingState, ok := existing.(*mcptypes.SessionState); ok {
				*existingState = *sess
			}
		})
	case mcptypes.SessionState:
		if sess.SessionID == "" {
			return errors.Validation("core/server", "session ID is required for updates")
		}
		return s.sessionManager.UpdateSession(sess.SessionID, func(existing interface{}) {
			if existingState, ok := existing.(*mcptypes.SessionState); ok {
				*existingState = sess
			}
		})
	default:
		// If we can't convert, just succeed silently to maintain compatibility
		return nil
	}
}

// Server represents the MCP server
type Server struct {
	config           ServerConfig
	sessionManager   *session.SessionManager
	workspaceManager *utils.WorkspaceManager
	circuitBreakers  *orchestration.CircuitBreakerRegistry
	jobManager       *orchestration.JobManager
	transport        InternalTransport
	logger           zerolog.Logger
	startTime        time.Time

	// Canonical orchestration system
	toolOrchestrator *orchestration.MCPToolOrchestrator
	toolRegistry     *orchestration.MCPToolRegistry

	// Conversation mode components
	conversationComponents *ConversationComponents

	// Gomcp manager for lean tool registration
	gomcpManager *GomcpManager

	// OpenTelemetry components
	otelProvider   *observability.OTELProvider
	otelMiddleware *observability.MCPServerInstrumentation

	// Shutdown coordination
	shutdownMutex  sync.Mutex
	isShuttingDown bool
}

// NewServer creates a new MCP server
func NewServer(ctx context.Context, config ServerConfig) (*Server, error) {
	// Setup logger
	logLevel, err := zerolog.ParseLevel(config.LogLevel)
	if err != nil {
		logLevel = zerolog.InfoLevel
	}

	// Initialize log capture with 10k entry capacity
	utils.InitializeLogCapture(10000)
	logBuffer := utils.GetGlobalLogBuffer()

	// Create logger that writes to both stderr and the ring buffer
	logger := utils.CreateCaptureLogger(logBuffer, os.Stderr).
		Level(logLevel).
		With().
		Str("component", "mcp-server").
		Logger()

	// Create storage directory
	if config.StorePath != "" {
		if err := os.MkdirAll(filepath.Dir(config.StorePath), 0o755); err != nil {
			logger.Error().Err(err).Str("path", config.StorePath).Msg("Failed to create storage directory")
			return nil, errors.Wrapf(err, "core/server", "failed to create storage directory %s", config.StorePath)
		}
	}

	// Initialize session manager
	sessionManager, err := session.NewSessionManager(session.SessionManagerConfig{
		WorkspaceDir:      config.WorkspaceDir,
		MaxSessions:       config.MaxSessions,
		SessionTTL:        config.SessionTTL,
		MaxDiskPerSession: config.MaxDiskPerSession,
		TotalDiskLimit:    config.TotalDiskLimit,
		StorePath:         config.StorePath,
		Logger:            logger.With().Str("component", "session_manager").Logger(),
	})
	if err != nil {
		logger.Error().Err(err).Msg("Failed to initialize session manager")
		return nil, errors.Wrap(err, "core/server", "failed to initialize session manager")
	}

	// Initialize workspace manager
	workspaceManager, err := utils.NewWorkspaceManager(ctx, utils.WorkspaceConfig{
		BaseDir:           config.WorkspaceDir,
		MaxSizePerSession: config.MaxDiskPerSession,
		TotalMaxSize:      config.TotalDiskLimit,
		Cleanup:           true,
		SandboxEnabled:    config.SandboxEnabled,
		Logger:            logger.With().Str("component", "workspace_manager").Logger(),
	})
	if err != nil {
		logger.Error().Err(err).Msg("Failed to initialize workspace manager")
		return nil, errors.Wrap(err, "core/server", "failed to initialize workspace manager")
	}

	// Initialize circuit breakers
	circuitBreakers := orchestration.CreateDefaultCircuitBreakers(logger.With().Str("component", "circuit_breaker").Logger())

	// Initialize job manager
	jobManager := orchestration.NewJobManager(orchestration.JobManagerConfig{
		MaxWorkers: config.MaxWorkers,
		JobTTL:     config.JobTTL,
		Logger:     logger.With().Str("component", "job_manager").Logger(),
	})

	// Initialize transport
	var mcpTransport InternalTransport
	switch config.TransportType {
	case "http":
		httpConfig := transport.HTTPTransportConfig{
			Port:           config.HTTPPort,
			CORSOrigins:    config.CORSOrigins,
			APIKey:         config.APIKey,
			RateLimit:      config.RateLimit,
			Logger:         logger.With().Str("transport", "http").Logger(),
			LogBodies:      config.LogHTTPBodies,
			MaxBodyLogSize: config.MaxBodyLogSize,
			LogLevel:       config.LogLevel,
		}
		httpTransport := transport.NewHTTPTransport(httpConfig)
		mcpTransport = NewTransportAdapter(httpTransport)
	case "stdio":
		fallthrough
	default:
		// Use factory for consistent stdio transport creation
		mcpTransport = transport.NewDefaultStdioTransport(logger)
	}

	// Create gomcp manager with builder pattern
	gomcpConfig := GomcpConfig{
		Name:            "Container-Kit MCP",
		ProtocolVersion: "2024-11-05",
		LogLevel:        convertZerologToSlog(logger.GetLevel()),
	}
	gomcpManager := NewGomcpManager(gomcpConfig).
		WithTransport(mcpTransport).
		WithLogger(*slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: convertZerologToSlog(logger.GetLevel()),
		})))

	// Set GomcpManager on transport for proper lifecycle management
	// Use type assertion since InternalTransport interface doesn't have Name() method
	if setter, ok := mcpTransport.(interface{ SetGomcpManager(interface{}) }); ok {
		setter.SetGomcpManager(gomcpManager)
	}

	// Initialize OpenTelemetry if enabled
	var otelProvider *observability.OTELProvider
	var otelMiddleware *observability.MCPServerInstrumentation

	if config.EnableOTEL {
		logger.Info().Msg("Initializing OpenTelemetry middleware")

		// Create OTEL configuration
		otelConfig := &observability.OTELConfig{
			ServiceName:     config.ServiceName,
			ServiceVersion:  config.ServiceVersion,
			Environment:     config.Environment,
			EnableOTLP:      true,
			OTLPEndpoint:    config.OTELEndpoint,
			OTLPHeaders:     config.OTELHeaders,
			OTLPInsecure:    true, // Default for local development
			TraceSampleRate: config.TraceSampleRate,
			Logger:          logger.With().Str("component", "otel").Logger(),
		}

		// Validate OTEL configuration
		if err := otelConfig.Validate(); err != nil {
			logger.Error().Err(err).Msg("Failed to validate OpenTelemetry configuration")
			return nil, errors.Wrap(err, "core/server", "failed to validate OpenTelemetry configuration")
		}

		// Create and initialize OTEL provider
		otelProvider = observability.NewOTELProvider(otelConfig)
		ctx := context.Background()
		if err := otelProvider.Initialize(ctx); err != nil {
			logger.Error().Err(err).Msg("Failed to initialize OpenTelemetry provider")
			return nil, errors.Wrap(err, "core/server", "failed to initialize OpenTelemetry provider")
		}

		// Create server instrumentation
		otelMiddleware = observability.NewMCPServerInstrumentation(config.ServiceName, logger.With().Str("component", "otel_middleware").Logger())

		logger.Info().
			Str("service_name", config.ServiceName).
			Str("otlp_endpoint", config.OTELEndpoint).
			Float64("sample_rate", config.TraceSampleRate).
			Msg("OpenTelemetry middleware initialized successfully")
	} else {
		logger.Info().Msg("OpenTelemetry disabled")
	}

	// Initialize canonical tool orchestrator
	toolRegistry := orchestration.NewMCPToolRegistry(logger.With().Str("component", "tool_registry").Logger())

	// Create session manager adapter for orchestrator
	sessionManagerAdapter := &sessionManagerAdapterImpl{sessionManager: sessionManager}

	toolOrchestrator := orchestration.NewMCPToolOrchestrator(
		toolRegistry,
		sessionManagerAdapter,
		logger.With().Str("component", "tool_orchestrator").Logger(),
	)

	server := &Server{
		config:           config,
		sessionManager:   sessionManager,
		workspaceManager: workspaceManager,
		circuitBreakers:  circuitBreakers,
		jobManager:       jobManager,
		transport:        mcpTransport,
		logger:           logger,
		startTime:        time.Now(),
		toolOrchestrator: toolOrchestrator,
		toolRegistry:     toolRegistry,
		gomcpManager:     gomcpManager,
		otelProvider:     otelProvider,
		otelMiddleware:   otelMiddleware,
	}

	return server, nil
}

// convertZerologToSlog converts zerolog level to slog level
func convertZerologToSlog(level zerolog.Level) slog.Level {
	switch level {
	case zerolog.DebugLevel:
		return slog.LevelDebug
	case zerolog.InfoLevel:
		return slog.LevelInfo
	case zerolog.WarnLevel:
		return slog.LevelWarn
	case zerolog.ErrorLevel:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// IsConversationModeEnabled checks if conversation mode is enabled
func (s *Server) IsConversationModeEnabled() bool {
	return s.conversationComponents != nil
}

// GetTransport returns the server's transport
func (s *Server) GetTransport() InternalTransport {
	return s.transport
}

// GetSessionManager returns the server's session manager
func (s *Server) GetSessionManager() interface{} {
	return s.sessionManager
}

// GetWorkspaceManager returns the server's workspace manager
func (s *Server) GetWorkspaceManager() interface{} {
	return s.workspaceManager
}

// ExportToolSchemas exports tool schemas to a file
func (s *Server) ExportToolSchemas(outputPath string) error {
	// Get the tool registry from gomcp manager
	if s.gomcpManager == nil || !s.gomcpManager.isInitialized {
		return errors.Internal("core/server", "server not properly initialized")
	}

	s.logger.Info().
		Str("output_path", outputPath).
		Msg("Starting tool schema export")

	// Create proper schema export structure
	schemas := map[string]interface{}{
		"schema_version": "1.0.0",
		"generated_at":   time.Now(),
		"generator":      "container-kit-mcp",
		"description":    "Machine-readable schema for Container Kit MCP tools",
		"tools":          s.getAvailableToolSchemas(),
		"metadata": map[string]interface{}{
			"export_method": "server_direct",
			"has_gomcp":     s.gomcpManager != nil,
			"initialized":   s.gomcpManager != nil && s.gomcpManager.isInitialized,
		},
	}

	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return errors.Wrap(err, "core/server", "failed to create output directory")
	}

	// Write to file
	data, err := json.MarshalIndent(schemas, "", "  ")
	if err != nil {
		return errors.Wrap(err, "core/server", "failed to marshal JSON")
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return errors.Wrap(err, "core/server", "failed to write file")
	}

	s.logger.Info().
		Str("output_path", outputPath).
		Int64("file_size", int64(len(data))).
		Msg("Schema export completed successfully")

	return nil
}

// getAvailableToolSchemas attempts to retrieve tool schemas from available sources
func (s *Server) getAvailableToolSchemas() map[string]interface{} {
	tools := make(map[string]interface{})

	// Conversation handler doesn't provide tool schemas directly
	// Tools are registered in the orchestrator

	// Fallback: provide basic tool information from known atomic tools
	atomicTools := []string{
		"atomic_analyze_repository",
		"atomic_build_image",
		"atomic_generate_manifests",
		"atomic_deploy_kubernetes",
		"atomic_validate_dockerfile",
		"atomic_scan_secrets",
		"atomic_scan_image_security",
		"atomic_tag_image",
		"atomic_push_image",
		"atomic_pull_image",
		"atomic_check_health",
	}

	for _, toolName := range atomicTools {
		tools[toolName] = map[string]interface{}{
			"name":        toolName,
			"category":    "atomic",
			"description": fmt.Sprintf("Atomic tool for %s operations", toolName[7:]), // Remove "atomic_" prefix
			"available":   true,
			"schema_note": "Full schema available via proper tool registry access",
		}
	}

	return tools
}

// GetLogger returns the server's logger
func (s *Server) GetLogger() zerolog.Logger {
	return s.logger
}

// GetCircuitBreakers returns the server's circuit breakers
func (s *Server) GetCircuitBreakers() *orchestration.CircuitBreakerRegistry {
	return s.circuitBreakers
}

// GetJobManager returns the server's job manager
func (s *Server) GetJobManager() *orchestration.JobManager {
	return s.jobManager
}

// GetOTELProvider returns the server's OpenTelemetry provider
func (s *Server) GetOTELProvider() *observability.OTELProvider {
	return s.otelProvider
}

// GetOTELMiddleware returns the server's OpenTelemetry middleware
func (s *Server) GetOTELMiddleware() *observability.MCPServerInstrumentation {
	return s.otelMiddleware
}

// IsOTELEnabled returns whether OpenTelemetry is enabled
func (s *Server) IsOTELEnabled() bool {
	return s.otelProvider != nil && s.otelProvider.IsInitialized()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	s.shutdownMutex.Lock()
	defer s.shutdownMutex.Unlock()

	if s.isShuttingDown {
		return nil // Already shutting down
	}
	s.isShuttingDown = true

	s.logger.Info().Msg("Starting server shutdown")

	// Stop job manager
	if s.jobManager != nil {
		s.jobManager.Stop()
	}

	// Shutdown OpenTelemetry
	if s.otelProvider != nil {
		if err := s.otelProvider.Shutdown(ctx); err != nil {
			s.logger.Error().Err(err).Msg("Failed to shutdown OpenTelemetry provider")
		}
	}

	s.logger.Info().Msg("Server shutdown completed")
	return nil
}
