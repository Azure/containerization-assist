// Package application provides dependency injection for MCP server components.
package application

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/session"
	"github.com/Azure/container-kit/pkg/mcp/domain/progress"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/prompts"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/resources"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/sampling"
)

// Dependencies holds all the server dependencies in a structured way.
type Dependencies struct {
	// Core services
	Logger         *slog.Logger
	Config         workflow.ServerConfig
	SessionManager session.SessionManager
	ResourceStore  *resources.Store

	// Domain services
	ProgressFactory *progress.SinkFactory

	// Infrastructure services
	SamplingClient *sampling.Client
	PromptManager  *prompts.Manager
}

// NewDependencies creates and wires up all server dependencies.
func NewDependencies(config workflow.ServerConfig, logger *slog.Logger) *Dependencies {
	// Core logger with component tagging
	baseLogger := logger.With("component", "mcp-server")

	// Create session manager
	sessionManager := session.NewMemorySessionManager(
		baseLogger.With("service", "session"),
		config.SessionTTL,
		config.MaxSessions,
	)

	// Create resource store
	resourceStore := resources.NewStore(baseLogger.With("service", "resources"))

	// Create progress factory
	progressFactory := progress.NewSinkFactory(baseLogger.With("service", "progress"))

	// Create sampling client with intelligent defaults
	samplingClient := sampling.NewClient(
		baseLogger.With("service", "sampling"),
		sampling.WithMaxTokens(4000),
		sampling.WithTemperature(0.1), // Conservative for infrastructure tasks
		sampling.WithRetry(3, 10000),  // 3 attempts, 10k token budget
	)

	// Create prompt manager
	promptManager, err := prompts.NewManager(
		baseLogger.With("service", "prompts"),
		prompts.ManagerConfig{
			EnableHotReload: false, // Disable in production
			AllowOverride:   false, // Use embedded templates only
		},
	)
	if err != nil {
		baseLogger.Error("Failed to create prompt manager", "error", err)
		// Continue without error - prompt manager is not critical for basic operation
		promptManager = nil
	}

	return &Dependencies{
		Logger:          baseLogger,
		Config:          config,
		SessionManager:  sessionManager,
		ResourceStore:   resourceStore,
		ProgressFactory: progressFactory,
		SamplingClient:  samplingClient,
		PromptManager:   promptManager,
	}
}

// ServerBuilder provides a fluent interface for building servers with dependencies.
type ServerBuilder struct {
	deps *Dependencies
}

// NewServerBuilder creates a new server builder.
func NewServerBuilder() *ServerBuilder {
	return &ServerBuilder{}
}

// WithConfig sets the server configuration.
func (b *ServerBuilder) WithConfig(config workflow.ServerConfig) *ServerBuilder {
	if b.deps == nil {
		b.deps = &Dependencies{Config: config}
	} else {
		b.deps.Config = config
	}
	return b
}

// WithLogger sets the logger.
func (b *ServerBuilder) WithLogger(logger *slog.Logger) *ServerBuilder {
	if b.deps == nil {
		b.deps = &Dependencies{Logger: logger}
	} else {
		b.deps.Logger = logger
	}
	return b
}

// WithSessionManager sets a custom session manager.
func (b *ServerBuilder) WithSessionManager(sm session.SessionManager) *ServerBuilder {
	if b.deps == nil {
		b.deps = &Dependencies{}
	}
	b.deps.SessionManager = sm
	return b
}

// WithSamplingClient sets a custom sampling client.
func (b *ServerBuilder) WithSamplingClient(client *sampling.Client) *ServerBuilder {
	if b.deps == nil {
		b.deps = &Dependencies{}
	}
	b.deps.SamplingClient = client
	return b
}

// Build creates the server with all dependencies properly wired.
func (b *ServerBuilder) Build() (*serverImpl, error) {
	// Ensure we have dependencies
	if b.deps == nil {
		return nil, fmt.Errorf("no dependencies configured")
	}

	// Fill in missing dependencies with defaults
	if b.deps.Logger == nil {
		b.deps.Logger = slog.Default()
	}
	if b.deps.Config.WorkspaceDir == "" && b.deps.Config.StorePath == "" {
		b.deps.Config = workflow.DefaultServerConfig()
	}

	// Create full dependency graph if not already done
	fullDeps := NewDependencies(b.deps.Config, b.deps.Logger)

	// Override with any custom dependencies provided
	if b.deps.SessionManager != nil {
		fullDeps.SessionManager = b.deps.SessionManager
	}
	if b.deps.SamplingClient != nil {
		fullDeps.SamplingClient = b.deps.SamplingClient
	}
	if b.deps.ResourceStore != nil {
		fullDeps.ResourceStore = b.deps.ResourceStore
	}
	if b.deps.PromptManager != nil {
		fullDeps.PromptManager = b.deps.PromptManager
	}
	if b.deps.ProgressFactory != nil {
		fullDeps.ProgressFactory = b.deps.ProgressFactory
	}

	// Create server with dependencies
	server := &serverImpl{
		deps:      fullDeps,
		startTime: time.Now(),
	}

	return server, nil
}
