// Package messaging provides unified dependency injection for messaging and event services
package messaging

import (
	"log/slog"

	domainevents "github.com/Azure/container-kit/pkg/mcp/domain/events"
	domainprogress "github.com/Azure/container-kit/pkg/mcp/domain/progress"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/messaging/events"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/messaging/progress"
	"github.com/google/wire"
)

// Providers provides all messaging domain dependencies
var Providers = wire.NewSet(
	// Event publishing
	events.NewPublisher,
	wire.Bind(new(domainevents.Publisher), new(*events.Publisher)),

	// Progress tracking - unified factory approach
	progress.NewSinkFactory,
	ProvideUnifiedProgressFactory,
	wire.Bind(new(workflow.ProgressEmitterFactory), new(*progress.UnifiedFactory)),
	wire.Bind(new(domainprogress.EmitterFactory), new(*progress.UnifiedFactory)),

	// Legacy factory for backward compatibility
	ProvideProgressEmitterFactory,

	// Interface bindings would go here if needed
)

// ProvideUnifiedProgressFactory creates a unified progress emitter factory
func ProvideUnifiedProgressFactory(logger *slog.Logger) *progress.UnifiedFactory {
	return progress.NewUnifiedFactory(logger)
}

// ProvideProgressEmitterFactory creates a progress emitter factory (legacy)
func ProvideProgressEmitterFactory(sinkFactory *progress.SinkFactory) *progress.ProgressEmitterFactory {
	// Use default configuration for progress emitter
	config := progress.DefaultEmitterConfig()
	return progress.NewProgressEmitterFactory(config, sinkFactory)
}
