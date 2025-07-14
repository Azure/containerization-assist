// Package wiring provides centralized dependency injection configuration for the MCP server.
// This package wires together all layers of the application following clean architecture principles.
package wiring

import (
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/common/runner"
	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/Azure/container-kit/pkg/mcp/application"
	"github.com/Azure/container-kit/pkg/mcp/application/config"
	"github.com/Azure/container-kit/pkg/mcp/application/session"
	domainevents "github.com/Azure/container-kit/pkg/mcp/domain/events"
	domainml "github.com/Azure/container-kit/pkg/mcp/domain/ml"
	domainprompts "github.com/Azure/container-kit/pkg/mcp/domain/prompts"
	domainresources "github.com/Azure/container-kit/pkg/mcp/domain/resources"
	"github.com/Azure/container-kit/pkg/mcp/domain/saga"
	domainsampling "github.com/Azure/container-kit/pkg/mcp/domain/sampling"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/container"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/events"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/kubernetes"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/ml"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/persistence"
	infraprogress "github.com/Azure/container-kit/pkg/mcp/infrastructure/progress"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/prompts"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/resources"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/sampling"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/steps"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/steps/optimized"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/tracing"
	"github.com/google/wire"
)

// CommonProviders - Basic infrastructure shared across all components
var CommonProviders = wire.NewSet(
	// Configuration
	ProvideConfig,
	ProvideServerConfigFromUnified,

	// State storage
	ProvideStateStore,

	// Tracing
	tracing.NewTracerAdapter,

	// Command runner
	ProvideCommandRunner,
)

// ConfigProviders - Configuration providers for different subsystems
var ConfigProviders = wire.NewSet(
	// Configuration conversions
	ProvideTracingConfig,
	ProvideSecurityConfig,
	ProvideRegistryConfig,
)

// DomainProviders - Core domain services and business logic
var DomainProviders = wire.NewSet(
	// Events and coordination
	events.NewPublisher,
	wire.Bind(new(domainevents.Publisher), new(*events.Publisher)),
	saga.NewSagaCoordinator,

	// Workflow services
	ProvideStepFactory,
	ProvideBaseOrchestrator,
	ProvideEventOrchestrator,
	ProvideSagaOrchestrator,
	ProvideWorkflowOrchestrator,

	// Sampling domain adapter
	provideDomainSampler,
	wire.Bind(new(domainsampling.UnifiedSampler), new(*sampling.DomainAdapter)),
)

// InfraProviders - Infrastructure implementations and external integrations
var InfraProviders = wire.NewSet(
	// Progress tracking
	ProvideProgressFactory,
	wire.Bind(new(workflow.ProgressTrackerFactory), new(*infraprogress.SinkFactory)),

	// AI/ML services
	ProvideSamplingClient,
	ProvidePromptManager,
	
	// Interface bindings for domain abstractions
	wire.Bind(new(domainprompts.Manager), new(*prompts.Manager)),

	// Container and deployment
	ProvideContainerManager,
	ProvideDeploymentManager,

	// Step implementations
	ProvideStepProvider,
	ProvideOptimizedBuildStep,
)

// MLProviders - Machine learning and enhanced capabilities (optional)
var MLProviders = wire.NewSet(
	ProvideResourcePredictor,
	ProvideBuildOptimizer,
	ProvideErrorPatternRecognizer,
	ProvideEnhancedErrorHandler,
	ProvideStepEnhancer,
	ProvideMLOptimizedBuildStep,
	
	// Interface bindings for domain abstractions
	wire.Bind(new(domainml.ErrorPatternRecognizer), new(*ml.ErrorPatternRecognizer)),
	wire.Bind(new(domainml.EnhancedErrorHandler), new(*ml.EnhancedErrorHandler)),
	wire.Bind(new(domainml.StepEnhancer), new(*ml.StepEnhancer)),
)

