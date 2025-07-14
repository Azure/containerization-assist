// Package application provides dependency injection structures for the MCP server
package application

import (
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/Azure/container-kit/pkg/mcp/application/session"
	domainevents "github.com/Azure/container-kit/pkg/mcp/domain/events"
	domainml "github.com/Azure/container-kit/pkg/mcp/domain/ml"
	domainprompts "github.com/Azure/container-kit/pkg/mcp/domain/prompts"
	domainresources "github.com/Azure/container-kit/pkg/mcp/domain/resources"
	domainsampling "github.com/Azure/container-kit/pkg/mcp/domain/sampling"
	"github.com/Azure/container-kit/pkg/mcp/domain/saga"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
)

// Option represents a functional option for configuring dependencies
type Option func(*Dependencies)

// WithLogger sets a custom logger
func WithLogger(logger *slog.Logger) Option {
	return func(d *Dependencies) {
		d.Logger = logger
	}
}

// WithConfig sets the server configuration
func WithConfig(config workflow.ServerConfig) Option {
	return func(d *Dependencies) {
		d.Config = config
	}
}

// Dependencies holds all the server dependencies in a structured way.
type Dependencies struct {
	// Core services
	Logger         *slog.Logger
	Config         workflow.ServerConfig
	SessionManager session.OptimizedSessionManager
	ResourceStore  domainresources.Store

	// Domain services
	ProgressFactory workflow.ProgressTrackerFactory
	EventPublisher  domainevents.Publisher
	SagaCoordinator *saga.SagaCoordinator

	// Workflow orchestrators
	WorkflowOrchestrator   workflow.WorkflowOrchestrator
	EventAwareOrchestrator workflow.EventAwareOrchestrator
	SagaAwareOrchestrator  workflow.SagaAwareOrchestrator

	// AI/ML services - using domain interfaces for clean architecture
	ErrorPatternRecognizer domainml.ErrorPatternRecognizer
	EnhancedErrorHandler   domainml.EnhancedErrorHandler
	StepEnhancer           domainml.StepEnhancer

	// Infrastructure services - using domain interfaces for clean architecture
	SamplingClient domainsampling.UnifiedSampler
	PromptManager  domainprompts.Manager
}

// NewMCPServerFromDeps creates a new MCP server that implements api.MCPServer.
// This is used by Wire for dependency injection.
func NewMCPServerFromDeps(deps *Dependencies) api.MCPServer {
	return &serverImpl{
		deps:      deps,
		startTime: time.Now(),
	}
}

// GetChatModeFunctions returns the function names available in chat mode
func GetChatModeFunctions() []string {
	return []string{
		"containerize_and_deploy",
	}
}