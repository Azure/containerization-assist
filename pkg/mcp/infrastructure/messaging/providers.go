// Package messaging provides unified dependency injection for messaging and event services
package messaging

import (
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/messaging/progress"
	"github.com/google/wire"
)

// MessagingProviders provides all messaging domain dependencies
var MessagingProviders = wire.NewSet(
	// Event publishing - placeholder for when events.NewPublisher exists
	// events.NewPublisher,

	// Progress tracking - existing constructor
	progress.NewSinkFactory,

	// Progress emitters - component constructors (not used directly by wire)
	// progress.NewTrackerEmitter,
	// progress.NewBatchedEmitter,
	// progress.NewStreamingEmitter,

	// Interface bindings would go here if needed
)
