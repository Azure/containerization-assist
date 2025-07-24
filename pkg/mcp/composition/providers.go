// Package composition provides the consolidated provider set for dependency injection
package composition

import (
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/google/wire"

	"github.com/Azure/container-kit/pkg/common/runner"
	"github.com/Azure/container-kit/pkg/mcp/application"
	applicationsession "github.com/Azure/container-kit/pkg/mcp/application/session"
	domainevents "github.com/Azure/container-kit/pkg/mcp/domain/events"
	domainml "github.com/Azure/container-kit/pkg/mcp/domain/ml"
	domainprompts "github.com/Azure/container-kit/pkg/mcp/domain/prompts"
	domainresources "github.com/Azure/container-kit/pkg/mcp/domain/resources"
	domainsampling "github.com/Azure/container-kit/pkg/mcp/domain/sampling"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/ai_ml/ml"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/ai_ml/prompts"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/ai_ml/sampling"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/core/filesystem"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/core/resources"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/core/validation"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/messaging/events"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/messaging/progress"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/observability/health"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/observability/tracing"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/orchestration/container"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/orchestration/kubernetes"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/orchestration/steps"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/persistence"
)

// Providers is the single consolidated provider set that replaces all scattered approaches
var Providers = wire.NewSet(
	// Core infrastructure
	provideCommandRunner,
	filesystem.NewFileSystemManager,
	validation.NewPreflightValidator,

	// Workflow orchestration
	provideOrchestrator,
	steps.NewRegistryStepProvider,
	provideStepFactory,
	wire.Bind(new(workflow.WorkflowOrchestrator), new(*workflow.Orchestrator)),

	// Container & Kubernetes
	container.NewDockerContainerManager,
	kubernetes.NewKubernetesDeploymentManager,

	// Persistence
	provideSessionManager,
	provideResourceStore,
	provideStateStore,
	wire.Bind(new(domainresources.Store), new(*resources.Store)),

	// Messaging & Events
	events.NewPublisher,
	progress.NewDirectProgressFactory,
	wire.Bind(new(domainevents.Publisher), new(*events.Publisher)),
	wire.Bind(new(workflow.ProgressEmitterFactory), new(*progress.DirectProgressFactory)),

	// AI/ML Services
	provideSamplingClient,
	sampling.NewDomainAdapter,
	ml.NewAdvancedPatternRecognizer,
	ml.NewEnhancedErrorHandler,
	ml.NewStepEnhancer,
	providePromptManager,
	provideResourcePredictor,
	provideBuildOptimizer,
	wire.Bind(new(domainsampling.UnifiedSampler), new(*sampling.DomainAdapter)),
	wire.Bind(new(domainml.ErrorPatternRecognizer), new(*ml.AdvancedPatternRecognizer)),
	wire.Bind(new(domainml.EnhancedErrorHandler), new(*ml.EnhancedErrorHandler)),
	wire.Bind(new(domainml.StepEnhancer), new(*ml.StepEnhancer)),
	wire.Bind(new(domainprompts.Manager), new(*prompts.Manager)),
	wire.Bind(new(workflow.BuildOptimizer), new(*ml.BuildOptimizer)),

	// Observability
	health.NewMonitor,
	tracing.NewTracerAdapter,

	// Application layer - provide dependencies and server
	provideDependencies,
	application.ProvideServer,
)

// Provider functions - consolidating logic from scattered provider files

// Core infrastructure providers
func provideCommandRunner() runner.CommandRunner {
	return &runner.DefaultCommandRunner{}
}

// Workflow providers
func provideOrchestrator(
	stepProvider workflow.StepProvider,
	emitterFactory workflow.ProgressEmitterFactory,
	logger *slog.Logger,
) (*workflow.Orchestrator, error) {
	return workflow.NewOrchestrator(stepProvider, emitterFactory, logger)
}

func provideStepFactory(stepProvider workflow.StepProvider, optimizer workflow.BuildOptimizer, logger *slog.Logger) *workflow.StepFactory {
	return workflow.NewStepFactory(stepProvider, optimizer, nil, logger)
}

// Persistence providers
func provideSessionManager(config workflow.ServerConfig, logger *slog.Logger) (applicationsession.OptimizedSessionManager, error) {
	// Use StorePath from config or default to workspace dir
	dbPath := config.StorePath
	if dbPath == "" {
		dbPath = filepath.Join(config.WorkspaceDir, "sessions.db")
	}

	// Create the adapter that implements OptimizedSessionManager
	adapter, err := applicationsession.NewBoltStoreAdapter(dbPath, logger, config.SessionTTL, config.MaxSessions)
	if err != nil {
		return nil, fmt.Errorf("failed to create session manager: %w", err)
	}

	return adapter, nil
}

func provideStateStore(config workflow.ServerConfig, logger *slog.Logger) workflow.StateStore {
	return persistence.NewFileStateStore(config.WorkspaceDir, logger)
}

func provideResourceStore(logger *slog.Logger) *resources.Store {
	return resources.NewStore(logger)
}

// AI/ML providers
func provideSamplingClient(logger *slog.Logger) (*sampling.Client, error) {
	client, err := sampling.NewClientFromEnv(logger)
	if err != nil {
		return sampling.NewClient(logger), nil
	}
	return client, nil
}

func providePromptManager(logger *slog.Logger) (*prompts.Manager, error) {
	config := prompts.ManagerConfig{
		TemplateDir:     "",
		EnableHotReload: false,
		AllowOverride:   false,
	}
	return prompts.NewManager(logger, config)
}

func provideResourcePredictor(sampler domainsampling.UnifiedSampler, logger *slog.Logger) *ml.ResourcePredictor {
	return ml.NewResourcePredictor(sampler, logger)
}

func provideBuildOptimizer(predictor *ml.ResourcePredictor, logger *slog.Logger) *ml.BuildOptimizer {
	return ml.NewBuildOptimizer(predictor, logger)
}

// Application layer provider
func provideDependencies(
	logger *slog.Logger,
	config workflow.ServerConfig,
	sessionManager applicationsession.OptimizedSessionManager,
	resourceStore domainresources.Store,
	progressEmitterFactory workflow.ProgressEmitterFactory,
	eventPublisher domainevents.Publisher,
	workflowOrchestrator workflow.WorkflowOrchestrator,
	errorPatternRecognizer domainml.ErrorPatternRecognizer,
	enhancedErrorHandler domainml.EnhancedErrorHandler,
	stepEnhancer domainml.StepEnhancer,
	samplingClient domainsampling.UnifiedSampler,
	promptManager domainprompts.Manager,
) *application.Dependencies {
	return application.ProvideDependencies(
		logger,
		config,
		sessionManager,
		resourceStore,
		progressEmitterFactory,
		eventPublisher,
		workflowOrchestrator,
		errorPatternRecognizer,
		enhancedErrorHandler,
		stepEnhancer,
		samplingClient,
		promptManager,
	)
}
