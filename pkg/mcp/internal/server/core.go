package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/errors"
	"github.com/Azure/container-kit/pkg/mcp/internal/observability"
	"github.com/Azure/container-kit/pkg/mcp/internal/orchestration"
	"github.com/Azure/container-kit/pkg/mcp/internal/session"
	"github.com/Azure/container-kit/pkg/mcp/internal/transport"
	"github.com/Azure/container-kit/pkg/mcp/internal/utils"
	"github.com/rs/zerolog"
)

// Server represents the consolidated MCP server
// Consolidated from pkg/mcp/internal/core/server.go + server_lifecycle.go
type Server struct {
	config           ServerConfig
	sessionManager   *session.SessionManager
	workspaceManager *utils.WorkspaceManager
	circuitBreakers  *orchestration.CircuitBreakerRegistry
	jobManager       *orchestration.JobManager
	transport        interface{} // stdio or http transport
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

// ConversationComponents represents conversation mode components
type ConversationComponents struct {
	// Add conversation-specific components here
	isEnabled bool
}

// GomcpManager represents the gomcp manager
type GomcpManager struct {
	// Add gomcp manager fields here
	server interface{}
}

// Initialize initializes the gomcp manager
func (g *GomcpManager) Initialize() error {
	// Implementation would go here
	return nil
}

// RegisterTools registers tools with gomcp
func (g *GomcpManager) RegisterTools(server *Server) error {
	// Implementation would go here
	return nil
}

// StartServer starts the gomcp server
func (g *GomcpManager) StartServer() error {
	// Implementation would go here
	return nil
}

// NewServer creates a new consolidated MCP server
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
			return nil, errors.Wrapf(err, "server/core", "failed to create storage directory %s", config.StorePath)
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
		return nil, errors.Wrap(err, "server/core", "failed to initialize session manager")
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
		return nil, errors.Wrap(err, "server/core", "failed to initialize workspace manager")
	}

	// Initialize circuit breakers
	circuitBreakers := orchestration.NewCircuitBreakerRegistry(logger.With().Str("component", "circuit_breakers").Logger())

	// Initialize job manager
	jobManager := orchestration.NewJobManager(orchestration.JobManagerConfig{
		MaxWorkers: config.MaxWorkers,
		JobTTL:     config.JobTTL,
		Logger:     logger.With().Str("component", "job_manager").Logger(),
	})

	// Initialize tool registry
	toolRegistry := orchestration.NewMCPToolRegistry(logger.With().Str("component", "tool_registry").Logger())

	// Initialize tool orchestrator
	toolOrchestrator := orchestration.NewMCPToolOrchestrator(toolRegistry, sessionManager, logger.With().Str("component", "tool_orchestrator").Logger())

	// Initialize OpenTelemetry if enabled
	var otelProvider *observability.OTELProvider
	var otelMiddleware *observability.MCPServerInstrumentation
	if config.EnableOTEL {
		otelProvider = observability.NewOTELProvider(&observability.OTELConfig{
			ServiceName:      config.ServiceName,
			ServiceVersion:   config.ServiceVersion,
			Environment:      config.Environment,
			EnableOTLP:       true,
			OTLPEndpoint:     config.OTELEndpoint,
			OTLPHeaders:      config.OTELHeaders,
			TraceSampleRate:  config.TraceSampleRate,
			CustomAttributes: make(map[string]string),
			Logger:           logger.With().Str("component", "otel").Logger(),
		})
		if otelProvider == nil {
			logger.Error().Msg("Failed to initialize OpenTelemetry provider")
			return nil, fmt.Errorf("failed to initialize OpenTelemetry provider")
		}

		otelMiddleware = observability.NewMCPServerInstrumentation(config.ServiceName, logger.With().Str("component", "otel_middleware").Logger())
	}

	// Create transport
	var mcpTransport interface{}
	switch config.TransportType {
	case "stdio":
		mcpTransport = transport.NewStdioTransport()
	case "http":
		mcpTransport = transport.NewHTTPTransport(transport.HTTPTransportConfig{
			Port:           config.HTTPPort,
			CORSOrigins:    config.CORSOrigins,
			APIKey:         config.APIKey,
			RateLimit:      config.RateLimit,
			Logger:         logger.With().Str("component", "http_transport").Logger(),
			LogBodies:      config.LogHTTPBodies,
			MaxBodyLogSize: config.MaxBodyLogSize,
			LogLevel:       config.LogLevel,
		})
	default:
		return nil, fmt.Errorf("unsupported transport type: %s", config.TransportType)
	}

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
		otelProvider:     otelProvider,
		otelMiddleware:   otelMiddleware,
		gomcpManager:     &GomcpManager{},
		conversationComponents: &ConversationComponents{
			isEnabled: false,
		},
	}

	logger.Info().
		Str("transport", config.TransportType).
		Str("workspace_dir", config.WorkspaceDir).
		Int("max_sessions", config.MaxSessions).
		Msg("MCP Server initialized successfully")

	return server, nil
}

