package core

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/core/orchestration"
	"github.com/Azure/container-kit/pkg/mcp/core/transport"
	"github.com/Azure/container-kit/pkg/mcp/errors"
	"github.com/Azure/container-kit/pkg/mcp/errors/rich"
	"github.com/localrivet/gomcp/server"
)

// GomcpConfig holds configuration for the gomcp server
type GomcpConfig struct {
	Name            string
	ProtocolVersion string
	LogLevel        slog.Level
}

// GomcpManager manages the gomcp server and tool registration
type GomcpManager struct {
	server           server.Server
	config           GomcpConfig
	logger           slog.Logger
	transport        interface{}                        // Injected transport (stdio or http)
	isInitialized    bool                               // Prevent mutation after creation
	startTime        time.Time                          // Server start time for uptime tracking
	toolOrchestrator *orchestration.MCPToolOrchestrator // Reference to tool orchestrator
}

// NewGomcpManager creates a new gomcp manager with builder pattern
func NewGomcpManager(config GomcpConfig) *GomcpManager {
	// Create slog logger
	slogHandler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: config.LogLevel,
	})
	logger := *slog.New(slogHandler)

	return &GomcpManager{
		config:        config,
		logger:        logger,
		isInitialized: false,
		startTime:     time.Now(),
	}
}

// WithTransport sets the transport for the gomcp manager
func (gm *GomcpManager) WithTransport(t interface{}) *GomcpManager {
	if gm.isInitialized {
		gm.logger.Error("cannot set transport: manager already initialized")
		return gm
	}
	gm.transport = t
	return gm
}

// WithLogger updates the logger for the gomcp manager
func (gm *GomcpManager) WithLogger(logger slog.Logger) *GomcpManager {
	if gm.isInitialized {
		gm.logger.Error("cannot set logger: manager already initialized")
		return gm
	}
	gm.logger = logger
	return gm
}

// Initialize creates and configures the gomcp server
func (gm *GomcpManager) Initialize() error {
	if gm.isInitialized {
		return rich.NewError().
			Code(rich.CodeResourceAlreadyExists).
			Type(rich.ErrTypeInternal).
			Severity(rich.SeverityMedium).
			Message("manager already initialized").
			Context("module", "core/gomcp-manager").
			Context("component", "GomcpManager").
			Context("is_initialized", gm.isInitialized).
			Suggestion("Use an existing manager instance or create a new one").
			WithLocation().
			Build()
	}

	// Validate transport is set
	if gm.transport == nil {
		return rich.NewError().
			Code(rich.CodeMissingParameter).
			Type(rich.ErrTypeConfiguration).
			Severity(rich.SeverityHigh).
			Message("transport must be set before initialization").
			Context("module", "core/gomcp-manager").
			Context("component", "GomcpManager").
			Context("transport_set", false).
			Suggestion("Call SetTransport() with a valid transport before Initialize()").
			WithLocation().
			Build()
	}

	// Create gomcp server with stdio transport
	// AsStdio() must be chained directly with NewServer() for proper initialization
	gm.server = server.NewServer(gm.config.Name,
		server.WithLogger(&gm.logger),
		server.WithProtocolVersion(gm.config.ProtocolVersion),
	).AsStdio()

	// Verify server was created successfully
	if gm.server == nil {
		return rich.NewError().
			Code(rich.CodeInternalError).
			Type(rich.ErrTypeInternal).
			Severity(rich.SeverityCritical).
			Message("failed to create stdio server: NewServer().AsStdio() returned nil").
			Context("module", "core/gomcp-manager").
			Context("component", "GomcpManager").
			Context("server_name", gm.config.Name).
			Context("protocol_version", gm.config.ProtocolVersion).
			Suggestion("Check server configuration and ensure all dependencies are available").
			WithLocation().
			Build()
	}

	gm.isInitialized = true
	return nil
}

// SetToolOrchestrator sets the tool orchestrator reference
func (gm *GomcpManager) SetToolOrchestrator(orchestrator interface{}) {
	if orch, ok := orchestrator.(*orchestration.MCPToolOrchestrator); ok {
		gm.toolOrchestrator = orch
	}
}

// GetServer returns the underlying gomcp server
func (gm *GomcpManager) GetServer() server.Server {
	return gm.server
}

// GetTransport returns the configured transport
func (gm *GomcpManager) GetTransport() interface{} {
	return gm.transport
}

// RegisterTools registers tools (simplified stub for compatibility)
func (gm *GomcpManager) RegisterTools(server *Server) error {
	// Simplified approach - tools are registered directly with server
	// This is a compatibility stub for the old architecture
	return nil
}

