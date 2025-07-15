// Package application provides grouped dependency structures for better organization
package application

import (
	"log/slog"

	"github.com/Azure/container-kit/pkg/common/runner"
	"github.com/Azure/container-kit/pkg/mcp/application/session"
	domainevents "github.com/Azure/container-kit/pkg/mcp/domain/events"
	domainml "github.com/Azure/container-kit/pkg/mcp/domain/ml"
	domainprompts "github.com/Azure/container-kit/pkg/mcp/domain/prompts"
	domainresources "github.com/Azure/container-kit/pkg/mcp/domain/resources"
	"github.com/Azure/container-kit/pkg/mcp/domain/saga"
	domainsampling "github.com/Azure/container-kit/pkg/mcp/domain/sampling"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
)

// CoreDeps holds fundamental system dependencies
type CoreDeps struct {
	Logger *slog.Logger
	Config workflow.ServerConfig
	Runner runner.CommandRunner
}

// WorkflowDeps holds workflow orchestration dependencies
type WorkflowDeps struct {
	Orchestrator           workflow.WorkflowOrchestrator
	EventAwareOrchestrator workflow.EventAwareOrchestrator
	SagaAwareOrchestrator  workflow.SagaAwareOrchestrator
	EventPublisher         domainevents.Publisher
	ProgressEmitterFactory workflow.ProgressEmitterFactory
	SagaCoordinator        *saga.SagaCoordinator
}

// PersistenceDeps holds data persistence dependencies
type PersistenceDeps struct {
	SessionManager session.OptimizedSessionManager
	ResourceStore  domainresources.Store
}

// AIDeps holds AI/ML service dependencies
type AIDeps struct {
	SamplingClient         domainsampling.UnifiedSampler
	PromptManager          domainprompts.Manager
	ErrorPatternRecognizer domainml.ErrorPatternRecognizer
	EnhancedErrorHandler   domainml.EnhancedErrorHandler
	StepEnhancer           domainml.StepEnhancer
}

// GroupedDependencies provides a more organized structure for dependencies
type GroupedDependencies struct {
	Core        CoreDeps
	Workflow    WorkflowDeps
	Persistence PersistenceDeps
	AI          AIDeps
}

// ToLegacyDependencies converts GroupedDependencies to the current Dependencies struct
// This provides backward compatibility during the transition
func (gd *GroupedDependencies) ToLegacyDependencies() *Dependencies {
	return &Dependencies{
		// Core services
		Logger:         gd.Core.Logger,
		Config:         gd.Core.Config,
		SessionManager: gd.Persistence.SessionManager,
		ResourceStore:  gd.Persistence.ResourceStore,

		// Domain services
		ProgressEmitterFactory: gd.Workflow.ProgressEmitterFactory,
		EventPublisher:         gd.Workflow.EventPublisher,
		SagaCoordinator:        gd.Workflow.SagaCoordinator,

		// Workflow orchestrators
		WorkflowOrchestrator:   gd.Workflow.Orchestrator,
		EventAwareOrchestrator: gd.Workflow.EventAwareOrchestrator,
		SagaAwareOrchestrator:  gd.Workflow.SagaAwareOrchestrator,

		// AI/ML services
		ErrorPatternRecognizer: gd.AI.ErrorPatternRecognizer,
		EnhancedErrorHandler:   gd.AI.EnhancedErrorHandler,
		StepEnhancer:           gd.AI.StepEnhancer,

		// Infrastructure services
		SamplingClient: gd.AI.SamplingClient,
		PromptManager:  gd.AI.PromptManager,
	}
}

// FromLegacyDependencies creates GroupedDependencies from the current Dependencies struct
// This helps migrate existing code to the new structure
// Note: CommandRunner is not available in legacy Dependencies, so it will be nil
func FromLegacyDependencies(deps *Dependencies) *GroupedDependencies {
	return &GroupedDependencies{
		Core: CoreDeps{
			Logger: deps.Logger,
			Config: deps.Config,
			Runner: nil, // Legacy Dependencies doesn't include CommandRunner
		},
		Workflow: WorkflowDeps{
			Orchestrator:           deps.WorkflowOrchestrator,
			EventAwareOrchestrator: deps.EventAwareOrchestrator,
			SagaAwareOrchestrator:  deps.SagaAwareOrchestrator,
			EventPublisher:         deps.EventPublisher,
			ProgressEmitterFactory: deps.ProgressEmitterFactory,
			SagaCoordinator:        deps.SagaCoordinator,
		},
		Persistence: PersistenceDeps{
			SessionManager: deps.SessionManager,
			ResourceStore:  deps.ResourceStore,
		},
		AI: AIDeps{
			SamplingClient:         deps.SamplingClient,
			PromptManager:          deps.PromptManager,
			ErrorPatternRecognizer: deps.ErrorPatternRecognizer,
			EnhancedErrorHandler:   deps.EnhancedErrorHandler,
			StepEnhancer:           deps.StepEnhancer,
		},
	}
}
