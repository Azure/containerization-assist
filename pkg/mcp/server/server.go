package server

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/Azure/container-kit/pkg/mcp/errors"
	"github.com/localrivet/gomcp/server"
)

// Simple local session types to replace deleted domain/session package
type SessionManager interface {
	GetSession(ctx context.Context, sessionID string) (*SessionState, error)
	GetSessionTyped(ctx context.Context, sessionID string) (*SessionState, error)
	GetSessionConcrete(ctx context.Context, sessionID string) (*SessionState, error)
	GetOrCreateSession(ctx context.Context, sessionID string) (*SessionState, error)
	GetOrCreateSessionTyped(ctx context.Context, sessionID string) (*SessionState, error)
	UpdateSession(ctx context.Context, sessionID string, updateFunc func(*SessionState) error) error
	ListSessionsTyped(ctx context.Context) ([]*SessionState, error)
	ListSessionSummaries(ctx context.Context) ([]*SessionSummary, error)
	UpdateJobStatus(ctx context.Context, sessionID, jobID string, status JobStatus, result interface{}, err error) error
	StartCleanupRoutine(ctx context.Context) error
	Stop(ctx context.Context) error
}

type SessionState struct {
	SessionID string
	CreatedAt time.Time
	UpdatedAt time.Time
	ExpiresAt time.Time
	Status    string
	Stage     string
	UserID    string
	Labels    map[string]string
	Metadata  map[string]interface{}
}

type SessionSummary struct {
	ID     string
	Labels map[string]string
}

type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
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

// adaptSlogToLogging creates a *slog.Logger from an slog.Logger
func adaptSlogToLogging(slogLogger *slog.Logger) *slog.Logger {
	// Just return the logger as-is since we're using direct slog now
	return slogLogger
}

// adaptMCPContext creates a context.Context from a gomcp server.Context
func adaptMCPContext(mcpCtx *server.Context) context.Context {
	// For now, return a background context
	// In a real implementation, we might need to extract request metadata
	return context.Background()
}

// serverImpl represents the consolidated MCP server implementation
type serverImpl struct {
	config         ServerConfig
	sessionManager SessionManager
	// workspaceManager *runtime.WorkspaceManager // TODO: Type needs to be implemented
	// circuitBreakers  *execution.CircuitBreakerRegistry // TODO: Type needs to be implemented
	// TODO: Fix job manager type after migration
	// jobManager api.JobExecutionService
	// transport removed - using gomcp server directly
	logger    *slog.Logger
	startTime time.Time

	// Direct gomcp server instead of manager abstraction
	gomcpServer        server.Server
	isGomcpInitialized bool

	shutdownMutex  sync.Mutex
	isShuttingDown bool
}

// ConversationComponents represents conversation mode components
type ConversationComponents struct {
	isEnabled bool
}

