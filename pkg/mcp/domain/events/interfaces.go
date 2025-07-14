package events

import "context"

// Publisher defines the interface for publishing domain events.
// This interface is implemented by infrastructure layer.
type Publisher interface {
	// Publish publishes a domain event to all registered handlers
	Publish(ctx context.Context, event DomainEvent) error

	// PublishAsync publishes an event asynchronously without waiting for handlers
	PublishAsync(ctx context.Context, event DomainEvent)
}

// Handler represents a function that handles domain events
type Handler func(ctx context.Context, event DomainEvent) error
