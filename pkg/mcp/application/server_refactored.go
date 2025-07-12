package application

import (
	"context"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/common/errors"
	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/Azure/container-kit/pkg/mcp/application/config"
	"github.com/Azure/container-kit/pkg/mcp/application/lifecycle"
	"github.com/Azure/container-kit/pkg/mcp/application/monitoring"
	"github.com/Azure/container-kit/pkg/mcp/application/registry"
	"github.com/Azure/container-kit/pkg/mcp/application/session"
	"github.com/Azure/container-kit/pkg/mcp/application/transport"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/resources"
)

// RefactoredServer represents the refactored MCP server with separated concerns
type RefactoredServer struct {
	// Core components
	transport        transport.MCPTransport
	configManager    config.Manager
	lifecycleManager lifecycle.Manager
	monitor          monitoring.Monitor
	registry         *registry.Registry

	// Domain services
	sessionManager session.SessionManager
	resourceStore  *resources.Store

	// Infrastructure
	logger *slog.Logger
}

// NewRefactoredServer creates a new server with properly separated concerns
func NewRefactoredServer(opts ...ServerOption) (api.MCPServer, error) {
	// Apply options to get configuration
	options := &serverOptions{
		config: workflow.DefaultServerConfig(),
	}
	for _, opt := range opts {
		opt(options)
	}

	logger := options.logger
	if logger == nil {
		logger = slog.Default()
	}
	logger = logger.With("component", "mcp_server")

	// Create configuration manager
	configManager := config.NewManager(options.config, logger)
	if err := configManager.ValidateConfig(); err != nil {
		return nil, errors.New(errors.CodeValidationFailed, "config", "invalid configuration", err)
	}

	// Ensure directories exist
	if err := configManager.EnsureDirectories(); err != nil {
		return nil, errors.New(errors.CodeInternalError, "config", "failed to create directories", err)
	}

	cfg := configManager.GetConfig()

	// Create domain services
	sessionManager := session.NewMemorySessionManager(logger, cfg.SessionTTL, cfg.MaxSessions)

	resourceStore := resources.NewStore(logger)

	// Create infrastructure components
	mcpTransport := transport.NewStdioTransport("container-kit-mcp", "1.0.0", logger)
	lifecycleManager := lifecycle.NewManager(logger)
	monitor := monitoring.NewMonitor("1.0.0", cfg.TransportType, logger)
	toolRegistry := registry.NewRegistry(logger)

	// Wire up monitoring
	monitor.SetSessionManager(sessionManager)
	monitor.SetLifecycleManager(lifecycleManager)
	monitor.SetResourceStore(resourceStore)

	// Create server
	server := &RefactoredServer{
		transport:        mcpTransport,
		configManager:    configManager,
		lifecycleManager: lifecycleManager,
		monitor:          monitor,
		registry:         toolRegistry,
		sessionManager:   sessionManager,
		resourceStore:    resourceStore,
		logger:           logger,
	}

	// Setup lifecycle hooks
	server.setupLifecycleHooks()

	// Initialize lifecycle
	if err := lifecycleManager.Initialize(); err != nil {
		return nil, errors.New(errors.CodeInternalError, "lifecycle", "failed to initialize", err)
	}

	logger.Info("Refactored MCP server created successfully")
	return server, nil
}

// setupLifecycleHooks configures lifecycle hooks
func (s *RefactoredServer) setupLifecycleHooks() {
	// On start hooks
	s.lifecycleManager.OnStart(func(ctx context.Context) error {
		// Start session cleanup
		return s.sessionManager.StartCleanupRoutine(ctx)
	})

	s.lifecycleManager.OnStart(func(ctx context.Context) error {
		// Start resource cleanup
		s.resourceStore.StartCleanupRoutine(30*time.Minute, 24*time.Hour)
		return nil
	})

	// Transport initialization happens in Start

	s.lifecycleManager.OnStart(func(ctx context.Context) error {
		// Register all components
		return s.registry.RegisterAll(ctx, s.transport, s.resourceStore)
	})

	s.lifecycleManager.OnStart(func(ctx context.Context) error {
		// Register diagnostic tools
		return s.monitor.RegisterDiagnosticTools(s.transport)
	})

	// On stop hooks
	s.lifecycleManager.OnStop(func(ctx context.Context) error {
		// Stop transport
		return s.transport.Stop(ctx)
	})

	s.lifecycleManager.OnStop(func(ctx context.Context) error {
		// Stop session manager
		return s.sessionManager.Stop(ctx)
	})

	s.lifecycleManager.OnStop(func(ctx context.Context) error {
		// Stop resource store
		s.resourceStore.StopCleanupRoutine()
		return nil
	})
}