// registerTools registers the single comprehensive workflow tool
func (s *serverImpl) registerTools() error {
	if s.gomcpServer == nil {
		return errors.New(errors.CodeInternalError, "server", "gomcp server not initialized", nil)
	}

	s.logger.Info("Registering single comprehensive workflow tool for AI-powered containerization")

	// Register ONLY the workflow tool - this ensures AI assistants use complete workflows
	// instead of individual atomic tools for true workflow-focused operation
	if err := RegisterWorkflowTools(s.gomcpServer, s.logger); err != nil {
		return errors.New(errors.CodeToolExecutionFailed, "server", "failed to register workflow tools", err)
	}

	// Keep essential diagnostic tools
	s.gomcpServer.Tool("ping", "Simple ping tool to test MCP connectivity",
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

	s.gomcpServer.Tool("server_status", "Get basic server status information",
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

	s.logger.Info("Workflow tools registered successfully - AI will now use complete workflows instead of atomic tools")
	return nil
}

// extractSessionID extracts or generates a session ID from gomcp input
func extractSessionID(input map[string]interface{}) string {
	if sessionID, ok := input["session_id"].(string); ok && sessionID != "" {
		return sessionID
	}
	// Generate a new session ID if not provided
	return fmt.Sprintf("session_%d", time.Now().UnixNano())
}

// dependencies holds internal dependencies for the server
type dependencies struct {
	sessionManager SessionManager
	logger         *slog.Logger
}

// NewServer creates a new MCP server with the given options
// This is the primary public API for creating MCP servers
func NewServer(ctx context.Context, logger *slog.Logger, opts ...Option) (api.MCPServer, error) {
	// Build configuration from functional options
	config := DefaultServerConfig()
	for _, opt := range opts {
		opt(&config)
	}
	// Create server logger
	serverLogger := logger.With("component", "mcp-server")
	
	// Create internal dependencies
	sessionManager := newMemorySessionManager(logger, config.SessionTTL, config.MaxSessions)
	serverLogger.Info("Created enhanced in-memory session manager", 
		"default_ttl", config.SessionTTL,
		"max_sessions", config.MaxSessions)
	
	deps := &dependencies{
		sessionManager: sessionManager,
		logger:         serverLogger,
	}

	if config.StorePath != "" {
		if err := os.MkdirAll(filepath.Dir(config.StorePath), 0o755); err != nil {
			serverLogger.Error("Failed to create storage directory", "error", err, "path", config.StorePath)
			return nil, errors.New(errors.CodeIoError, "server", fmt.Sprintf("failed to create storage directory %s", config.StorePath), err)
		}
	}

	// Validate workspace directory exists or can be created
	if config.WorkspaceDir != "" {
		if err := os.MkdirAll(config.WorkspaceDir, 0o755); err != nil {
			serverLogger.Error("Failed to create workspace directory", "error", err, "path", config.WorkspaceDir)
			return nil, errors.New(errors.CodeIoError, "server", fmt.Sprintf("failed to create workspace directory %s", config.WorkspaceDir), err)
		}
	}

	// sessionManager is already set above

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

	// TODO: Create job manager from service container
	// TODO: Fix after migration
	// var jobManager api.JobExecutionService

	// Note: Tool registry and orchestrator removed for workflow-focused operation

	// Note: Transport creation removed - using gomcp server directly
	// The gomcp server handles stdio/http transport internally via .AsStdio() or .AsHTTP()

	// Note: Service container removed - using direct dependency injection

	server := &serverImpl{
		config:         config,
		sessionManager: deps.sessionManager,
		// workspaceManager: workspaceManager,
		// circuitBreakers:  circuitBreakers,
		// jobManager:       jobManager, // TODO: Fix after migration
		// transport removed - using gomcp server directly
		logger:    deps.logger,
		startTime: time.Now(),
	}

	deps.logger.Info("MCP Server initialized successfully",
		"transport", config.TransportType,
		"workspace_dir", config.WorkspaceDir,
		"max_sessions", config.MaxSessions)

	return server, nil
}

// Start starts the MCP server
func (s *serverImpl) Start(ctx context.Context) error {
	s.logger.Info("Starting Container Kit MCP Server",
		"transport", s.config.TransportType,
		"workspace_dir", s.config.WorkspaceDir,
		"max_sessions", s.config.MaxSessions)

	if err := s.sessionManager.StartCleanupRoutine(ctx); err != nil {
		s.logger.Error("Failed to start cleanup routine", "error", err)
		return err
	}

	// Initialize gomcp server directly without manager abstraction
	if !s.isGomcpInitialized {
		s.logger.Info("Initializing gomcp server")
		s.gomcpServer = server.NewServer("Container Kit MCP Server",
			server.WithLogger(s.logger),
			server.WithProtocolVersion("1.0.0"),
		).AsStdio()

		if s.gomcpServer == nil {
			return errors.New(errors.CodeInternalError, "transport", "failed to create gomcp stdio server", nil)
		}

		// Register tools directly
		if err := s.registerTools(); err != nil {
			return errors.New(errors.CodeToolExecutionFailed, "transport", "failed to register tools with gomcp", err)
		}

		s.isGomcpInitialized = true
		s.logger.Info("Gomcp server initialized successfully")
	}

	// Transport setter removed - using gomcp server directly

	transportDone := make(chan error, 1)
	go func() {
		if mcpServer, ok := s.gomcpServer.(interface{ Run() error }); ok {
			transportDone <- mcpServer.Run()
		} else {
			transportDone <- errors.New(errors.CodeNotImplemented, "transport", "server does not implement Run() method", nil)
		}
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

// Shutdown gracefully shuts down the server with proper context handling
func (s *serverImpl) Shutdown(ctx context.Context) error {
	s.shutdownMutex.Lock()
	defer s.shutdownMutex.Unlock()

	if s.isShuttingDown {
		return nil // Already shutting down
	}
	s.isShuttingDown = true

	s.logger.Info("Gracefully shutting down MCP Server")

	// Stop session manager with context awareness
	done := make(chan error, 1)
	go func() {
		if err := s.sessionManager.Stop(ctx); err != nil {
			s.logger.Error("Failed to stop session manager", "error", err)
			done <- err
			return
		}
		done <- nil
	}()

	// Wait for shutdown or context cancellation
	select {
	case <-ctx.Done():
		s.logger.Warn("Shutdown cancelled by context", "error", ctx.Err())
		return ctx.Err()
	case err := <-done:
		if err != nil {
			return err
		}
	}

	s.logger.Info("MCP Server shutdown complete")
	return nil
}

// Stop stops the MCP server (implements api.MCPServer)
func (s *serverImpl) Stop(ctx context.Context) error {
	// Use the provided context for shutdown
	return s.Shutdown(ctx)
}

// EnableConversationMode enables conversation mode (workflow-focused server - no-op)
func (s *serverImpl) EnableConversationMode(_ interface{}) error {
	s.logger.Info("Conversation mode not supported in workflow-focused server")
	return nil
}

// IsConversationModeEnabled returns whether conversation mode is enabled (always false)
func (s *serverImpl) IsConversationModeEnabled() bool {
	return false // Workflow-focused server doesn't support conversation mode
}

// GetName returns the server name
func (s *serverImpl) GetName() string {
	return "container-kit-mcp-server"
}

// GetStats returns server statistics
func (s *serverImpl) GetStats() (interface{}, error) {
	return map[string]interface{}{
		"name":              s.GetName(),
		"uptime":            time.Since(s.startTime).String(),
		"status":            "running",
		"session_count":     0, // TODO: Get actual session count
		"transport_type":    s.config.TransportType,
		"conversation_mode": s.IsConversationModeEnabled(),
	}, nil
}

// GetSessionManagerStats returns session manager statistics
func (s *serverImpl) GetSessionManagerStats() (interface{}, error) {
	if s.sessionManager != nil {
		// TODO: Add proper session manager stats when interface is available
		return map[string]interface{}{
			"active_sessions": 0,
			"total_sessions":  0,
			"max_sessions":    s.config.MaxSessions,
		}, nil
	}
	return map[string]interface{}{
		"error": "session manager not initialized",
	}, nil
}

// ============================================================================
// api.MCPServer Implementation Methods
// ============================================================================

// RegisterTool registers a tool with the server (implements api.MCPServer)
func (s *serverImpl) RegisterTool(tool api.Tool) error {
	// For workflow-focused server, tools are registered directly in gomcp server
	// during registerTools() call in Start() method
	s.logger.Info("Tool registration requested", "tool", tool.Name())
	return nil
}

// GetRegistry returns the tool registry (implements api.MCPServer)
func (s *serverImpl) GetRegistry() api.Registry {
	// For workflow-focused server, we don't expose a separate registry
	// Tools are managed directly by the gomcp server
	return nil
}

// GetSessionManager returns the session manager (implements api.MCPServer)
func (s *serverImpl) GetSessionManager() interface{} {
	return s.sessionManager
}

// GetOrchestrator returns the tool orchestrator (implements api.MCPServer)
func (s *serverImpl) GetOrchestrator() api.Orchestrator {
	// For workflow-focused server, orchestration happens within workflows
	// No separate orchestrator needed
	return nil
}

