package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Azure/containerization-assist/pkg/mcp/api"
	"github.com/Azure/containerization-assist/pkg/mcp/domain/workflow"
	"github.com/Azure/containerization-assist/pkg/mcp/infrastructure/ai_ml/prompts"
	"github.com/Azure/containerization-assist/pkg/mcp/infrastructure/ai_ml/sampling"
	"github.com/Azure/containerization-assist/pkg/mcp/infrastructure/core/resources"
	"github.com/Azure/containerization-assist/pkg/mcp/infrastructure/messaging"
	"github.com/Azure/containerization-assist/pkg/mcp/infrastructure/orchestration/steps"
	"github.com/Azure/containerization-assist/pkg/mcp/service/session"
	"github.com/mark3labs/mcp-go/mcp"
)

type ServerFactory struct {
	logger *slog.Logger
	config workflow.ServerConfig
}

func NewServerFactory(logger *slog.Logger, config workflow.ServerConfig) *ServerFactory {
	return &ServerFactory{
		logger: logger,
		config: config,
	}
}

func (f *ServerFactory) CreateServer(ctx context.Context) (api.MCPServer, error) {
	f.logger.Info("Creating MCP server with direct dependency injection")

	// Build dependencies in correct order
	deps, err := f.buildDependencies(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to build dependencies: %w", err)
	}

	// Create and return server
	server, err := NewMCPServerFromDeps(deps)
	if err != nil {
		return nil, fmt.Errorf("failed to create server: %w", err)
	}

	f.logger.Info("MCP server created successfully")
	return server, nil
}

// BuildDependenciesForTools builds dependencies for tool execution (exposed for tool mode)
func (f *ServerFactory) BuildDependenciesForTools(ctx context.Context) (*Dependencies, error) {
	return f.buildDependencies(ctx)
}

func (f *ServerFactory) buildDependencies(ctx context.Context) (*Dependencies, error) {
	// Create session manager
	sessionManager, err := f.createSessionManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create session manager: %w", err)
	}

	// Create resource store
	resourceStore, err := f.createResourceStore()
	if err != nil {
		return nil, fmt.Errorf("failed to create resource store: %w", err)
	}

	// Create event publisher
	eventPublisher, err := f.createEventPublisher()
	if err != nil {
		return nil, fmt.Errorf("failed to create event publisher: %w", err)
	}

	// Create AI/ML services
	samplingClient, err := f.createSamplingClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create sampling client: %w", err)
	}

	promptManager, err := f.createPromptManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create prompt manager: %w", err)
	}

	// ML services simplified - removed over-abstracted ErrorPatternRecognizer

	// Create workflow orchestrator
	workflowOrchestrator, err := f.createWorkflowOrchestrator(
		sessionManager,
		samplingClient,
		promptManager,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create workflow orchestrator: %w", err)
	}

	// Assemble dependencies
	deps := &Dependencies{
		Logger:               f.logger,
		Config:               f.config,
		SessionManager:       sessionManager,
		ResourceStore:        resourceStore,
		EventPublisher:       eventPublisher,
		WorkflowOrchestrator: workflowOrchestrator,
		SamplingClient:       samplingClient,
		PromptManager:        promptManager,
	}

	// Validate dependencies
	if err := deps.Validate(); err != nil {
		return nil, fmt.Errorf("dependency validation failed: %w", err)
	}

	return deps, nil
}

func (f *ServerFactory) createSessionManager() (*session.BoltStoreAdapter, error) {
	return session.NewBoltStoreAdapter(f.config.StorePath, f.logger, f.config.SessionTTL, f.config.MaxSessions)
}

func (f *ServerFactory) createResourceStore() (*resources.Store, error) {
	return resources.NewStore(f.logger), nil
}

func (f *ServerFactory) createEventPublisher() (*messaging.Publisher, error) {
	return messaging.NewPublisher(f.logger), nil
}

func (f *ServerFactory) createSamplingClient() (*sampling.Client, error) {
	return sampling.NewClient(f.logger), nil
}

func (f *ServerFactory) createPromptManager() (*prompts.Manager, error) {
	config := prompts.ManagerConfig{
		TemplateDir:     "", // Use embedded templates only
		EnableHotReload: false,
		AllowOverride:   false,
	}
	return prompts.NewManager(f.logger, config)
}

func (f *ServerFactory) createWorkflowOrchestrator(
	sessionManager *session.BoltStoreAdapter,
	samplingClient *sampling.Client,
	promptManager *prompts.Manager,
) (*workflow.Orchestrator, error) {
	// Create step provider using the registry-based provider
	stepProvider := steps.NewRegistryStepProvider()

	// Create orchestrator using domain implementation
	progressFactory := func(ctx context.Context, req *mcp.CallToolRequest) api.ProgressEmitter {
		return messaging.CreateProgressEmitter(ctx, req, 10, f.logger)
	}
	return workflow.NewOrchestrator(stepProvider, f.logger, progressFactory)
}

func InitializeServer(logger *slog.Logger, config workflow.ServerConfig) (api.MCPServer, error) {
	factory := NewServerFactory(logger, config)
	return factory.CreateServer(context.Background())
}