// Start starts the MCP server
func (s *RefactoredServer) Start(ctx context.Context) error {
	s.logger.Info("Starting refactored MCP server")

	// Start lifecycle
	if err := s.lifecycleManager.Start(ctx); err != nil {
		return errors.New(errors.CodeInternalError, "lifecycle", "failed to start", err)
	}

	// Start transport in a goroutine
	transportDone := make(chan error, 1)
	go func() {
		transportDone <- s.transport.Start(ctx)
	}()

	// Wait for context cancellation or transport completion
	select {
	case <-ctx.Done():
		s.logger.Info("Server stopped by context cancellation")
		return s.Stop(context.Background()) // Use new context for cleanup
	case err := <-transportDone:
		if err != nil {
			s.logger.Error("Transport stopped with error", "error", err)
			return errors.New(errors.CodeInternalError, "transport", "transport failed", err)
		}
		return nil
	}
}

// Stop gracefully stops the server
func (s *RefactoredServer) Stop(ctx context.Context) error {
	s.logger.Info("Stopping refactored MCP server")
	return s.lifecycleManager.Stop(ctx)
}

// Shutdown is an alias for Stop for interface compatibility
func (s *RefactoredServer) Shutdown(ctx context.Context) error {
	return s.Stop(ctx)
}

// GetStats returns server statistics
func (s *RefactoredServer) GetStats() (interface{}, error) {
	return s.monitor.GetStats()
}

// RegisterChatModes registers chat modes (placeholder for compatibility)
func (s *RefactoredServer) RegisterChatModes() error {
	s.logger.Debug("Chat modes not supported in refactored server")
	return nil
}

// ConversationComponents returns conversation components (placeholder)
func (s *RefactoredServer) ConversationComponents() *ConversationComponents {
	return &ConversationComponents{}
}

// GetSessionManager returns the session manager
func (s *RefactoredServer) GetSessionManager() session.SessionManager {
	return s.sessionManager
}

// GetSessionManagerStats returns session manager statistics
func (s *RefactoredServer) GetSessionManagerStats() (interface{}, error) {
	return s.sessionManager.GetStats()
}

// serverOptions holds server configuration options
type serverOptions struct {
	config workflow.ServerConfig
	logger *slog.Logger
}

// ServerOption is a function that modifies server options
type ServerOption func(*serverOptions)

// WithConfig sets the server configuration
func WithConfig(config workflow.ServerConfig) ServerOption {
	return func(o *serverOptions) {
		o.config = config
	}
}

// WithLogger sets the logger
func WithLogger(logger *slog.Logger) ServerOption {
	return func(o *serverOptions) {
		o.logger = logger
	}
}

// WithWorkspaceDir sets the workspace directory
func WithWorkspaceDir(dir string) ServerOption {
	return func(o *serverOptions) {
		o.config.WorkspaceDir = dir
	}
}

// WithStoragePath sets the storage path
func WithStoragePath(path string) ServerOption {
	return func(o *serverOptions) {
		o.config.StorePath = path
	}
}

// WithServerMaxSessions sets the maximum number of sessions
func WithServerMaxSessions(max int) ServerOption {
	return func(o *serverOptions) {
		o.config.MaxSessions = max
	}
}

// Builder provides a fluent interface for building a server
type Builder struct {
	options []ServerOption
}

// NewBuilder creates a new server builder
func NewBuilder() *Builder {
	return &Builder{}
}

// WithOption adds an option to the builder
func (b *Builder) WithOption(opt ServerOption) *Builder {
	b.options = append(b.options, opt)
	return b
}

// WithConfig sets the configuration
func (b *Builder) WithConfig(config workflow.ServerConfig) *Builder {
	return b.WithOption(WithConfig(config))
}

// WithLogger sets the logger
func (b *Builder) WithLogger(logger *slog.Logger) *Builder {
	return b.WithOption(WithLogger(logger))
}

// Build builds the server
func (b *Builder) Build() (api.MCPServer, error) {
	return NewRefactoredServer(b.options...)
}
