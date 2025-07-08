package server

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/docker"
	"github.com/Azure/container-kit/pkg/k8s"
	"github.com/Azure/container-kit/pkg/kind"
	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/Azure/container-kit/pkg/mcp/core"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/errors"
	"github.com/Azure/container-kit/pkg/mcp/internal/pipeline"
	"github.com/Azure/container-kit/pkg/mcp/internal/runtime"
	"github.com/Azure/container-kit/pkg/mcp/services"
	"github.com/Azure/container-kit/pkg/mcp/session"
	"github.com/Azure/container-kit/pkg/mcp/tools/analyze"
	"github.com/Azure/container-kit/pkg/mcp/tools/build"
	"github.com/Azure/container-kit/pkg/mcp/tools/deploy"
	"github.com/Azure/container-kit/pkg/mcp/tools/scan"
	"github.com/Azure/container-kit/pkg/mcp/transport"
	"github.com/Azure/container-kit/pkg/mcp/workflow"
	"github.com/Azure/container-kit/pkg/runner"
	"github.com/localrivet/gomcp/server"
	"github.com/rs/zerolog"
)

// parseSlogLevel converts a string log level to slog.Level
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

// adaptSlogToZerolog creates a zerolog.Logger from an slog.Logger
func adaptSlogToZerolog(slogLogger *slog.Logger) zerolog.Logger {
	// Create a zerolog logger with appropriate level
	level := zerolog.InfoLevel
	return zerolog.New(os.Stderr).Level(level).With().Timestamp().Logger()
}

// adaptMCPContext creates a context.Context from a gomcp server.Context
func adaptMCPContext(mcpCtx *server.Context) context.Context {
	// For now, return a background context
	// In a real implementation, we might need to extract request metadata
	return context.Background()
}

// Server represents the consolidated MCP server
type Server struct {
	config         ServerConfig
	sessionManager *session.SessionManager
	// workspaceManager *runtime.WorkspaceManager // TODO: Type needs to be implemented
	// circuitBreakers  *execution.CircuitBreakerRegistry // TODO: Type needs to be implemented
	jobManager *workflow.JobManager
	transport  interface{} // stdio or http transport
	logger     *slog.Logger
	startTime  time.Time

	toolOrchestrator api.Orchestrator
	toolRegistry     *runtime.ToolRegistry

	conversationComponents *ConversationComponents

	gomcpManager api.GomcpManager

	shutdownMutex  sync.Mutex
	isShuttingDown bool
}

// ConversationComponents represents conversation mode components
type ConversationComponents struct {
	isEnabled bool
}

// simplifiedGomcpManager provides simple tool registration without over-engineering
type simplifiedGomcpManager struct {
	server        server.Server
	isInitialized bool
	logger        *slog.Logger
	startTime     time.Time
}

// createRealGomcpManager creates a simplified gomcp manager
func createRealGomcpManager(_ interface{}, _ slog.Level, _ string, logger *slog.Logger) api.GomcpManager {
	return &simplifiedGomcpManager{
		logger:    logger.With("component", "simplified_gomcp_manager"),
		startTime: time.Now(),
	}
}

// Start creates and starts the simplified gomcp server
func (s *simplifiedGomcpManager) Start(_ context.Context) error {
	s.logger.Info("Initializing simplified gomcp server")

	s.server = server.NewServer("Container Kit MCP Server",
		server.WithLogger(s.logger),
		server.WithProtocolVersion("1.0.0"),
	).AsStdio()

	if s.server == nil {
		return errors.NewError().Messagef("failed to create gomcp stdio server").Build()
	}

	s.isInitialized = true
	s.logger.Info("Simplified gomcp server initialized successfully")

	if s.server == nil {
		return errors.NewError().Messagef("gomcp server is nil").Build()
	}

	s.logger.Info("Starting simplified gomcp server")

	if mcpServer, ok := s.server.(interface{ Run() error }); ok {
		return mcpServer.Run()
	}

	return errors.NewError().Messagef("server does not implement Run() method").Build()
}