// ApplicationProviders - Application layer services and coordination
var ApplicationProviders = wire.NewSet(
	// Session management
	ProvideSessionManager,
	ProvideResourceStore,

	// Application dependencies structure
	wire.Struct(
		new(application.Dependencies),
		"Logger", "Config", "SessionManager", "ResourceStore",
		"ProgressFactory", "EventPublisher", "SagaCoordinator",
		"WorkflowOrchestrator", "EventAwareOrchestrator", "SagaAwareOrchestrator",
		"ErrorPatternRecognizer", "EnhancedErrorHandler", "StepEnhancer",
		"SamplingClient", "PromptManager",
	),

	// Main server
	ProvideServer,
)

// ProviderSet - Complete dependency graph for the MCP server
var ProviderSet = wire.NewSet(
	CommonProviders,
	ConfigProviders,
	DomainProviders,
	InfraProviders,
	MLProviders,
	ApplicationProviders,
)

// BasicProviderSet - Minimal provider set for basic functionality (without ML)
var BasicProviderSet = wire.NewSet(
	CommonProviders,
	ConfigProviders,
	DomainProviders,
	InfraProviders,
	ApplicationProviders,
)

// TestProviderSet - Provider set optimized for testing scenarios
var TestProviderSet = wire.NewSet(
	CommonProviders,
	ConfigProviders,
	DomainProviders,
	
	// Simplified infra providers for testing
	ProvideProgressFactory,
	wire.Bind(new(workflow.ProgressTrackerFactory), new(*infraprogress.SinkFactory)),
	ProvideSamplingClient,
	ProvidePromptManager,
	ProvideStepProvider,
	
	// Application layer
	ProvideSessionManager,
	ProvideResourceStore,
)

// Configuration Providers