// StartServer starts the gomcp server after all tools are registered
func (gm *GomcpManager) StartServer() error {
	if !gm.isInitialized {
		return rich.NewError().
			Code(rich.CodeInternalError).
			Type(rich.ErrTypeInternal).
			Severity(rich.SeverityHigh).
			Message("manager not initialized").
			Context("module", "core/gomcp-manager").
			Context("component", "GomcpManager").
			Context("is_initialized", false).
			Suggestion("Call Initialize() before StartServer()").
			WithLocation().
			Build()
	}
	if gm.server == nil {
		return rich.NewError().
			Code(rich.CodeInternalError).
			Type(rich.ErrTypeInternal).
			Severity(rich.SeverityCritical).
			Message("server is nil - initialization may have failed").
			Context("module", "core/gomcp-manager").
			Context("component", "GomcpManager").
			Context("is_initialized", gm.isInitialized).
			Context("server_nil", true).
			Suggestion("Re-run Initialize() or check for initialization errors").
			WithLocation().
			Build()
	}
	gm.logger.Info("Starting gomcp server with all tools registered")
	return gm.server.Run()
}

// IsInitialized returns whether the manager has been initialized
func (gm *GomcpManager) IsInitialized() bool {
	return gm.isInitialized
}

// Shutdown gracefully shuts down the gomcp server
func (gm *GomcpManager) Shutdown(ctx context.Context) error {
	if !gm.isInitialized {
		return nil
	}

	gm.logger.Info("shutting down gomcp server")

	// Create error collector for potential errors during shutdown
	var shutdownErrors []error

	// Shutdown the underlying gomcp server if available
	if gm.server != nil {
		select {
		case <-ctx.Done():
			gm.logger.Warn("shutdown context cancelled before server shutdown")
			shutdownErrors = append(shutdownErrors, ctx.Err())
		default:
			// Attempt graceful shutdown of the server
			if gm.server != nil {
				if err := gm.server.Shutdown(); err != nil {
					gm.logger.Error("error shutting down gomcp server", "error", err)
					shutdownErrors = append(shutdownErrors, err)
				} else {
					gm.logger.Info("gomcp server shut down successfully")
				}
			} else {
				gm.logger.Warn("gomcp server is nil during shutdown")
			}
		}
	}

	// Shutdown the transport if available
	if gm.transport != nil {
		select {
		case <-ctx.Done():
			gm.logger.Warn("shutdown context cancelled before transport shutdown")
			shutdownErrors = append(shutdownErrors, ctx.Err())
		default:
			// Stop the transport
			if stopper, ok := gm.transport.(interface{ Stop(context.Context) error }); ok {
				if err := stopper.Stop(ctx); err != nil {
					gm.logger.Error("error stopping transport", "error", err)
					shutdownErrors = append(shutdownErrors, err)
				} else {
					gm.logger.Info("transport stopped successfully")
				}
			}
		}
	}

	// Mark as not initialized
	gm.isInitialized = false

	// Return first error if any occurred
	if len(shutdownErrors) > 0 {
		return errors.Wrapf(shutdownErrors[0], "core/gomcp-manager", "shutdown completed with %d errors", len(shutdownErrors))
	}

	gm.logger.Info("gomcp manager shutdown completed successfully")
	return nil
}

