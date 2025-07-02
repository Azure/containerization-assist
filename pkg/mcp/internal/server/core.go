package server

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/docker"
	"github.com/Azure/container-kit/pkg/k8s"
	"github.com/Azure/container-kit/pkg/kind"
	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/core/orchestration"
	"github.com/Azure/container-kit/pkg/mcp/core/session"
	"github.com/Azure/container-kit/pkg/mcp/core/transport"
	"github.com/Azure/container-kit/pkg/mcp/errors"
	"github.com/Azure/container-kit/pkg/mcp/internal/analyze"
	"github.com/Azure/container-kit/pkg/mcp/internal/build"
	"github.com/Azure/container-kit/pkg/mcp/internal/common/utils"
	"github.com/Azure/container-kit/pkg/mcp/internal/deploy"
	"github.com/Azure/container-kit/pkg/mcp/internal/observability"
	"github.com/Azure/container-kit/pkg/mcp/internal/pipeline"
	"github.com/Azure/container-kit/pkg/mcp/internal/scan"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/types"
	"github.com/Azure/container-kit/pkg/runner"
	"github.com/localrivet/gomcp/server"
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
	gomcpManager GomcpManagerInterface

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

// GomcpManagerInterface defines the interface for gomcp manager
type GomcpManagerInterface interface {
	Initialize() error
	SetToolOrchestrator(orchestrator interface{})
	RegisterTools(server *Server) error
	StartServer() error
}

// simplifiedGomcpManager provides simple tool registration without over-engineering
type simplifiedGomcpManager struct {
	server        server.Server
	isInitialized bool
	logger        zerolog.Logger
	startTime     time.Time
}

// createRealGomcpManager creates a simplified gomcp manager
func createRealGomcpManager(transport interface{}, slogLevel slog.Level, serviceName string, logger zerolog.Logger) GomcpManagerInterface {
	return &simplifiedGomcpManager{
		logger:    logger.With().Str("component", "simplified_gomcp_manager").Logger(),
		startTime: time.Now(),
	}
}

// Initialize creates the simplified gomcp server
func (s *simplifiedGomcpManager) Initialize() error {
	s.logger.Info().Msg("Initializing simplified gomcp server")

	// Create slog logger for gomcp
	slogHandler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	slogger := *slog.New(slogHandler)

	// Create the gomcp server directly
	s.server = server.NewServer("Container Kit MCP Server",
		server.WithLogger(&slogger),
		server.WithProtocolVersion("1.0.0"),
	).AsStdio()

	if s.server == nil {
		return fmt.Errorf("failed to create gomcp stdio server")
	}

	s.isInitialized = true
	s.logger.Info().Msg("Simplified gomcp server initialized successfully")
	return nil
}

// SetToolOrchestrator is a no-op in simplified manager
func (s *simplifiedGomcpManager) SetToolOrchestrator(orchestrator interface{}) {
	// Simplified approach - no orchestrator dependency
	s.logger.Debug().Msg("SetToolOrchestrator called on simplified manager (no-op)")
}

