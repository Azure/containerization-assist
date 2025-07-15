// Package composition defines the consolidated provider sets for dependency injection
package composition

import (
	"github.com/google/wire"

	"github.com/Azure/container-kit/pkg/mcp/application"
	"github.com/Azure/container-kit/pkg/mcp/application/config"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/ai_ml"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/core"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/messaging"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/observability"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/orchestration"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/persistence"
)

// CoreProviders provides fundamental system dependencies
var CoreProviders = wire.NewSet(
	// Core infrastructure
	core.Providers,

	// Configuration
	config.Providers,

	// Observability
	observability.ObservabilityProviders,
)

// AIMLProviders provides AI/ML services
var AIMLProviders = wire.NewSet(
	ai_ml.Providers,
	// TODO: Add configurable providers when LLM config is available
)

// InfrastructureProviders provides all infrastructure services
var InfrastructureProviders = wire.NewSet(
	// Core providers
	CoreProviders,

	// AI/ML providers
	AIMLProviders,

	// Orchestration providers
	orchestration.Providers,

	// Messaging providers
	messaging.Providers,

	// Persistence providers
	persistence.Providers,
)

// ApplicationProviders provides application layer services
var ApplicationProviders = wire.NewSet(
	application.Providers,
)

// AllProviders is the consolidated provider set that replaces the scattered approach
var AllProviders = wire.NewSet(
	// Infrastructure layer
	InfrastructureProviders,

	// Application layer
	ApplicationProviders,
)

// ProviderSet is the main provider set (maintained for backward compatibility)
var ProviderSet = AllProviders
