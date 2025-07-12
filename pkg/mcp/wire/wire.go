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
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	infraprogress "github.com/Azure/container-kit/pkg/mcp/infrastructure/progress"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/prompts"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/resources"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/sampling"
	"github.com/google/wire"
)

// ProviderSet contains all the providers for the MCP server
var ProviderSet = wire.NewSet(
	// Core providers
	provideServer,

	// Session management
	session.NewMemorySessionManager,
	provideDefaultTTL,
	provideMaxSessions,

	// Infrastructure providers
	resources.NewStore,
	provideSamplingClient,
	prompts.NewManager,
	providePromptManagerConfig,
	infraprogress.NewSinkFactory,

	// Dependencies struct
	provideDependencies,
)

// InitializeServer creates a fully wired MCP server
func InitializeServer(logger *slog.Logger, config workflow.ServerConfig) (api.MCPServer, error) {
	wire.Build(ProviderSet)
	return nil, nil
}

// provideDependencies creates the Dependencies struct with all wired components
func provideDependencies(
	logger *slog.Logger,
	config workflow.ServerConfig,
	sessionManager session.SessionManager,
	resourceStore *resources.Store,
	progressFactory *infraprogress.SinkFactory,
	samplingClient *sampling.Client,
	promptManager *prompts.Manager,
) *application.Dependencies {
	return &application.Dependencies{
		Logger:          logger,
		Config:          config,
		SessionManager:  sessionManager,
		ResourceStore:   resourceStore,
		ProgressFactory: progressFactory,
		SamplingClient:  samplingClient,
		PromptManager:   promptManager,
	}
}

// providePromptManagerConfig creates the config for the prompt manager
func providePromptManagerConfig(config workflow.ServerConfig) prompts.ManagerConfig {
	return prompts.ManagerConfig{
		TemplateDir:     "", // Use embedded templates only
		EnableHotReload: false,
		AllowOverride:   false,
	}
}

// provideDefaultTTL provides the default session TTL
func provideDefaultTTL() time.Duration {
	return 24 * time.Hour
}

// provideMaxSessions extracts max sessions from config
func provideMaxSessions(config workflow.ServerConfig) int {
	return config.MaxSessions
}

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
