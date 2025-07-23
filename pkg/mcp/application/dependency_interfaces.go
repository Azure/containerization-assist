// Package application provides interface capsules for dependency management
package application

import (
	"log/slog"

	"github.com/Azure/container-kit/pkg/common/runner"
	"github.com/Azure/container-kit/pkg/mcp/application/session"
	domainevents "github.com/Azure/container-kit/pkg/mcp/domain/events"
	domainml "github.com/Azure/container-kit/pkg/mcp/domain/ml"
	domainprompts "github.com/Azure/container-kit/pkg/mcp/domain/prompts"
	domainresources "github.com/Azure/container-kit/pkg/mcp/domain/resources"
	domainsampling "github.com/Azure/container-kit/pkg/mcp/domain/sampling"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
)

// CoreServices provides fundamental system operations
// Most frequently used together for initialization and operational logging
type CoreServices interface {
	Logger() *slog.Logger
	Config() workflow.ServerConfig
	Runner() runner.CommandRunner
}

// PersistenceServices provides data persistence operations
// Used together for session and resource management
type PersistenceServices interface {
	SessionManager() session.OptimizedSessionManager
	ResourceStore() domainresources.Store
}

// WorkflowServices provides workflow orchestration operations
// Used together for workflow coordination and progress tracking
type WorkflowServices interface {
	Orchestrator() workflow.WorkflowOrchestrator
	EventAwareOrchestrator() workflow.EventAwareOrchestrator
	EventPublisher() domainevents.Publisher
	ProgressFactory() workflow.ProgressEmitterFactory
}

// AIServices provides AI/ML operations
// Used together for intelligent error handling and step enhancement
type AIServices interface {
	ErrorRecognizer() domainml.ErrorPatternRecognizer
	ErrorHandler() domainml.EnhancedErrorHandler
	StepEnhancer() domainml.StepEnhancer
	SamplingClient() domainsampling.UnifiedSampler
	PromptManager() domainprompts.Manager
}

// AllServices aggregates all service interfaces for full system access
// Replaces the monolithic Dependencies struct
type AllServices interface {
	CoreServices
	PersistenceServices
	WorkflowServices
	AIServices
}

// Implementation of interface capsules using GroupedDependencies
type serviceProvider struct {
	grouped *GroupedDependencies
}

// NewServiceProvider creates interface capsules from GroupedDependencies
func NewServiceProvider(grouped *GroupedDependencies) AllServices {
	return &serviceProvider{grouped: grouped}
}

// CoreServices implementation
func (s *serviceProvider) Logger() *slog.Logger {
	return s.grouped.Core.Logger
}

func (s *serviceProvider) Config() workflow.ServerConfig {
	return s.grouped.Core.Config
}

func (s *serviceProvider) Runner() runner.CommandRunner {
	return s.grouped.Core.Runner
}

// PersistenceServices implementation
func (s *serviceProvider) SessionManager() session.OptimizedSessionManager {
	return s.grouped.Persistence.SessionManager
}

func (s *serviceProvider) ResourceStore() domainresources.Store {
	return s.grouped.Persistence.ResourceStore
}

// WorkflowServices implementation
func (s *serviceProvider) Orchestrator() workflow.WorkflowOrchestrator {
	return s.grouped.Workflow.Orchestrator
}

func (s *serviceProvider) EventAwareOrchestrator() workflow.EventAwareOrchestrator {
	return s.grouped.Workflow.EventAwareOrchestrator
}

func (s *serviceProvider) EventPublisher() domainevents.Publisher {
	return s.grouped.Workflow.EventPublisher
}

func (s *serviceProvider) ProgressFactory() workflow.ProgressEmitterFactory {
	return s.grouped.Workflow.ProgressEmitterFactory
}

// AIServices implementation
func (s *serviceProvider) ErrorRecognizer() domainml.ErrorPatternRecognizer {
	return s.grouped.AI.ErrorPatternRecognizer
}

func (s *serviceProvider) ErrorHandler() domainml.EnhancedErrorHandler {
	return s.grouped.AI.EnhancedErrorHandler
}

func (s *serviceProvider) StepEnhancer() domainml.StepEnhancer {
	return s.grouped.AI.StepEnhancer
}

func (s *serviceProvider) SamplingClient() domainsampling.UnifiedSampler {
	return s.grouped.AI.SamplingClient
}

func (s *serviceProvider) PromptManager() domainprompts.Manager {
	return s.grouped.AI.PromptManager
}

// Helper functions for easier migration

// CoreServicesFrom extracts core services from existing Dependencies
func CoreServicesFrom(deps *Dependencies) CoreServices {
	grouped := FromLegacyDependencies(deps)
	return NewServiceProvider(grouped)
}

// PersistenceServicesFrom extracts persistence services from existing Dependencies
func PersistenceServicesFrom(deps *Dependencies) PersistenceServices {
	grouped := FromLegacyDependencies(deps)
	return NewServiceProvider(grouped)
}

// WorkflowServicesFrom extracts workflow services from existing Dependencies
func WorkflowServicesFrom(deps *Dependencies) WorkflowServices {
	grouped := FromLegacyDependencies(deps)
	return NewServiceProvider(grouped)
}

// AIServicesFrom extracts AI services from existing Dependencies
func AIServicesFrom(deps *Dependencies) AIServices {
	grouped := FromLegacyDependencies(deps)
	return NewServiceProvider(grouped)
}
