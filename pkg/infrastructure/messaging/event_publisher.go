package messaging

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/Azure/containerization-assist/pkg/domain/events"
)

type EventHandler func(ctx context.Context, event events.DomainEvent) error

type Publisher struct {
	handlers   map[string][]EventHandler
	logger     *slog.Logger
	mu         sync.RWMutex
	workerPool chan struct{} // Limits concurrent async operations
	wg         sync.WaitGroup
}

func NewPublisher(logger *slog.Logger) *Publisher {
	return &Publisher{
		logger:     logger,
		handlers:   make(map[string][]EventHandler),
		workerPool: make(chan struct{}, 10), // Limit to 10 concurrent async operations
	}
}

func (p *Publisher) Subscribe(eventType string, handler EventHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.handlers[eventType] = append(p.handlers[eventType], handler)
}

func (p *Publisher) Publish(ctx context.Context, event events.DomainEvent) error {
	p.mu.RLock()
	handlers := p.handlers[event.EventType()]
	p.mu.RUnlock()

	if len(handlers) == 0 {
		return nil
	}

	// Execute handlers concurrently
	var wg sync.WaitGroup
	errors := make(chan error, len(handlers))

	for _, handler := range handlers {
		wg.Add(1)
		go func(h EventHandler) {
			defer wg.Done()
			if err := h(ctx, event); err != nil {
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

// PublishAsync publishes an event asynchronously without waiting for handlers.
// Uses a worker pool to prevent goroutine leaks and manage resource usage.
func (p *Publisher) PublishAsync(ctx context.Context, event events.DomainEvent) {
	// Try to acquire a worker slot with a timeout to prevent blocking
	select {
	case p.workerPool <- struct{}{}:
		// Successfully acquired a worker slot
		p.wg.Add(1)
		go func() {
			defer func() {
				<-p.workerPool // Release worker slot
				p.wg.Done()
			}()

			// Create a timeout context for the async operation
			asyncCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			if err := p.Publish(asyncCtx, event); err != nil {
			}
		}()
	case <-time.After(100 * time.Millisecond):
		// Worker pool is full, log and drop the event
	}
}

// GetHandlerCount returns the number of handlers for an event type (for testing)
func (p *Publisher) GetHandlerCount(eventType string) int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.handlers[eventType])
}

// Close gracefully shuts down the publisher, waiting for all async operations to complete
func (p *Publisher) Close(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
