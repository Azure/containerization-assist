//go:build wireinject
// +build wireinject

//go:generate wire

// Package wire provides dependency injection using Google Wire
package wire

import (
	"log/slog"
	"os"
	"strconv"
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

// ConfigurationSet - Configuration and environment providers
var ConfigurationSet = wire.NewSet(
	ProvideDefaultServerConfig,
)

// ApplicationSet - Core application dependencies
var ApplicationSet = wire.NewSet(
	ProvideSessionManager,
	ProvideResourceStore,
	ProvideProgressFactory,
)

// InfrastructureSet - Infrastructure layer dependencies
var InfrastructureSet = wire.NewSet(
	ProvideSamplingClient,
	ProvidePromptManager,
	provideDomainSampler,
	wire.Bind(new(domainsampling.UnifiedSampler), new(*sampling.DomainAdapter)),
)

// DomainSet - Domain services and events
var DomainSet = wire.NewSet(
	events.NewPublisher,
	saga.NewSagaCoordinator,
)

// WorkflowSet - Workflow orchestration (simplified for now)
var WorkflowSet = wire.NewSet(
	ProvideOrchestrator,
	ProvideEventOrchestrator,
	ProvideSagaOrchestrator,
)

// MLSet - Machine learning and enhanced capabilities (optional)
var MLSet = wire.NewSet(
	ml.ProvideErrorPatternRecognizer,
	ml.ProvideEnhancedErrorHandler,
	ml.ProvideStepEnhancer,
	// ml.ProvideOptimizedBuildStep, // Will add in Phase 2
)

// AppSet - Main application dependencies using wire.Struct
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

// ProviderSet - All providers for the MCP server
var ProviderSet = wire.NewSet(
	ConfigurationSet,
	ApplicationSet,
	InfrastructureSet,
	DomainSet,
	WorkflowSet,
	MLSet,
	AppSet,
	ProvideServer,
)

// InitializeServer creates a fully wired MCP server
func InitializeServer(logger *slog.Logger) (api.MCPServer, error) {
	wire.Build(ProviderSet)
	return nil, nil
}

// InitializeServerWithConfig creates a fully wired MCP server with custom config
func InitializeServerWithConfig(logger *slog.Logger, config workflow.ServerConfig) (api.MCPServer, error) {
	wire.Build(
		ApplicationSet,
		InfrastructureSet,
		DomainSet,
		WorkflowSet,
		MLSet,
		AppSet,
		ProvideServer,
	)
	return nil, nil
}

// Configuration Providers

// ProvideServerConfig creates a server config from individual components
func ProvideServerConfig(
	workspaceDir string,
	storePath string,
	maxSessions int,
	sessionTTL time.Duration,
	transportType string,
) workflow.ServerConfig {
	config := workflow.DefaultServerConfig()
	config.WorkspaceDir = workspaceDir
	config.StorePath = storePath
	config.MaxSessions = maxSessions
	config.SessionTTL = sessionTTL
	config.TransportType = transportType
	return config
}

// ProvideDefaultServerConfig creates a server config with default values from environment
func ProvideDefaultServerConfig() workflow.ServerConfig {
	config := workflow.DefaultServerConfig()
	config.WorkspaceDir = ProvideWorkspaceDir()
	config.StorePath = ProvideStorePath()
	config.MaxSessions = ProvideMaxSessions()
	config.SessionTTL = ProvideSessionTTL()
	config.TransportType = ProvideTransportType()
	return config
}

func ProvideWorkspaceDir() string {
	if dir := os.Getenv("CONTAINER_KIT_WORKSPACE"); dir != "" {
		return dir
	}
	return "/tmp/container-kit"
}

func ProvideStorePath() string {
	if path := os.Getenv("CONTAINER_KIT_STORE_PATH"); path != "" {
		return path
	}
	return "/tmp/container-kit/sessions.db"
}

func ProvideMaxSessions() int {
	if sessions := os.Getenv("CONTAINER_KIT_MAX_SESSIONS"); sessions != "" {
		if n, err := strconv.Atoi(sessions); err == nil {
			return n
		}
	}
	return 10
}

func ProvideSessionTTL() time.Duration {
	if ttl := os.Getenv("CONTAINER_KIT_SESSION_TTL"); ttl != "" {
		if d, err := time.ParseDuration(ttl); err == nil {
			return d
		}
	}
	return 24 * time.Hour
}

func ProvideTransportType() string {
	if transport := os.Getenv("CONTAINER_KIT_TRANSPORT"); transport != "" {
		return transport
	}
	return "stdio"
}

// Application Providers

func ProvideSessionManager(config workflow.ServerConfig, logger *slog.Logger) session.SessionManager {
	return session.NewMemorySessionManager(logger, config.SessionTTL, config.MaxSessions)
}

func ProvideResourceStore(logger *slog.Logger) *resources.Store {
	return resources.NewStore(logger)
}

func ProvideProgressFactory(logger *slog.Logger) *infraprogress.SinkFactory {
	return infraprogress.NewSinkFactory(logger)
}

// Infrastructure Providers

func ProvideSamplingClient(logger *slog.Logger) (*sampling.Client, error) {
	// Use environment-based configuration if available
	if os.Getenv("AZURE_OPENAI_ENDPOINT") != "" && os.Getenv("AZURE_OPENAI_KEY") != "" {
		return sampling.NewClientFromEnv(logger)
	}
	return sampling.NewClient(logger), nil
}

func ProvidePromptManager(logger *slog.Logger) (*prompts.Manager, error) {
	config := prompts.ManagerConfig{
		EnableHotReload: false,
		AllowOverride:   false,
	}
	return prompts.NewManager(logger, config)
}

// provideDomainSampler creates the domain adapter for sampling
func provideDomainSampler(client *sampling.Client) *sampling.DomainAdapter {
	return sampling.NewDomainAdapter(client)
}

// Workflow Providers

func ProvideOrchestrator(logger *slog.Logger) *workflow.Orchestrator {
	return workflow.NewOrchestrator(logger)
}

func ProvideEventOrchestrator(logger *slog.Logger, eventPublisher *events.Publisher) *workflow.EventOrchestrator {
	return workflow.NewEventOrchestrator(logger, eventPublisher)
}

func ProvideSagaOrchestrator(logger *slog.Logger, eventPublisher *events.Publisher, sagaCoordinator *saga.SagaCoordinator) *workflow.SagaOrchestrator {
	return workflow.NewSagaOrchestrator(logger, eventPublisher, sagaCoordinator)
}

// Server Provider

func ProvideServer(deps *application.Dependencies) api.MCPServer {
	return application.NewServer(
		application.WithDependencies(deps),
	)
}
