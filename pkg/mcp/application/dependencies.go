// Package application provides dependency injection structures for the MCP server
package application

import (
	"fmt"
	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/Azure/container-kit/pkg/mcp/application/bootstrap"
	"github.com/Azure/container-kit/pkg/mcp/application/lifecycle"
	"github.com/Azure/container-kit/pkg/mcp/application/session"
	domainevents "github.com/Azure/container-kit/pkg/mcp/domain/events"
	domainml "github.com/Azure/container-kit/pkg/mcp/domain/ml"
	domainprompts "github.com/Azure/container-kit/pkg/mcp/domain/prompts"
	domainresources "github.com/Azure/container-kit/pkg/mcp/domain/resources"
	domainsampling "github.com/Azure/container-kit/pkg/mcp/domain/sampling"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
)

// Option represents a functional option for configuring dependencies
type Option func(*Dependencies)

func WithLogger(logger *slog.Logger) Option {
	return func(d *Dependencies) {
		d.Logger = logger
	}
}

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
	ProgressEmitterFactory workflow.ProgressEmitterFactory
	EventPublisher         domainevents.Publisher

	// Workflow orchestrators
	WorkflowOrchestrator   workflow.WorkflowOrchestrator
	EventAwareOrchestrator workflow.EventAwareOrchestrator

	// AI/ML services - using domain interfaces for clean architecture
	ErrorPatternRecognizer domainml.ErrorPatternRecognizer
	EnhancedErrorHandler   domainml.EnhancedErrorHandler
	StepEnhancer           domainml.StepEnhancer

	// Infrastructure services - using domain interfaces for clean architecture
	SamplingClient domainsampling.UnifiedSampler
	PromptManager  domainprompts.Manager
}

// Validate checks that all required dependencies are present
func (d *Dependencies) Validate() error {
	var errs []error

	// Core services validation
	if d.Logger == nil {
		errs = append(errs, fmt.Errorf("logger is required"))
	}
	if d.SessionManager == nil {
		errs = append(errs, fmt.Errorf("session manager is required"))
	}
	if d.ResourceStore == nil {
		errs = append(errs, fmt.Errorf("resource store is required"))
	}

	// Domain services validation
	if d.ProgressEmitterFactory == nil {
		errs = append(errs, fmt.Errorf("progress emitter factory is required"))
	}
	if d.EventPublisher == nil {
		errs = append(errs, fmt.Errorf("event publisher is required"))
	}

	// Workflow orchestrators validation
	if d.WorkflowOrchestrator == nil {
		errs = append(errs, fmt.Errorf("workflow orchestrator is required"))
	}

	// Infrastructure services validation
	if d.SamplingClient == nil {
		errs = append(errs, fmt.Errorf("sampling client is required"))
	}
	if d.PromptManager == nil {
		errs = append(errs, fmt.Errorf("prompt manager is required"))
	}

	if len(errs) > 0 {
		return fmt.Errorf("dependency validation failed: %v", errs)
	}
	return nil
}

// NewMCPServerFromDeps creates a new MCP server that implements api.MCPServer.
// This is used by Wire for dependency injection.
func NewMCPServerFromDeps(deps *Dependencies) (api.MCPServer, error) {
	// Validate dependencies first
	if err := deps.Validate(); err != nil {
		return nil, fmt.Errorf("invalid dependencies: %w", err)
	}

	// Create the bootstrapper component
	bootstrapper := bootstrap.NewBootstrapper(
		deps.Logger,
		deps.Config,
		deps.ResourceStore,
		deps.WorkflowOrchestrator,
	)

	// Create the lifecycle manager component
	lifecycleManager := lifecycle.NewLifecycleManager(
		deps.Logger,
		deps.Config,
		deps.SessionManager,
		bootstrapper,
	)

	return &serverImpl{
		dependencies:     deps,
		lifecycleManager: lifecycleManager,
		bootstrapper:     bootstrapper,
	}, nil
}

// GetChatModeFunctions returns the function names available in chat mode
func GetChatModeFunctions() []string {
	return []string{
		"containerize_and_deploy",
	}
}
