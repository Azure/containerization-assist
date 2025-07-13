//go:build wireinject
// +build wireinject

//go:generate wire

// Package wire provides dependency injection using Google Wire
package wire

import (
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/Azure/container-kit/pkg/mcp/application"
	"github.com/Azure/container-kit/pkg/mcp/application/session"
	"github.com/Azure/container-kit/pkg/mcp/domain/events"
	"github.com/Azure/container-kit/pkg/mcp/domain/saga"
	domainsampling "github.com/Azure/container-kit/pkg/mcp/domain/sampling"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/ml"
	infraprogress "github.com/Azure/container-kit/pkg/mcp/infrastructure/progress"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/prompts"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/resources"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/sampling"
	"github.com/google/wire"
)

// ConfigSet contains configuration-related providers
var ConfigSet = wire.NewSet(
	wire.Value(24*time.Hour), // Default TTL
	providePromptManagerConfig,
	wire.FieldsOf(new(workflow.ServerConfig), "MaxSessions"),
)

// CoreSet contains core infrastructure providers
var CoreSet = wire.NewSet(
	resources.NewStore,
	provideSamplingClient,
	provideDomainSampler,
	// Bind domain interfaces to the adapter implementation
	wire.Bind(new(domainsampling.Sampler), new(*sampling.DomainAdapter)),
	wire.Bind(new(domainsampling.AnalysisSampler), new(*sampling.DomainAdapter)),
	wire.Bind(new(domainsampling.FixSampler), new(*sampling.DomainAdapter)),
	prompts.NewManager,
	infraprogress.NewSinkFactory,
)

// SessionSet contains session management providers
var SessionSet = wire.NewSet(
	session.NewMemorySessionManager,
)

// EventSet contains event-driven architecture providers
var EventSet = wire.NewSet(
	events.NewPublisher,
	provideProgressEventHandler,
	provideMetricsEventHandler,
)

// SagaSet contains saga pattern providers
var SagaSet = wire.NewSet(
	saga.NewSagaCoordinator,
)

// MLSet contains machine learning and error analysis providers
var MLSet = wire.NewSet(
	ml.ProvideErrorPatternRecognizer,
	ml.ProvideEnhancedErrorHandler,
	ml.ProvideStepEnhancer,
	ml.ProvideResourcePredictor,
	ml.ProvideBuildOptimizer,
	ml.ProvideOptimizedBuildStep,
)

// OrchestrationSet contains workflow orchestration providers
var OrchestrationSet = wire.NewSet(
	workflow.ProvideStepFactory,
	workflow.ProvideOptimizedOrchestrator,
	workflow.NewEventOrchestrator,
	workflow.NewSagaOrchestrator,
)

// AppSet contains the main application dependencies using wire.Struct
var AppSet = wire.NewSet(
	wire.Struct(
		new(application.Dependencies),
		"Logger", "Config", "SessionManager", "ResourceStore",
		"ProgressFactory", "EventPublisher", "SagaCoordinator",
		"Orchestrator", "EventOrchestrator", "SagaOrchestrator",
		"ErrorPatternRecognizer", "EnhancedErrorHandler", "StepEnhancer",
		"SamplingClient", "PromptManager",
	),
)

// ProviderSet contains all the providers for the MCP server
var ProviderSet = wire.NewSet(
	ConfigSet,
	CoreSet,
	SessionSet,
	EventSet,
	SagaSet,
	MLSet,
	OrchestrationSet,
	AppSet,
	provideServer,
)

// InitializeServer creates a fully wired MCP server
func InitializeServer(logger *slog.Logger, config workflow.ServerConfig) (api.MCPServer, error) {
	wire.Build(ProviderSet)
	return nil, nil
}

// Note: provideDependencies removed - using wire.Struct for automatic field wiring

// providePromptManagerConfig creates the config for the prompt manager
func providePromptManagerConfig(config workflow.ServerConfig) prompts.ManagerConfig {
	return prompts.ManagerConfig{
		TemplateDir:     "", // Use embedded templates only
		EnableHotReload: false,
		AllowOverride:   false,
	}
}

// Note: provideDefaultTTL removed - using wire.Value(24*time.Hour) instead

// Note: provideMaxSessions removed - using wire.FieldsOf(ServerConfig, \"MaxSessions\") instead"

// provideServer creates the MCP server with dependencies
func provideServer(deps *application.Dependencies) api.MCPServer {
	return application.NewServer(
		application.WithDependencies(deps),
	)
}

// provideSamplingClient creates the sampling client without options
func provideSamplingClient(logger *slog.Logger) *sampling.Client {
	return sampling.NewClient(logger)
}

// provideProgressEventHandler creates the progress event handler
func provideProgressEventHandler(logger *slog.Logger) *events.ProgressEventHandler {
	return events.NewProgressEventHandler(logger)
}

// provideMetricsEventHandler creates the metrics event handler
func provideMetricsEventHandler(logger *slog.Logger) *events.MetricsEventHandler {
	return events.NewMetricsEventHandler(logger)
}

// provideDomainSampler creates the domain adapter for sampling
func provideDomainSampler(client *sampling.Client) *sampling.DomainAdapter {
	return sampling.NewDomainAdapter(client)
}