// Stop stops the gomcp server
func (s *simplifiedGomcpManager) Stop(_ context.Context) error {
	s.logger.Info("Stopping simplified gomcp server")
	s.isInitialized = false
	return nil
}

// RegisterTool registers a tool with gomcp
func (s *simplifiedGomcpManager) RegisterTool(name, _ string, _ interface{}) error {
	if !s.isInitialized {
		return errors.NewError().Messagef("gomcp manager not initialized").Build()
	}
	s.logger.Debug("Registering tool with simplified manager", "tool", name)
	return nil
}

// GetServer returns the underlying gomcp server
func (s *simplifiedGomcpManager) GetServer() *server.Server {
	return nil
}

// IsRunning checks if the server is running
func (s *simplifiedGomcpManager) IsRunning() bool {
	return s.isInitialized
}

// RegisterTools registers essential containerization tools
func (s *simplifiedGomcpManager) RegisterTools(srv *Server) error {
	if !s.isInitialized {
		return errors.NewError().Messagef("gomcp manager not initialized").Build()
	}

	s.logger.Info("Registering essential containerization tools")

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

	var unifiedSessionMgr session.UnifiedSessionManager = srv.sessionManager

	// Create service container adapter - temporarily disabled
	// serviceContainer := containerization.NewSessionManagerAdapter(unifiedSessionMgr, srv.logger)
	var serviceContainer services.ServiceContainer // Temporary nil value for compilation

	analyzeRepoTool := analyze.NewAtomicAnalyzeRepositoryTool(pipelineOps, unifiedSessionMgr, srv.logger)
	detectDatabasesTool := analyze.NewAtomicDetectDatabasesTool(srv.logger)
	buildImageTool := build.NewAtomicBuildImageTool(pipelineOps, unifiedSessionMgr, srv.logger)
	pushImageTool := build.NewAtomicPushImageTool(pipelineOps, unifiedSessionMgr, srv.logger)
	generateManifestsTool := deploy.NewAtomicGenerateManifestsTool(pipelineOps, unifiedSessionMgr, srv.logger)
	scanImageTool := scan.NewAtomicScanImageSecurityToolLegacy(pipelineOps, unifiedSessionMgr, srv.logger)

	// Register analyze_repository tool
	s.server.Tool("analyze_repository", "Analyze repository structure and generate Dockerfile recommendations. Creates a new session for tracking analysis workflow",
		func(ctx *server.Context, args *analyze.AtomicAnalyzeRepositoryArgs) (*analyze.AtomicAnalysisResult, error) {
			result, err := analyzeRepoTool.ExecuteRepositoryAnalysis(adaptMCPContext(ctx), *args)
			return result, err
		})

	// Register detect_databases tool
	s.server.Tool("detect_databases", "Detect databases (PostgreSQL, MySQL, MongoDB, Redis) in repository through config files, Docker Compose, and environment variables. Uses session context for database detection workflow management",
		func(ctx *server.Context, args *analyze.DatabaseDetectionParams) (*analyze.DatabaseDetectionResult, error) {
			return detectDatabasesTool.ExecuteWithContext(adaptMCPContext(ctx), args)
		})

	// Register build_image tool
	s.server.Tool("build_image", "Build Docker images from Dockerfile. Uses session context for build configuration",
		func(ctx *server.Context, args *build.AtomicBuildImageArgs) (*build.AtomicBuildImageResult, error) {
			return buildImageTool.ExecuteWithContext(adaptMCPContext(ctx), args)
		})

	// Register push_image tool
	s.server.Tool("push_image", "Push Docker images to container registries",
		func(ctx *server.Context, args *build.AtomicPushImageArgs) (*build.AtomicPushImageResult, error) {
			return pushImageTool.ExecuteWithContext(adaptMCPContext(ctx), args)
		})

	// Register generate_manifests tool
	s.server.Tool("generate_manifests", "Generate Kubernetes manifests for containerized applications. Uses session context for manifest generation",
		func(ctx *server.Context, args *deploy.GenerateManifestsArgs) (*deploy.GenerateManifestsResult, error) {
			return generateManifestsTool.ExecuteWithContext(adaptMCPContext(ctx), args)
		})

	// Register generate_dockerfile tool
	generateDockerfileTool := analyze.NewAtomicGenerateDockerfileToolWithServices(serviceContainer, srv.logger)
	s.server.Tool("generate_dockerfile", "Generate optimized Dockerfile based on repository analysis. Uses session context for Dockerfile generation",
		func(ctx *server.Context, args *analyze.GenerateDockerfileArgs) (*analyze.GenerateDockerfileResult, error) {
			return generateDockerfileTool.ExecuteWithContext(adaptMCPContext(ctx), args)
		})

	// Register scan_image tool
	s.server.Tool("scan_image", "Scan Docker images for security vulnerabilities. Uses session context for vulnerability scanning",
		func(ctx *server.Context, args *scan.AtomicScanImageSecurityArgs) (*scan.AtomicScanImageSecurityResult, error) {
			return scanImageTool.ExecuteWithContext(adaptMCPContext(ctx), args)
		})

	// Register list_sessions tool
	s.server.Tool("list_sessions", "List all active and recent sessions with their status",
		func(ctx *server.Context, args *struct {
			Limit *int `json:"limit,omitempty"`
		}) (*struct {
			Sessions []map[string]interface{} `json:"sessions"`
			Total    int                      `json:"total"`
		}, error) {
			sessions, err := srv.sessionManager.ListSessionSummaries(adaptMCPContext(ctx))
			if err != nil {
				return &struct {
					Sessions []map[string]interface{} `json:"sessions"`
					Total    int                      `json:"total"`
				}{}, err
			}
			limit := 50
			if args.Limit != nil && *args.Limit > 0 {
				limit = *args.Limit
			}

			sessionData := make([]map[string]interface{}, 0)
			for i, session := range sessions {
				if i >= limit {
					break
				}
				sessionInfo := map[string]interface{}{
					"session_id":    session.ID,
					"created_at":    session.CreatedAt,
					"last_accessed": session.UpdatedAt, // Use UpdatedAt instead of LastAccessed
					"status":        session.Status,
					"labels":        session.Labels,
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

	s.logger.Info("Essential containerization tools registered successfully")
	return nil
}

func NewServer(ctx context.Context, config ServerConfig) (*Server, error) {
	logLevel := parseSlogLevel(config.LogLevel)

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel,
	})).With("component", "mcp-server")

	if config.StorePath != "" {
		if err := os.MkdirAll(filepath.Dir(config.StorePath), 0o755); err != nil {
			logger.Error("Failed to create storage directory", "error", err, "path", config.StorePath)
			return nil, errors.Wrapf(err, "server/core", "failed to create storage directory %s", config.StorePath)
		}
	}

	sessionManager, err := session.NewSessionManager(session.SessionManagerConfig{
		WorkspaceDir:      config.WorkspaceDir,
		MaxSessions:       config.MaxSessions,
		SessionTTL:        config.SessionTTL,
		MaxDiskPerSession: config.MaxDiskPerSession,
		TotalDiskLimit:    config.TotalDiskLimit,
		StorePath:         config.StorePath,
		Logger:            logger.With("component", "session_manager"),
	})
	if err != nil {
		logger.Error("Failed to initialize session manager", "error", err)
		return nil, errors.Wrap(err, "server/core", "failed to initialize session manager")
	}

	// TODO: Implement WorkspaceManager
	// workspaceManager, err := runtime.NewWorkspaceManager(ctx, runtime.WorkspaceConfig{
	//	BaseDir: config.WorkspaceDir,
	//	Logger:  logger.With("component", "workspace_manager"),
	// })
	// if err != nil {
	//	logger.Error("Failed to initialize workspace manager", "error", err)
	//	return nil, errors.Wrap(err, "server/core", "failed to initialize workspace manager")
	// }

	// TODO: Implement CircuitBreakerRegistry
	// circuitBreakers := execution.NewCircuitBreakerRegistry(logger.With("component", "circuit_breakers"))

	jobManager := workflow.NewJobManager(workflow.JobManagerConfig{
		MaxWorkers: config.MaxWorkers,
		JobTTL:     config.JobTTL,
		Logger:     logger.With("component", "job_manager"),
	})

	toolRegistry := runtime.NewToolRegistry(adaptSlogToZerolog(logger.With("component", "tool_registry")))

	// TODO: Implement Orchestrator
	// toolOrchestrator := orchestration.NewOrchestrator(
	//	orchestration.WithLogger(logger.With("component", "tool_orchestrator")),
	//	orchestration.WithTimeout(10*time.Minute),
	//	orchestration.WithMetrics(true),
	// )
	var toolOrchestrator api.Orchestrator // Temporary nil value

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
			Logger:         logger.With("component", "http_transport"),
			LogBodies:      config.LogHTTPBodies,
			MaxBodyLogSize: config.MaxBodyLogSize,
			LogLevel:       config.LogLevel,
		})
	default:
		return nil, errors.NewError().Messagef("unsupported transport type: %s", config.TransportType).WithLocation().Build()
	}

	gomcpManager := createRealGomcpManager(mcpTransport, logLevel, config.ServiceName, logger)

	server := &Server{
		config:         config,
		sessionManager: sessionManager,
		// workspaceManager: workspaceManager,
		// circuitBreakers:  circuitBreakers,
		jobManager:       jobManager,
		transport:        mcpTransport,
		logger:           logger,
		startTime:        time.Now(),
		toolOrchestrator: toolOrchestrator,
		toolRegistry:     toolRegistry,
		gomcpManager:     gomcpManager,
		conversationComponents: &ConversationComponents{
			isEnabled: false,
		},
	}

	logger.Info("MCP Server initialized successfully",
		"transport", config.TransportType,
		"workspace_dir", config.WorkspaceDir,
		"max_sessions", config.MaxSessions)

	return server, nil
}

