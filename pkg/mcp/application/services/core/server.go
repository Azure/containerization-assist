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

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	coreinterfaces "github.com/Azure/container-kit/pkg/mcp/application/core"
	"github.com/Azure/container-kit/pkg/mcp/application/orchestration"
	"github.com/Azure/container-kit/pkg/mcp/application/orchestration/execution"
	"github.com/Azure/container-kit/pkg/mcp/application/orchestration/registry"
	"github.com/Azure/container-kit/pkg/mcp/application/orchestration/workflow"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors/codes"
	"github.com/Azure/container-kit/pkg/mcp/domain/session"
	"github.com/Azure/container-kit/pkg/mcp/infra/runtime"
	"github.com/Azure/container-kit/pkg/mcp/infra/transport"
	"github.com/Azure/container-kit/pkg/mcp/services"
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

// Use canonical GomcpManager interface from interfaces package

// Server represents the MCP server
type Server struct {
	config           ServerConfig
	sessionManager   *session.SessionManager
	workspaceManager *runtime.WorkspaceManager
	circuitBreakers  *execution.CircuitBreakerRegistry
	jobManager       *workflow.JobManager
	transport        interface{} // stdio or http transport
	logger           *slog.Logger
	startTime        time.Time

	// Canonical orchestration system
	toolOrchestrator api.Orchestrator
	toolRegistry     *registry.ToolRegistry

	// Conversation mode components
	conversationComponents *ConversationComponents

	// Gomcp manager for lean tool registration
	gomcpManager api.GomcpManager

	// Service container for service-based architecture
	serviceContainer services.ServiceContainer

	// Shutdown coordination
	shutdownMutex  sync.Mutex
	isShuttingDown bool
}

// NewServer creates a new MCP server
func NewServer(ctx context.Context, config ServerConfig) (*Server, error) {
	return createServer(ctx, config, nil)
}

// NewServerWithServices creates a new MCP server using service-based architecture
func NewServerWithServices(ctx context.Context, config ServerConfig, container interface{}) (*Server, error) {
	return createServiceBasedServer(ctx, config, container)
}

// createServiceBasedServer creates a new server using the service container architecture
func createServiceBasedServer(ctx context.Context, config ServerConfig, container interface{}) (*Server, error) {
	// Cast container to ServiceContainer
	serviceContainer, ok := container.(services.ServiceContainer)
	if !ok {
		return nil, fmt.Errorf("invalid service container provided")
	}

	// Create server using service-based architecture
	return createServerWithServices(ctx, config, serviceContainer)
}

// NewServerWithUnifiedSessionManager creates a new MCP server using a unified session manager
func NewServerWithUnifiedSessionManager(ctx context.Context, config ServerConfig, unifiedSessionManager session.UnifiedSessionManager) (*Server, error) {
	return createServer(ctx, config, unifiedSessionManager)
}

