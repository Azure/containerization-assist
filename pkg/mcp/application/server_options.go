// Package application provides server configuration options using the options pattern
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

// ServerOption represents a functional option for configuring the MCP server
type ServerOption func(*serverConfig)

// serverConfig holds the configuration for the server
type serverConfig struct {
	core        CoreServices
	persistence PersistenceServices
	workflow    WorkflowServices
	ai          AIServices
	llmConfig   *LLMConfig
}

// WithCoreServices sets the core services (Logger, Config, Runner)
func WithCoreServices(core CoreServices) ServerOption {
	return func(cfg *serverConfig) {
		cfg.core = core
	}
}

// WithPersistenceServices sets the persistence services (SessionManager, ResourceStore)
func WithPersistenceServices(persistence PersistenceServices) ServerOption {
	return func(cfg *serverConfig) {
		cfg.persistence = persistence
	}
}

// WithWorkflowServices sets the workflow services (Orchestrators, Events, Progress)
func WithWorkflowServices(workflow WorkflowServices) ServerOption {
	return func(cfg *serverConfig) {
		cfg.workflow = workflow
	}
}

// WithAIServices sets the AI/ML services (Error handling, Sampling, Prompts)
func WithAIServices(ai AIServices) ServerOption {
	return func(cfg *serverConfig) {
		cfg.ai = ai
	}
}

// Individual service options for fine-grained control

// WithLogger sets a custom logger
func WithLoggerService(logger *slog.Logger) ServerOption {
	return func(cfg *serverConfig) {
		if cfg.core != nil {
			// If core services already exist, we need to wrap them
			cfg.core = &coreServicesWrapper{
				logger: logger,
				config: cfg.core.Config(),
				runner: cfg.core.Runner(),
			}
		}
	}
}

// WithConfigService sets the server configuration
func WithConfigService(config workflow.ServerConfig) ServerOption {
	return func(cfg *serverConfig) {
		if cfg.core != nil {
			cfg.core = &coreServicesWrapper{
				logger: cfg.core.Logger(),
				config: config,
				runner: cfg.core.Runner(),
			}
		}
	}
}

// WithSessionManager sets a custom session manager
func WithSessionManagerService(sessionManager session.OptimizedSessionManager) ServerOption {
	return func(cfg *serverConfig) {
		if cfg.persistence != nil {
			cfg.persistence = &persistenceServicesWrapper{
				sessionManager: sessionManager,
				resourceStore:  cfg.persistence.ResourceStore(),
			}
		}
	}
}

// NewServerWithOptions creates a new MCP server using the options pattern
func NewServerWithOptions(opts ...ServerOption) AllServices {
	cfg := &serverConfig{}

	for _, opt := range opts {
		opt(cfg)
	}

	// Create grouped dependencies from the configuration
	grouped := &GroupedDependencies{}

	if cfg.core != nil {
		grouped.Core = CoreDeps{
			Logger: cfg.core.Logger(),
			Config: cfg.core.Config(),
			Runner: cfg.core.Runner(),
		}
	}

	if cfg.persistence != nil {
		grouped.Persistence = PersistenceDeps{
			SessionManager: cfg.persistence.SessionManager(),
			ResourceStore:  cfg.persistence.ResourceStore(),
		}
	}

	if cfg.workflow != nil {
		grouped.Workflow = WorkflowDeps{
			Orchestrator:           cfg.workflow.Orchestrator(),
			EventAwareOrchestrator: cfg.workflow.EventAwareOrchestrator(),
			EventPublisher:         cfg.workflow.EventPublisher(),
			ProgressEmitterFactory: cfg.workflow.ProgressFactory(),
		}
	}

	if cfg.ai != nil {
		grouped.AI = AIDeps{
			SamplingClient:         cfg.ai.SamplingClient(),
			PromptManager:          cfg.ai.PromptManager(),
			ErrorPatternRecognizer: cfg.ai.ErrorRecognizer(),
			EnhancedErrorHandler:   cfg.ai.ErrorHandler(),
			StepEnhancer:           cfg.ai.StepEnhancer(),
		}
	}

	return NewServiceProvider(grouped)
}

// Wrapper implementations for individual service updates

type coreServicesWrapper struct {
	logger *slog.Logger
	config workflow.ServerConfig
	runner runner.CommandRunner
}

func (w *coreServicesWrapper) Logger() *slog.Logger          { return w.logger }
func (w *coreServicesWrapper) Config() workflow.ServerConfig { return w.config }
func (w *coreServicesWrapper) Runner() runner.CommandRunner  { return w.runner }

type persistenceServicesWrapper struct {
	sessionManager session.OptimizedSessionManager
	resourceStore  domainresources.Store
}

func (w *persistenceServicesWrapper) SessionManager() session.OptimizedSessionManager {
	return w.sessionManager
}
func (w *persistenceServicesWrapper) ResourceStore() domainresources.Store {
	return w.resourceStore
}

type workflowServicesWrapper struct {
	orchestrator           workflow.WorkflowOrchestrator
	eventAwareOrchestrator workflow.EventAwareOrchestrator
	eventPublisher         domainevents.Publisher
	progressFactory        workflow.ProgressEmitterFactory
}

func (w *workflowServicesWrapper) Orchestrator() workflow.WorkflowOrchestrator {
	return w.orchestrator
}
func (w *workflowServicesWrapper) EventAwareOrchestrator() workflow.EventAwareOrchestrator {
	return w.eventAwareOrchestrator
}
func (w *workflowServicesWrapper) EventPublisher() domainevents.Publisher {
	return w.eventPublisher
}
func (w *workflowServicesWrapper) ProgressFactory() workflow.ProgressEmitterFactory {
	return w.progressFactory
}

type aiServicesWrapper struct {
	errorRecognizer domainml.ErrorPatternRecognizer
	errorHandler    domainml.EnhancedErrorHandler
	stepEnhancer    domainml.StepEnhancer
	samplingClient  domainsampling.UnifiedSampler
	promptManager   domainprompts.Manager
}

func (w *aiServicesWrapper) ErrorRecognizer() domainml.ErrorPatternRecognizer {
	return w.errorRecognizer
}
func (w *aiServicesWrapper) ErrorHandler() domainml.EnhancedErrorHandler {
	return w.errorHandler
}
func (w *aiServicesWrapper) StepEnhancer() domainml.StepEnhancer {
	return w.stepEnhancer
}
func (w *aiServicesWrapper) SamplingClient() domainsampling.UnifiedSampler {
	return w.samplingClient
}
func (w *aiServicesWrapper) PromptManager() domainprompts.Manager {
	return w.promptManager
}