// Start starts the MCP server
func (s *Server) Start(ctx context.Context) error {
	s.logger.Info("Starting Container Kit MCP Server",
		"transport", s.config.TransportType,
		"workspace_dir", s.config.WorkspaceDir,
		"max_sessions", s.config.MaxSessions)

	s.sessionManager.StartCleanupRoutine()

	if s.gomcpManager == nil {
		return errors.NewError().Messagef("gomcp manager is nil - server initialization failed").Build()
	}

	if simplifiedMgr, ok := s.gomcpManager.(*simplifiedGomcpManager); ok {
		if err := simplifiedMgr.RegisterTools(s); err != nil {
			return errors.NewError().Message("failed to register tools with gomcp").Cause(err).Build()
		}
	}

	if setter, ok := s.transport.(interface{ SetHandler(interface{}) }); ok {
		setter.SetHandler(s)
	}

	transportDone := make(chan error, 1)
	go func() {
		transportDone <- s.gomcpManager.Start(ctx)
	}()

	select {
	case <-ctx.Done():
		s.logger.Info("Server stopped by context cancellation")
		return ctx.Err()
	case err := <-transportDone:
		s.logger.Error("Transport stopped with error", "error", err)
		return err
	}
}

// Stop stops the MCP server
func (s *Server) Stop() error {
	s.logger.Info("Stopping MCP Server")

	if err := s.sessionManager.Stop(); err != nil {
		s.logger.Error("Failed to stop session manager", "error", err)
		return err
	}

	s.logger.Info("MCP Server stopped successfully")
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

	s.logger.Info("Gracefully shutting down MCP Server")

	if err := s.Stop(); err != nil {
		s.logger.Error("Error during server stop", "error", err)
		return err
	}

	s.logger.Info("MCP Server shutdown complete")
	return nil
}

// EnableConversationMode enables conversation mode
func (s *Server) EnableConversationMode(config core.ConsolidatedConversationConfig) error {
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

// GetName returns the server name
func (s *Server) GetName() string {
	return "container-kit-mcp-server"
}