// createServer is the common server creation logic
func createServer(ctx context.Context, config ServerConfig, unifiedSessionManager session.UnifiedSessionManager) (*Server, error) {
	logLevel := parseSlogLevel(config.LogLevel)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel,
	})).With("component", "mcp-server")

	if config.StorePath != "" {
		if err := os.MkdirAll(filepath.Dir(config.StorePath), 0o755); err != nil {
			logger.Error("Failed to create storage directory", "error", err, "path", config.StorePath)
			return nil, errors.NewError().
				Code(codes.SYSTEM_ERROR).
				Message("Failed to create storage directory for server persistence").
				Type(errors.ErrTypeSystem).
				Severity(errors.SeverityHigh).
				Cause(err).
				Context("store_path", config.StorePath).
				Context("directory", filepath.Dir(config.StorePath)).
				Context("component", "server_initialization").
				Suggestion("Check directory permissions and available disk space for storage").
				WithLocation().
				Build()
		}
	}

	var sessionManager *session.SessionManager
	if unifiedSessionManager != nil {
		logger.Info("Using provided unified session manager")
		if sm, ok := unifiedSessionManager.(*session.SessionManager); ok {
			sessionManager = sm
			logger.Info("Successfully using unified session manager for server")
		} else {
			logger.Warn("Unified session manager is not a SessionManager instance, falling back to regular session manager")
		}
	}
	if sessionManager == nil {
		var err error
		sessionManager, err = session.NewSessionManager(session.SessionManagerConfig{
			WorkspaceDir:      config.WorkspaceDir,
			MaxSessions:       config.MaxSessions,
			SessionTTL:        config.SessionTTL,
			MaxDiskPerSession: config.MaxDiskPerSession,
			TotalDiskLimit:    config.TotalDiskLimit,
			StorePath:         "", // Use in-memory store for now
			Logger:            logger.With("component", "session_manager"),
		})
		if err != nil {
			logger.Error("Failed to initialize session manager", "error", err)
			return nil, errors.NewError().
				Code(codes.SYSTEM_ERROR).
				Message("Failed to initialize session management system").
				Type(errors.ErrTypeSystem).
				Severity(errors.SeverityHigh).
				Cause(err).
				Context("component", "server_initialization").
				Context("workspace_dir", config.WorkspaceDir).
				Context("max_sessions", config.MaxSessions).
				Suggestion("Check workspace directory permissions and session configuration").
				WithLocation().
				Build()
		}
	}

	workspaceManager, err := runtime.NewWorkspaceManager(ctx, runtime.WorkspaceConfig{
		BaseDir:           config.WorkspaceDir,
		MaxSizePerSession: config.MaxDiskPerSession,
		TotalMaxSize:      config.TotalDiskLimit,
		Cleanup:           true, // Enable auto-cleanup
		SandboxEnabled:    config.SandboxEnabled,
		Logger:            logger.With("component", "workspace_manager"),
	})
	if err != nil {
		logger.Error("Failed to initialize workspace manager", "error", err)
		return nil, errors.NewError().
			Code(codes.SYSTEM_ERROR).
			Message("Failed to initialize workspace management system").
			Type(errors.ErrTypeSystem).
			Severity(errors.SeverityHigh).
			Cause(err).
			Context("component", "server_initialization").
			Context("base_dir", config.WorkspaceDir).
			Context("sandbox_enabled", config.SandboxEnabled).
			Suggestion("Check workspace directory permissions and sandbox configuration").
			WithLocation().
			Build()
	}

	// Initialize circuit breakers
	circuitBreakers := execution.CreateDefaultCircuitBreakers(logger.With("component", "circuit_breaker"))

	// Initialize job manager
	jobManager := workflow.NewJobManager(workflow.JobManagerConfig{
		MaxWorkers: config.MaxWorkers,
		JobTTL:     config.JobTTL,
		Logger:     logger.With("component", "job_manager"),
	})

	// Initialize transport
	var transportInstance interface{}
	switch config.TransportType {
	case "http":
		httpConfig := transport.HTTPTransportConfig{
			Port:           config.HTTPPort,
			CORSOrigins:    config.CORSOrigins,
			APIKey:         config.APIKey,
			RateLimit:      config.RateLimit,
			Logger:         logger.With("component", "http_transport"),
			LogBodies:      config.LogHTTPBodies,
			MaxBodyLogSize: config.MaxBodyLogSize,
			LogLevel:       config.LogLevel,
		}
		transportInstance = transport.NewHTTPTransport(httpConfig)
	case "stdio":
		fallthrough
	default:
		// Create stdio transport
		transportInstance = transport.NewStdioTransport() // TODO: Update when transport supports slog
	}

	// Create gomcp manager with builder pattern
	gomcpConfig := GomcpConfig{
		Name:            "Container-Kit MCP",
		ProtocolVersion: "2024-11-05",
		LogLevel:        slog.LevelInfo, // Use default info level
	}
	gomcpManager := NewGomcpManager(gomcpConfig).
		WithTransport(transportInstance).
		WithLogger(logger)

	// Set GomcpManager on transport for proper lifecycle management
	// Use type assertion
	if setter, ok := transportInstance.(interface{ SetGomcpManager(interface{}) }); ok {
		setter.SetGomcpManager(gomcpManager)
	}

	// Telemetry disabled - using standard logging

	// Initialize canonical tool orchestrator
	toolRegistry := registry.NewToolRegistry(logger.With("component", "tool_registry"))

	// Use session manager directly with orchestrator, with unified session manager support if available
	// Use unified orchestrator for tool coordination
	toolOrchestrator := orchestration.NewOrchestrator(
		orchestration.WithLogger(logger.With("component", "tool_orchestrator")),
		orchestration.WithTimeout(10*time.Minute),
		orchestration.WithMetrics(true),
	)
	logger.Info("Created unified tool orchestrator")

	server := &Server{
		config:           config,
		sessionManager:   sessionManager,
		workspaceManager: workspaceManager,
		circuitBreakers:  circuitBreakers,
		jobManager:       jobManager,
		transport:        transportInstance,
		logger:           logger,
		startTime:        time.Now(),
		toolOrchestrator: toolOrchestrator,
		toolRegistry:     toolRegistry,
		gomcpManager:     gomcpManager,
	}

	return server, nil
}