// Start starts the MCP server (consolidated from server_lifecycle.go)
func (s *Server) Start(ctx context.Context) error {
	s.logger.Info().
		Str("transport", s.config.TransportType).
		Str("workspace_dir", s.config.WorkspaceDir).
		Int("max_sessions", s.config.MaxSessions).
		Msg("Starting Container Kit MCP Server")

	// Start session cleanup routine
	s.sessionManager.StartCleanupRoutine()

	// Initialize and configure gomcp server
	if s.gomcpManager == nil {
		return fmt.Errorf("gomcp manager is nil - server initialization failed")
	}
	if err := s.gomcpManager.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize gomcp manager: %w", err)
	}

	// Register all tools with gomcp
	if err := s.gomcpManager.RegisterTools(s); err != nil {
		return fmt.Errorf("failed to register tools with gomcp: %w", err)
	}

	// Set the server as the request handler for the transport
	if setter, ok := s.transport.(interface{ SetHandler(interface{}) }); ok {
		setter.SetHandler(s)
	}

	// Start transport serving
	transportDone := make(chan error, 1)
	go func() {
		// Start transport - use gomcp manager since transport doesn't have Serve method
		transportDone <- s.gomcpManager.StartServer()
	}()

	// Wait for context cancellation or transport error
	select {
	case <-ctx.Done():
		s.logger.Info().Msg("Server stopped by context cancellation")
		return ctx.Err()
	case err := <-transportDone:
		s.logger.Error().Err(err).Msg("Transport stopped with error")
		return err
	}
}

// Stop stops the MCP server
func (s *Server) Stop() error {
	s.logger.Info().Msg("Stopping MCP Server")

	// Stop session manager (this handles cleanup routine too)
	if err := s.sessionManager.Stop(); err != nil {
		s.logger.Error().Err(err).Msg("Failed to stop session manager")
		return err
	}

	// Additional cleanup would go here

	s.logger.Info().Msg("MCP Server stopped successfully")
	return nil
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	s.shutdownMutex.Lock()
	defer s.shutdownMutex.Unlock()

	if s.isShuttingDown {
		return nil // Already shutting down
	}
	s.isShuttingDown = true

	s.logger.Info().Msg("Gracefully shutting down MCP Server")

	// Shutdown OpenTelemetry
	if s.otelProvider != nil {
		if err := s.otelProvider.Shutdown(ctx); err != nil {
			s.logger.Error().Err(err).Msg("Failed to shutdown OpenTelemetry provider")
		}
	}

	// Stop components
	if err := s.Stop(); err != nil {
		s.logger.Error().Err(err).Msg("Error during server stop")
		return err
	}

	s.logger.Info().Msg("MCP Server shutdown complete")
	return nil
}

// EnableConversationMode enables conversation mode
func (s *Server) EnableConversationMode(config core.ConversationConfig) error {
	if s.conversationComponents != nil {
		s.conversationComponents.isEnabled = true
	}
	return nil
}