// RegisterHTTPHandlers registers tool handlers with the HTTP transport
func (gm *GomcpManager) RegisterHTTPHandlers(transportInstance interface{}) error {
	if !gm.isInitialized {
		return errors.Internal("core/gomcp-manager", "manager not initialized")
	}

	gm.logger.Info("attempting to register HTTP handlers for transport", "transport_type", fmt.Sprintf("%T", transportInstance))

	// Check if transport is HTTP
	httpTransport, ok := transportInstance.(*transport.HTTPTransport)
	if !ok {
		gm.logger.Info("transport is not HTTP, skipping HTTP handler registration")
		return nil // Not an HTTP transport, skip registration
	}

	gm.logger.Info("registering HTTP handlers for core tools")

	// Register analyze_repository redirect handler
	analyzeHandler := transport.ToolHandler(func(ctx context.Context, args interface{}) (interface{}, error) {
		// Use the tool orchestrator to execute the tool
		if gm.toolOrchestrator != nil {
			gm.logger.Info("executing analyze_repository via orchestrator")
			return gm.toolOrchestrator.ExecuteTool(ctx, "analyze_repository", args)
		}
		gm.logger.Error("tool orchestrator is nil")
		return nil, errors.Internal("core/gomcp-manager", "tool orchestrator not available")
	})
	if err := httpTransport.RegisterTool("analyze_repository", "Analyze a repository to detect language, framework, and containerization requirements. Creates a new session to track the analysis workflow", analyzeHandler); err != nil {
		gm.logger.Error("failed to register analyze_repository", "error", err)
		return err
	}
	gm.logger.Info("registered analyze_repository HTTP handler")

	// Register generate_dockerfile handler
	dockerfileHandler := transport.ToolHandler(func(ctx context.Context, args interface{}) (interface{}, error) {
		if gm.toolOrchestrator != nil {
			return gm.toolOrchestrator.ExecuteTool(ctx, "generate_dockerfile", args)
		}
		return nil, errors.Internal("core/gomcp-manager", "tool orchestrator not available")
	})
	if err := httpTransport.RegisterTool("generate_dockerfile", "Generate a Dockerfile for the analyzed repository using session-based configuration", dockerfileHandler); err != nil {
		gm.logger.Error("failed to register generate_dockerfile", "error", err)
		return err
	}
	gm.logger.Info("registered generate_dockerfile HTTP handler")

	// Register build_image handler
	buildHandler := transport.ToolHandler(func(ctx context.Context, args interface{}) (interface{}, error) {
		if gm.toolOrchestrator != nil {
			return gm.toolOrchestrator.ExecuteTool(ctx, "build_image_atomic", args)
		}
		return nil, errors.Internal("core/gomcp-manager", "tool orchestrator not available")
	})
	if err := httpTransport.RegisterTool("build_image", "Build a Docker image from the analyzed repository using generated Dockerfile and session context", buildHandler); err != nil {
		gm.logger.Error("failed to register build_image", "error", err)
		return err
	}
	gm.logger.Info("registered build_image HTTP handler")

	// Register generate_manifests handler
	manifestsHandler := transport.ToolHandler(func(ctx context.Context, args interface{}) (interface{}, error) {
		if gm.toolOrchestrator != nil {
			return gm.toolOrchestrator.ExecuteTool(ctx, "generate_manifests", args)
		}
		return nil, errors.Internal("core/gomcp-manager", "tool orchestrator not available")
	})
	if err := httpTransport.RegisterTool("generate_manifests", "Generate Kubernetes manifests for the containerized application using session-based configuration", manifestsHandler); err != nil {
		gm.logger.Error("failed to register generate_manifests", "error", err)
		return err
	}
	gm.logger.Info("registered generate_manifests HTTP handler")

	// Register scan_image handler
	scanHandler := transport.ToolHandler(func(ctx context.Context, args interface{}) (interface{}, error) {
		if gm.toolOrchestrator != nil {
			return gm.toolOrchestrator.ExecuteTool(ctx, "atomic_scan_image_security", args)
		}
		return nil, errors.Internal("core/gomcp-manager", "tool orchestrator not available")
	})
	if err := httpTransport.RegisterTool("scan_image", "Scan a Docker image for vulnerabilities using session-tracked build artifacts", scanHandler); err != nil {
		gm.logger.Error("failed to register scan_image", "error", err)
		return err
	}
	gm.logger.Info("registered scan_image HTTP handler")

	// Register list_sessions handler
	listSessionsHandler := transport.ToolHandler(func(ctx context.Context, args interface{}) (interface{}, error) {
		if gm.toolOrchestrator != nil {
			return gm.toolOrchestrator.ExecuteTool(ctx, "list_sessions", args)
		}
		return nil, errors.Internal("core/gomcp-manager", "tool orchestrator not available")
	})
	if err := httpTransport.RegisterTool("list_sessions", "List all active sessions", listSessionsHandler); err != nil {
		gm.logger.Error("failed to register list_sessions", "error", err)
		return err
	}
	gm.logger.Info("registered list_sessions HTTP handler")

	// Register get_session handler
	getSessionHandler := transport.ToolHandler(func(ctx context.Context, args interface{}) (interface{}, error) {
		if gm.toolOrchestrator != nil {
			return gm.toolOrchestrator.ExecuteTool(ctx, "get_session", args)
		}
		return nil, errors.Internal("core/gomcp-manager", "tool orchestrator not available")
	})
	if err := httpTransport.RegisterTool("get_session", "Get session details", getSessionHandler); err != nil {
		gm.logger.Error("failed to register get_session", "error", err)
		return err
	}
	gm.logger.Info("registered get_session HTTP handler")

	// Register delete_session handler
	deleteSessionHandler := transport.ToolHandler(func(ctx context.Context, args interface{}) (interface{}, error) {
		if gm.toolOrchestrator != nil {
			return gm.toolOrchestrator.ExecuteTool(ctx, "delete_session", args)
		}
		return nil, errors.Internal("core/gomcp-manager", "tool orchestrator not available")
	})
	if err := httpTransport.RegisterTool("delete_session", "Delete a session", deleteSessionHandler); err != nil {
		gm.logger.Error("failed to register delete_session", "error", err)
		return err
	}
	gm.logger.Info("registered delete_session HTTP handler")

	// Register server_status handler
	serverStatusHandler := transport.ToolHandler(func(ctx context.Context, args interface{}) (interface{}, error) {
		if gm.toolOrchestrator != nil {
			return gm.toolOrchestrator.ExecuteTool(ctx, "server_status", args)
		}
		return nil, errors.Internal("core/gomcp-manager", "tool orchestrator not available")
	})
	if err := httpTransport.RegisterTool("server_status", "Get server status", serverStatusHandler); err != nil {
		gm.logger.Error("failed to register server_status", "error", err)
		return err
	}
	gm.logger.Info("registered server_status HTTP handler")

	return nil
}
