// Package composition defines the provider sets for dependency injection
package composition

import (
	"github.com/google/wire"

	"github.com/Azure/container-kit/pkg/mcp/application"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/ai_ml"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/core"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/messaging"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/observability"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/orchestration"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/persistence"
)

// ProviderSet collects all individual layer sets.
var ProviderSet = wire.NewSet(
	// Infrastructure layer providers
	core.Providers,
	ai_ml.Providers,
	orchestration.Providers,
	observability.ObservabilityProviders,
	messaging.Providers,
	persistence.Providers,

	// Application layer providers
	application.Providers,
)