// RegisterTools registers essential containerization tools
func (s *simplifiedGomcpManager) RegisterTools(srv *Server) error {
	if !s.isInitialized {
		return fmt.Errorf("gomcp manager not initialized")
	}

	s.logger.Info().Msg("Registering essential containerization tools")

	// Create dependencies needed for tools
	cmdRunner := &runner.DefaultCommandRunner{}
	mcpClients := mcptypes.NewMCPClients(
		docker.NewDockerCmdRunner(cmdRunner),
		kind.NewKindCmdRunner(cmdRunner),
		k8s.NewKubeCmdRunner(cmdRunner),
	)

	pipelineOps := pipeline.NewOperations(
		srv.sessionManager,
		mcpClients,
		srv.logger,
	)

	// Create tools
	analyzeRepoTool := analyze.NewAtomicAnalyzeRepositoryTool(pipelineOps, srv.sessionManager, srv.logger)
	buildImageTool := build.NewAtomicBuildImageTool(pipelineOps, srv.sessionManager, srv.logger)
	pushImageTool := build.NewAtomicPushImageTool(pipelineOps, srv.sessionManager, srv.logger)
	generateManifestsTool := deploy.NewAtomicGenerateManifestsTool(pipelineOps, srv.sessionManager, srv.logger)
	scanImageTool := scan.NewAtomicScanImageSecurityTool(pipelineOps, srv.sessionManager, srv.logger)

	// Register analyze_repository tool
	s.server.Tool("analyze_repository", "Analyze repository structure and generate Dockerfile recommendations. Creates a new session for tracking analysis workflow",
		func(ctx *server.Context, args *analyze.AtomicAnalyzeRepositoryArgs) (*analyze.AtomicAnalysisResult, error) {
			return analyzeRepoTool.ExecuteWithContext(ctx, args)
		})

	// Register build_image tool
	s.server.Tool("build_image", "Build Docker images from Dockerfile. Uses session context for build configuration",
		func(ctx *server.Context, args *build.AtomicBuildImageArgs) (*build.AtomicBuildImageResult, error) {
			return buildImageTool.ExecuteWithContext(ctx, args)
		})

	// Register push_image tool
	s.server.Tool("push_image", "Push Docker images to container registries",
		func(ctx *server.Context, args *build.AtomicPushImageArgs) (*build.AtomicPushImageResult, error) {
			return pushImageTool.ExecuteWithFixes(context.Background(), *args)
		})

	// Register generate_manifests tool
	s.server.Tool("generate_manifests", "Generate Kubernetes manifests for containerized applications. Uses session context for manifest generation",
		func(ctx *server.Context, args *deploy.GenerateManifestsArgs) (*deploy.GenerateManifestsResult, error) {
			result, err := generateManifestsTool.Execute(context.Background(), *args)
			if err != nil {
				return nil, err
			}
			if typed, ok := result.(*deploy.GenerateManifestsResult); ok {
				return typed, nil
			}
			return nil, fmt.Errorf("unexpected result type: %T", result)
		})

	// Register generate_dockerfile tool
	generateDockerfileTool := analyze.NewAtomicGenerateDockerfileTool(srv.sessionManager, srv.logger)
	s.server.Tool("generate_dockerfile", "Generate optimized Dockerfile based on repository analysis. Uses session context for Dockerfile generation",
		func(ctx *server.Context, args *analyze.GenerateDockerfileArgs) (*analyze.GenerateDockerfileResult, error) {
			return generateDockerfileTool.ExecuteTyped(context.Background(), *args)
		})

	// Register scan_image tool
	s.server.Tool("scan_image", "Scan Docker images for security vulnerabilities. Uses session context for vulnerability scanning",
		func(ctx *server.Context, args *scan.AtomicScanImageSecurityArgs) (*scan.AtomicScanImageSecurityResult, error) {
			return scanImageTool.ExecuteWithContext(ctx, args)
		})

	// Register list_sessions tool
	s.server.Tool("list_sessions", "List all active and recent sessions with their status",
		func(ctx *server.Context, args *struct {
			Limit *int `json:"limit,omitempty"`
		}) (*struct {
			Sessions []map[string]interface{} `json:"sessions"`
			Total    int                      `json:"total"`
		}, error) {
			sessions := srv.sessionManager.ListSessionSummaries()
			limit := 50 // default limit
			if args.Limit != nil && *args.Limit > 0 {
				limit = *args.Limit
			}

			sessionData := make([]map[string]interface{}, 0)
			for i, session := range sessions {
				if i >= limit {
					break
				}
				sessionInfo := map[string]interface{}{
					"session_id":    session.SessionID,
					"created_at":    session.CreatedAt,
					"last_accessed": session.LastAccessed,
					"status":        session.Status,
					"disk_usage":    session.DiskUsage,
					"active_jobs":   session.ActiveJobs,
				}
				if session.RepoURL != "" {
					sessionInfo["repo_url"] = session.RepoURL
				}
				sessionData = append(sessionData, sessionInfo)
			}

			return &struct {
				Sessions []map[string]interface{} `json:"sessions"`
				Total    int                      `json:"total"`
			}{
				Sessions: sessionData,
				Total:    len(sessions),
			}, nil
		})

	// Register diagnostic tools
	s.server.Tool("ping", "Simple ping tool to test MCP connectivity",
		func(ctx *server.Context, args struct {
			Message string `json:"message,omitempty"`
		}) (interface{}, error) {
			response := "pong"
			if args.Message != "" {
				response = "pong: " + args.Message
			}
			return map[string]interface{}{
				"response":  response,
				"timestamp": time.Now().Format(time.RFC3339),
			}, nil
		})

	s.server.Tool("server_status", "Get basic server status information",
		func(ctx *server.Context, args *struct {
			Details bool `json:"details,omitempty"`
		}) (*struct {
			Status  string `json:"status"`
			Version string `json:"version"`
			Uptime  string `json:"uptime"`
		}, error) {
			return &struct {
				Status  string `json:"status"`
				Version string `json:"version"`
				Uptime  string `json:"uptime"`
			}{
				Status:  "running",
				Version: "dev",
				Uptime:  time.Since(s.startTime).String(),
			}, nil
		})

	s.logger.Info().Msg("Essential containerization tools registered successfully")
	return nil
}

// StartServer starts the simplified gomcp server
func (s *simplifiedGomcpManager) StartServer() error {
	if !s.isInitialized {
		return fmt.Errorf("gomcp manager not initialized")
	}
	if s.server == nil {
		return fmt.Errorf("gomcp server is nil")
	}

	s.logger.Info().Msg("Starting simplified gomcp server")

	// Cast to the gomcp server interface and run
	if mcpServer, ok := s.server.(interface{ Run() error }); ok {
		return mcpServer.Run()
	}

	return fmt.Errorf("server does not implement Run() method")
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
		BaseDir: config.WorkspaceDir,
		Logger:  logger.With().Str("component", "workspace_manager").Logger(),
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

	// Convert zerolog to slog level
	var slogLevel slog.Level
	switch logLevel {
	case zerolog.DebugLevel:
		slogLevel = slog.LevelDebug
	case zerolog.InfoLevel:
		slogLevel = slog.LevelInfo
	case zerolog.WarnLevel:
		slogLevel = slog.LevelWarn
	case zerolog.ErrorLevel:
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}

	// Create a simplified tool manager
	gomcpManager := createRealGomcpManager(mcpTransport, slogLevel, config.ServiceName, logger)

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
		gomcpManager:     gomcpManager,
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

	// Set tool orchestrator reference for the gomcp manager
	s.gomcpManager.SetToolOrchestrator(s.toolOrchestrator)

	// Register tools with gomcp
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

// IsConversationModeEnabled returns whether conversation mode is enabled
func (s *Server) IsConversationModeEnabled() bool {
	if s.conversationComponents != nil {
		return s.conversationComponents.isEnabled
	}
	return false
}
