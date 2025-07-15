// Package application provides examples of how to use the new grouped dependency structure
package application

import (
	"context"
	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/application/registrar"
)

// ExampleUsageOfGroupedDependencies demonstrates how to use the new grouped dependencies
// This shows the migration path from the flat Dependencies struct to the grouped approach
func ExampleUsageOfGroupedDependencies(s *serverImpl) {
	// Convert existing dependencies to grouped structure
	grouped := s.deps.GetGroupedDependencies()

	// Access dependencies by logical groups
	logger := grouped.Core.Logger
	config := grouped.Core.Config

	// Use workflow dependencies together
	orchestrator := grouped.Workflow.Orchestrator
	eventPublisher := grouped.Workflow.EventPublisher
	progressFactory := grouped.Workflow.ProgressEmitterFactory

	// Use AI dependencies together
	sampler := grouped.AI.SamplingClient
	promptManager := grouped.AI.PromptManager
	errorAnalyzer := grouped.AI.ErrorPatternRecognizer

	// Use persistence dependencies together
	sessionManager := grouped.Persistence.SessionManager
	resourceStore := grouped.Persistence.ResourceStore

	// Example: Creating a registrar with grouped dependencies
	// Before: required accessing individual fields from flat Dependencies
	// After: can easily group related dependencies
	_ = registrar.NewRegistrar(logger, resourceStore, orchestrator)

	// Example: Workflow operations using grouped dependencies
	ctx := context.Background()

	// Progress reporting setup using workflow group
	// Note: Actual CallToolRequest creation should be done with proper MCP client types
	_ = ctx
	_ = progressFactory

	// Event publishing using workflow group
	_ = eventPublisher

	// Session management using persistence group
	_, _ = sessionManager.GetOrCreate(ctx, "example-session")

	// AI operations using AI group
	_ = sampler
	_ = promptManager
	_ = errorAnalyzer

	logger.Info("Grouped dependencies example completed",
		"config_workspace", config.WorkspaceDir,
		"groups", "core, workflow, persistence, ai")
}

// ExampleNewServiceWithGroupedDeps shows how new services can accept grouped dependencies
func ExampleNewServiceWithGroupedDeps(grouped *GroupedDependencies) *ExampleService {
	return &ExampleService{
		logger:       grouped.Core.Logger,
		config:       grouped.Core.Config,
		orchestrator: grouped.Workflow.Orchestrator,
		sampler:      grouped.AI.SamplingClient,
		sessions:     grouped.Persistence.SessionManager,
	}
}

// ExampleService demonstrates a service that uses grouped dependencies
type ExampleService struct {
	logger       *slog.Logger
	config       interface{} // workflow.ServerConfig
	orchestrator interface{} // workflow.WorkflowOrchestrator
	sampler      interface{} // domainsampling.UnifiedSampler
	sessions     interface{} // session.OptimizedSessionManager
}

// ProcessWorkflow shows how the service would use its grouped dependencies
func (e *ExampleService) ProcessWorkflow(ctx context.Context, workflowID string) error {
	e.logger.Info("Processing workflow with grouped dependencies",
		"workflow_id", workflowID)

	// Use orchestrator for workflow execution
	// Use sampler for AI operations
	// Use sessions for state management

	return nil
}