// createServerWithServices creates a server using the service container architecture
func createServerWithServices(ctx context.Context, config ServerConfig, serviceContainer services.ServiceContainer) (*Server, error) {
	logLevel := parseSlogLevel(config.LogLevel)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel,
	})).With("component", "mcp-server-services")

	logger.Info("Creating service-based MCP server")

	// Create storage directory (still needed for basic file operations)
	if config.StorePath != "" {
		if err := os.MkdirAll(filepath.Dir(config.StorePath), 0o755); err != nil {
			logger.Error("Failed to create storage directory", "error", err, "path", config.StorePath)
			return nil, errors.NewError().
				Code(codes.SYSTEM_ERROR).
				Message("Failed to create storage directory for server persistence").
				Type(errors.ErrTypeSystem).
				Severity(errors.SeverityHigh).
				Cause(err).
				Build()
		}
	}

	// The service container already provides all the business logic services we need:
	// - SessionStore and SessionState (instead of SessionManager)
	// - BuildExecutor (for build operations)
	// - ToolRegistry (for tool management)
	// - WorkflowExecutor (for orchestration)
	// - Scanner (for security scanning)
	// - ConfigValidator (for validation)
	// - ErrorReporter (for error handling)

	// We only need to create infrastructure components that handle transport, etc.
	// Business logic should come from the service container

	// Circuit breakers for resilience (infrastructure component)
	circuitBreakers := execution.NewCircuitBreakerRegistry(logger.With("component", "circuit_breakers"))

	// Workspace manager for file management (infrastructure component)
	workspaceManager, err := runtime.NewWorkspaceManagerWithServices(config, serviceContainer, logger.With("component", "workspace_manager"))
	if err != nil {
		logger.Error("Failed to create workspace manager", "error", err)
		return nil, fmt.Errorf("failed to create workspace manager: %w", err)
	}

	// Job manager for async operations tracking (infrastructure component)
	jobManager, err := workflow.NewJobManagerWithServices(serviceContainer, logger.With("component", "job_manager"))
	if err != nil {
		logger.Error("Failed to create job manager", "error", err)
		return nil, fmt.Errorf("failed to create job manager: %w", err)
	}

	// Transport setup (infrastructure component)
	var transportInstance interface{}
	switch config.TransportType {
	case "http":
		transportInstance = transport.NewHTTPTransport(transport.HTTPTransportConfig{
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
		transportInstance = transport.NewStdioTransport()
	}

	// Get business logic services from the service container
	toolRegistryService := serviceContainer.ToolRegistry()
	workflowExecutorService := serviceContainer.WorkflowExecutor()

	// Create facade infrastructure components (still use zerolog for now)
	// TODO: Migrate these to use slog and service container services directly
	toolRegistry := registry.NewToolRegistry(logger.With("component", "tool_registry"))
	toolOrchestrator := orchestration.NewOrchestrator(
		orchestration.WithLogger(logger.With("component", "tool_orchestrator")),
		orchestration.WithTimeout(10*time.Minute),
		orchestration.WithMetrics(true),
	)

	// Log that services are available but not yet integrated
	logger.Info("Service container services available but using facade pattern",
		"tool_registry_service", toolRegistryService != nil,
		"workflow_executor_service", workflowExecutorService != nil)

	// Suppress unused variable warnings temporarily
	_ = toolRegistryService
	_ = workflowExecutorService

	// Gomcp manager (infrastructure component)
	gomcpConfig := GomcpConfig{
		Name:            "Container-Kit MCP",
		ProtocolVersion: "2024-11-05",
		LogLevel:        slog.LevelInfo,
	}
	gomcpManager := NewGomcpManager(gomcpConfig).
		WithTransport(transportInstance).
		WithLogger(logger.With("component", "gomcp"))

	// Create server instance
	// Create server with service container integration
	server := &Server{
		config:           config,
		sessionManager:   nil, // Business logic comes from service container
		workspaceManager: workspaceManager,
		circuitBreakers:  circuitBreakers,
		jobManager:       jobManager,
		transport:        transportInstance,
		logger:           logger,
		startTime:        time.Now(),
		toolOrchestrator: toolOrchestrator, // From service container
		toolRegistry:     toolRegistry,     // From service container
		gomcpManager:     gomcpManager,
		serviceContainer: serviceContainer, // Provide access to all services
	}

	logger.Info("Service-based MCP server created successfully",
		"services_available", []string{
			"SessionStore", "SessionState", "BuildExecutor", "ToolRegistry",
			"WorkflowExecutor", "Scanner", "ConfigValidator", "ErrorReporter",
		})

	return server, nil
}

// IsConversationModeEnabled checks if conversation mode is enabled
func (s *Server) IsConversationModeEnabled() bool {
	return s.conversationComponents != nil
}

// GetTransport returns the server's transport
func (s *Server) GetTransport() interface{} {
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
	if s.gomcpManager == nil {
		return errors.Internal("core/server", "server not properly initialized")
	}

	s.logger.Info("Starting tool schema export", "output_path", outputPath)

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
			"initialized":   s.gomcpManager != nil,
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

	s.logger.Info("Schema export completed successfully",
		"output_path", outputPath,
		"file_size", int64(len(data)))

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
			"description": fmt.Sprintf("Atomic tool for %s operations", toolName[7:]),
			"available":   true,
			"schema_note": "Full schema available via proper tool registry access",
		}
	}

	return tools
}