// ProvideConfig loads the unified configuration from environment and config files
func ProvideConfig() (*config.Config, error) {
	cfg, err := config.Load(config.FromEnv())
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// ProvideServerConfigFromUnified converts unified config to ServerConfig
func ProvideServerConfigFromUnified(cfg *config.Config) workflow.ServerConfig {
	return cfg.ToServerConfig()
}

// ProvideConfigFromServerConfig converts ServerConfig to Config for custom config usage
func ProvideConfigFromServerConfig(serverConfig workflow.ServerConfig) *config.Config {
	return &config.Config{
		// Map ServerConfig fields to Config
		WorkspaceDir:   serverConfig.WorkspaceDir,
		StorePath:      serverConfig.StorePath,
		MaxSessions:    serverConfig.MaxSessions,
		SessionTTL:     serverConfig.SessionTTL,
		LogLevel:       serverConfig.LogLevel,
		TransportType:  serverConfig.TransportType,
		HTTPAddr:       serverConfig.HTTPAddr,
		HTTPPort:       serverConfig.HTTPPort,
		
		// Add defaults for sampling config
		SamplingMaxTokens:     4096,
		SamplingTemperature:   0.7,
		SamplingRetryAttempts: 3,
		SamplingTokenBudget:   100000,
		SamplingStreaming:     false,
		
		// Add defaults for other fields
		MaxDiskPerSession: 1024 * 1024 * 1024, // 1GB
		TotalDiskLimit:    5 * 1024 * 1024 * 1024, // 5GB
		CleanupInterval:   30 * time.Minute,
		
		// Tracing defaults
		TracingEnabled:     false,
		TracingServiceName: "container-kit-mcp",
		TracingSampleRate:  1.0,
		
		// Security defaults
		SecurityScanEnabled:    true,
		SecurityFailOnHigh:     false,
		SecurityFailOnCritical: true,
		
		// Prompt defaults
		PromptTemplateDir:   "templates",
		PromptHotReload:     false,
		PromptAllowOverride: false,
	}
}

// Application Providers

func ProvideSessionManager(config workflow.ServerConfig, logger *slog.Logger) session.OptimizedSessionManager {
	return session.NewOptimizedSessionManager(logger, config.SessionTTL, config.MaxSessions)
}

func ProvideResourceStore(logger *slog.Logger) domainresources.Store {
	return resources.NewStore(logger)
}

func ProvideProgressFactory(logger *slog.Logger) *infraprogress.SinkFactory {
	return infraprogress.NewSinkFactory(logger)
}

// Infrastructure Providers

func ProvideSamplingClient(cfg *config.Config, logger *slog.Logger) (*sampling.Client, error) {
	// Use the centralized config conversion method
	samplingConfig := cfg.ToSamplingConfig()
	
	// Convert to infrastructure sampling config
	infraConfig := sampling.Config{
		MaxTokens:        samplingConfig.MaxTokens,
		Temperature:      samplingConfig.Temperature,
		RetryAttempts:    samplingConfig.RetryAttempts,
		TokenBudget:      samplingConfig.TokenBudget,
		BaseBackoff:      samplingConfig.BaseBackoff,
		MaxBackoff:       samplingConfig.MaxBackoff,
		StreamingEnabled: samplingConfig.StreamingEnabled,
		RequestTimeout:   samplingConfig.RequestTimeout,
	}

	return sampling.NewClient(logger, sampling.WithConfig(infraConfig)), nil
}

func ProvidePromptManager(cfg *config.Config, logger *slog.Logger) (*prompts.Manager, error) {
	promptConfig := cfg.ToPromptConfig()
	mcpPromptConfig := prompts.ManagerConfig{
		TemplateDir:     promptConfig.TemplateDir,
		EnableHotReload: promptConfig.EnableHotReload,
		AllowOverride:   promptConfig.AllowOverride,
	}
	return prompts.NewManager(logger, mcpPromptConfig)
}

// provideDomainSampler creates the domain adapter for sampling
func provideDomainSampler(client *sampling.Client) *sampling.DomainAdapter {
	return sampling.NewDomainAdapter(client)
}

func ProvideStateStore(config workflow.ServerConfig, logger *slog.Logger) workflow.StateStore {
	return persistence.NewFileStateStore(config.WorkspaceDir, logger)
}

// Server Provider

func ProvideServer(deps *application.Dependencies) api.MCPServer {
	return application.NewMCPServerFromDeps(deps)
}

// Domain Workflow Providers

// ProvideStepFactory creates a StepFactory with optional ML optimization
func ProvideStepFactory(stepProvider workflow.StepProvider, optimizedBuildStep workflow.Step, logger *slog.Logger) *workflow.StepFactory {
	return workflow.NewStepFactory(stepProvider, nil, optimizedBuildStep, logger)
}

// ProvideBaseOrchestrator creates a concrete BaseOrchestrator
func ProvideBaseOrchestrator(factory *workflow.StepFactory, progressFactory workflow.ProgressTrackerFactory, logger *slog.Logger, tracer workflow.Tracer) *workflow.BaseOrchestrator {
	// Create base orchestrator with common middleware using functional options
	var opts []workflow.OrchestratorOption

	// Add common middleware
	middlewares := []workflow.StepMiddleware{
		workflow.RetryMiddleware(),
		workflow.ProgressMiddleware(),
	}

	// Add tracing middleware if tracer is available
	if tracer != nil {
		middlewares = append([]workflow.StepMiddleware{workflow.TracingMiddleware(tracer)}, middlewares...)
	}

	opts = append(opts, workflow.WithMiddleware(middlewares...))

	return workflow.NewBaseOrchestrator(factory, progressFactory, logger, opts...)
}

// ProvideEventOrchestrator creates an EventOrchestrator using decorators
func ProvideEventOrchestrator(orchestrator *workflow.BaseOrchestrator, publisher *events.Publisher) workflow.EventAwareOrchestrator {
	// Use the decorator pattern to add event awareness
	return workflow.WithEvents(orchestrator, publisher)
}

// ProvideSagaOrchestrator creates a SagaOrchestrator using decorators
func ProvideSagaOrchestrator(eventOrchestrator workflow.EventAwareOrchestrator, sagaCoordinator *saga.SagaCoordinator, containerManager workflow.ContainerManager, deploymentManager workflow.DeploymentManager, logger *slog.Logger) workflow.SagaAwareOrchestrator {
	// Use the decorator pattern to add saga support
	return workflow.WithSagaAndDependencies(eventOrchestrator, sagaCoordinator, containerManager, deploymentManager, logger)
}

// ProvideWorkflowOrchestrator provides the base workflow orchestrator as an interface
func ProvideWorkflowOrchestrator(orchestrator *workflow.BaseOrchestrator) workflow.WorkflowOrchestrator {
	return orchestrator
}

// Infrastructure Providers

// ProvideContainerManager creates a Docker container manager
func ProvideContainerManager(runner runner.CommandRunner, logger *slog.Logger) workflow.ContainerManager {
	return container.NewDockerContainerManager(runner, logger)
}

// ProvideDeploymentManager creates a Kubernetes deployment manager
func ProvideDeploymentManager(runner runner.CommandRunner, logger *slog.Logger) workflow.DeploymentManager {
	return kubernetes.NewKubernetesDeploymentManager(runner, logger)
}

// ProvideStepProvider creates a step provider
func ProvideStepProvider(logger *slog.Logger) workflow.StepProvider {
	return steps.NewRegistryStepProvider()
}

// ProvideOptimizedBuildStep creates an optimized build step
func ProvideOptimizedBuildStep(optimizedBuildStep *ml.OptimizedBuildStep) workflow.Step {
	return optimized.NewOptimizedBuildStep(optimizedBuildStep)
}

// ProvideResourcePredictor creates a resource predictor
func ProvideResourcePredictor(sampler domainsampling.UnifiedSampler, logger *slog.Logger) *ml.ResourcePredictor {
	return ml.NewResourcePredictor(sampler, logger)
}

// ProvideBuildOptimizer creates a build optimizer
func ProvideBuildOptimizer(predictor *ml.ResourcePredictor, logger *slog.Logger) *ml.BuildOptimizer {
	return ml.NewBuildOptimizer(predictor, logger)
}

// ML Providers

// ProvideErrorPatternRecognizer creates an error pattern recognizer
func ProvideErrorPatternRecognizer(sampler domainsampling.UnifiedSampler, logger *slog.Logger) *ml.ErrorPatternRecognizer {
	return ml.NewErrorPatternRecognizer(sampler, logger)
}

// ProvideEnhancedErrorHandler creates an enhanced error handler
func ProvideEnhancedErrorHandler(sampler domainsampling.UnifiedSampler, publisher *events.Publisher, logger *slog.Logger) *ml.EnhancedErrorHandler {
	return ml.NewEnhancedErrorHandler(sampler, publisher, logger)
}

// ProvideStepEnhancer creates a step enhancer
func ProvideStepEnhancer(errorHandler *ml.EnhancedErrorHandler, logger *slog.Logger) *ml.StepEnhancer {
	return ml.NewStepEnhancer(errorHandler, logger)
}

// ProvideMLOptimizedBuildStep creates an ML-optimized build step
func ProvideMLOptimizedBuildStep(sampler domainsampling.UnifiedSampler, logger *slog.Logger) *ml.OptimizedBuildStep {
	return ml.NewOptimizedBuildStep(sampler, logger)
}

// ProvideCommandRunner creates a command runner
func ProvideCommandRunner() runner.CommandRunner {
	return &runner.DefaultCommandRunner{}
}

// ProvideTracingConfig creates tracing configuration from unified config
func ProvideTracingConfig(cfg *config.Config) *tracing.Config {
	tracingConfig := cfg.ToTracingConfig()
	
	// Convert to infrastructure tracing config
	return &tracing.Config{
		Enabled:        tracingConfig.Enabled,
		Endpoint:       tracingConfig.Endpoint,
		Headers:        make(map[string]string), // Can be populated from env vars
		ServiceName:    tracingConfig.ServiceName,
		ServiceVersion: "dev", // Could be made configurable
		Environment:    "development", // Could be made configurable
		SampleRate:     tracingConfig.SampleRate,
		ExportTimeout:  30 * time.Second,
	}
}

// ProvideSecurityConfig creates security configuration from unified config
func ProvideSecurityConfig(cfg *config.Config) *config.SecurityConfig {
	securityConfig := cfg.ToSecurityConfig()
	return &securityConfig
}

// ProvideRegistryConfig creates registry configuration from unified config
func ProvideRegistryConfig(cfg *config.Config) *config.RegistryConfig {
	registryConfig := cfg.ToRegistryConfig()
	return &registryConfig
}
