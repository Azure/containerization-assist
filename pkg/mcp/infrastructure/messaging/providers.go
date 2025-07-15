// Package messaging provides unified dependency injection for messaging and event services
package messaging

import (
	"log/slog"

	domainevents "github.com/Azure/container-kit/pkg/mcp/domain/events"
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

	// Progress tracking - direct approach only
	ProvideDirectProgressFactory,
	wire.Bind(new(workflow.ProgressEmitterFactory), new(*progress.DirectProgressFactory)),
)

// DirectProviders is now the same as Providers (kept for compatibility)
var DirectProviders = Providers

// ProvideDirectProgressFactory creates the new direct progress factory
func ProvideDirectProgressFactory(logger *slog.Logger) *progress.DirectProgressFactory {
	return progress.NewDirectProgressFactory(logger)
}