// GetLogger returns the server's logger
func (s *Server) GetLogger() interface{} {
	return s.logger
}

// GetStats returns server statistics (implements core.Server interface)
func (s *Server) GetStats() *coreinterfaces.ServerStats {
	stats, err := s.GetStatsWithContext(context.Background())
	if err != nil {
		s.logger.Error("Failed to get server stats", "error", err)
		return &coreinterfaces.ServerStats{
			Transport: s.config.TransportType,
			Sessions:  &coreinterfaces.SessionManagerStats{},
			Workspace: &coreinterfaces.WorkspaceStats{},
			Uptime:    time.Since(s.startTime),
			StartTime: s.startTime,
		}
	}
	return stats
}

// GetCircuitBreakers returns the server's circuit breakers
func (s *Server) GetCircuitBreakers() *execution.CircuitBreakerRegistry {
	return s.circuitBreakers
}

// GetJobManager returns the server's job manager
func (s *Server) GetJobManager() *workflow.JobManager {
	return s.jobManager
}

// GetGomcpManager returns the server's gomcp manager
func (s *Server) GetGomcpManager() api.GomcpManager {
	return s.gomcpManager
}

// GetToolOrchestrator returns the server's tool orchestrator
func (s *Server) GetToolOrchestrator() api.Orchestrator {
	return s.toolOrchestrator
}

// GetToolRegistry returns the server's tool registry
func (s *Server) GetToolRegistry() *registry.ToolRegistry {
	return s.toolRegistry
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	s.shutdownMutex.Lock()
	defer s.shutdownMutex.Unlock()

	if s.isShuttingDown {
		return nil // Already shutting down
	}
	s.isShuttingDown = true

	s.logger.Info("Starting server shutdown")

	// Stop job manager
	if s.jobManager != nil {
		s.jobManager.Stop()
	}

	s.logger.Info("Server shutdown completed")
	return nil
}

// GetServiceContainer returns the service container for service-based architecture
func (s *Server) GetServiceContainer() services.ServiceContainer {
	return s.serviceContainer
}
