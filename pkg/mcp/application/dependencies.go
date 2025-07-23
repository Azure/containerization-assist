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

	// Convert legacy Dependencies to interface capsules
	services := NewServiceProvider(FromLegacyDependencies(deps))

	// Create the bootstrapper component
	bootstrapper := bootstrap.NewBootstrapper(
		services.Logger(),
		services.Config(),
		services.ResourceStore(),
		services.Orchestrator(),
	)

	// Create the lifecycle manager component
	lifecycleManager := lifecycle.NewLifecycleManager(
		services.Logger(),
		services.Config(),
		services.SessionManager(),
		bootstrapper,
	)

	return &serverImpl{
		services:         services,
		lifecycleManager: lifecycleManager,
		bootstrapper:     bootstrapper,
	}, nil
}

// NewMCPServerFromServices creates a new MCP server using interface capsules.
// This is the preferred constructor for new code.
func NewMCPServerFromServices(services AllServices) api.MCPServer {
	// Create the bootstrapper component
	bootstrapper := bootstrap.NewBootstrapper(
		services.Logger(),
		services.Config(),
		services.ResourceStore(),
		services.Orchestrator(),
	)

	// Create the lifecycle manager component
	lifecycleManager := lifecycle.NewLifecycleManager(
		services.Logger(),
		services.Config(),
		services.SessionManager(),
		bootstrapper,
	)

	return &serverImpl{
		services:         services,
		lifecycleManager: lifecycleManager,
		bootstrapper:     bootstrapper,
	}
}

// GetGroupedDependencies returns a grouped view of the dependencies
// This provides a migration path to the new structured approach
func (d *Dependencies) GetGroupedDependencies() *GroupedDependencies {
	return FromLegacyDependencies(d)
}

// GetChatModeFunctions returns the function names available in chat mode
func GetChatModeFunctions() []string {
	return []string{
		"containerize_and_deploy",
	}
}
