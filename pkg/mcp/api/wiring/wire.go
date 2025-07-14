//go:build wireinject
// +build wireinject

//go:generate wire

// Package wiring provides centralized dependency injection configuration for the MCP server.
// This package wires together all layers of the application following clean architecture principles.
package wiring

import (
	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/Azure/container-kit/pkg/mcp/application"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/tracing"
	"github.com/google/wire"
)

// InitializeDefaultServer creates a fully wired MCP server with default configuration
func InitializeDefaultServer(logger *slog.Logger) (api.MCPServer, error) {
	wire.Build(ProviderSet)
	return nil, nil
}

// InitializeServerWithConfig creates a fully wired MCP server with custom configuration
func InitializeServerWithConfig(logger *slog.Logger, config workflow.ServerConfig) (api.MCPServer, error) {
	wire.Build(
		// Convert ServerConfig to Config for providers that need it
		ProvideConfigFromServerConfig,

		// Keep all other providers except configuration
		DomainProviders,
		InfraProviders,
		MLProviders,
		ApplicationProviders,

		// Still need these from CommonProviders (but not ProvideConfig)
		tracing.NewTracerAdapter,
		ProvideCommandRunner,
	)
	return nil, nil
}

// InitializeBasicServer creates a server without ML capabilities for testing
func InitializeBasicServer(logger *slog.Logger) (api.MCPServer, error) {
	wire.Build(
		CommonProviders,
		DomainProviders,
		InfraProviders,
		MLProviders,
		ApplicationProviders,
	)
	return nil, nil
}

// Testing helpers

// InitializeWorkflowOrchestrator creates just the workflow orchestrator for testing
func InitializeWorkflowOrchestrator(logger *slog.Logger) (workflow.WorkflowOrchestrator, error) {
	wire.Build(
		CommonProviders,
		DomainProviders,
		InfraProviders,
		MLProviders,
	)
	return nil, nil
}

// InitializeTestDependencies creates application dependencies for testing
func InitializeTestDependencies(logger *slog.Logger, config workflow.ServerConfig) (*application.Dependencies, error) {
	wire.Build(
		// Convert ServerConfig to Config for providers that need it
		ProvideConfigFromServerConfig,

		// Core providers needed for Dependencies struct
		DomainProviders,
		InfraProviders,
		MLProviders,

		// Application layer without server
		ProvideSessionManager,
		ProvideResourceStore,
		tracing.NewTracerAdapter,
		ProvideCommandRunner,

		// Application dependencies structure
		wire.Struct(
			new(application.Dependencies),
			"Logger", "Config", "SessionManager", "ResourceStore",
			"ProgressFactory", "EventPublisher", "SagaCoordinator",
			"WorkflowOrchestrator", "EventAwareOrchestrator", "SagaAwareOrchestrator",
			"ErrorPatternRecognizer", "EnhancedErrorHandler", "StepEnhancer",
			"SamplingClient", "PromptManager",
		),
	)
	return nil, nil
}
