package messaging

import (
	"context"
	"log/slog"
	"sync"

	"github.com/Azure/container-kit/pkg/mcp/domain/events"
)

type EventHandler func(ctx context.Context, event events.DomainEvent) error

type Publisher struct {
	handlers map[string][]EventHandler
	logger   *slog.Logger
	mu       sync.RWMutex
}

func NewPublisher(logger *slog.Logger) *Publisher {
	return &Publisher{
		handlers: make(map[string][]EventHandler),
		logger:   logger.With("component", "event_publisher"),
	}
}

func (p *Publisher) Subscribe(eventType string, handler EventHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.handlers[eventType] = append(p.handlers[eventType], handler)
	p.logger.Info("Event handler registered", "event_type", eventType)
}

func (p *Publisher) Publish(ctx context.Context, event events.DomainEvent) error {
	p.mu.RLock()
	handlers := p.handlers[event.EventType()]
	p.mu.RUnlock()

	if len(handlers) == 0 {
		p.logger.Debug("No handlers for event", "event_type", event.EventType(), "event_id", event.EventID())
		return nil
	}

	p.logger.Info("Publishing event",
		"event_type", event.EventType(),
		"event_id", event.EventID(),
		"workflow_id", event.WorkflowID(),
		"handler_count", len(handlers))

	// Execute handlers concurrently
	var wg sync.WaitGroup
	errors := make(chan error, len(handlers))

	for _, handler := range handlers {
		wg.Add(1)
		go func(h EventHandler) {
			defer wg.Done()
			if err := h(ctx, event); err != nil {
				p.logger.Error("Event handler failed",
					"event_type", event.EventType(),
					"event_id", event.EventID(),
					"error", err)
				errors <- err
			}
		}(handler)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		if err != nil {
			return err // Return first error encountered
		}
	}

	return nil
}

func (p *Publisher) PublishAsync(ctx context.Context, event events.DomainEvent) {
	go func() {
		if err := p.Publish(ctx, event); err != nil {
			p.logger.Error("Async event publishing failed",
				"event_type", event.EventType(),
				"event_id", event.EventID(),
				"error", err)
		}
	}()
}

// GetHandlerCount returns the number of handlers for an event type (for testing)
func (p *Publisher) GetHandlerCount(eventType string) int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.handlers[eventType])
}
