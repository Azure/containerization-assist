package messaging

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Azure/containerization-assist/pkg/domain/events"
	"github.com/stretchr/testify/assert"
)

// mockDomainEvent implements events.DomainEvent for testing
type mockDomainEvent struct {
	eventType string
}

func (m *mockDomainEvent) EventType() string { return m.eventType }

// Test helper to create a publisher with discarded logs
func createTestPublisher() *Publisher {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewPublisher(logger)
}

func TestNewPublisher(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	publisher := NewPublisher(logger)

	assert.NotNil(t, publisher)
	assert.NotNil(t, publisher.handlers)
	assert.NotNil(t, publisher.logger)
	assert.Equal(t, 0, len(publisher.handlers))
}

func TestPublisher_Subscribe(t *testing.T) {
	publisher := createTestPublisher()

	// Subscribe to an event type
	handler := func(ctx context.Context, event events.DomainEvent) error {
		return nil
	}

	publisher.Subscribe("test_event", handler)

	// Verify handler was registered
	assert.Equal(t, 1, publisher.GetHandlerCount("test_event"))
	assert.Equal(t, 0, publisher.GetHandlerCount("nonexistent_event"))
}

func TestPublisher_Subscribe_MultipleHandlers(t *testing.T) {
	publisher := createTestPublisher()

	// Subscribe multiple handlers to the same event type
	handler1 := func(ctx context.Context, event events.DomainEvent) error { return nil }
	handler2 := func(ctx context.Context, event events.DomainEvent) error { return nil }
	handler3 := func(ctx context.Context, event events.DomainEvent) error { return nil }

	publisher.Subscribe("test_event", handler1)
	publisher.Subscribe("test_event", handler2)
	publisher.Subscribe("test_event", handler3)

	assert.Equal(t, 3, publisher.GetHandlerCount("test_event"))
}

func TestPublisher_Publish_NoHandlers(t *testing.T) {
	publisher := createTestPublisher()

	event := &mockDomainEvent{
		eventType: "test_event",
	}

	// Should not error when no handlers are registered
	err := publisher.Publish(context.Background(), event)
	assert.NoError(t, err)
}

func TestPublisher_Publish_SingleHandler_Success(t *testing.T) {
	publisher := createTestPublisher()

	// Track handler execution
	var handlerCalled int32
	var receivedEvent events.DomainEvent

	handler := func(ctx context.Context, event events.DomainEvent) error {
		atomic.AddInt32(&handlerCalled, 1)
		receivedEvent = event
		return nil
	}

	publisher.Subscribe("test_event", handler)

	event := &mockDomainEvent{
		eventType: "test_event",
	}

	err := publisher.Publish(context.Background(), event)

	assert.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&handlerCalled))
	assert.Equal(t, event, receivedEvent)
}

func TestPublisher_Publish_MultipleHandlers_Success(t *testing.T) {
	publisher := createTestPublisher()

	// Track handler executions
	var handler1Called, handler2Called, handler3Called int32

	handler1 := func(ctx context.Context, event events.DomainEvent) error {
		atomic.AddInt32(&handler1Called, 1)
		return nil
	}
	handler2 := func(ctx context.Context, event events.DomainEvent) error {
		atomic.AddInt32(&handler2Called, 1)
		return nil
	}
	handler3 := func(ctx context.Context, event events.DomainEvent) error {
		atomic.AddInt32(&handler3Called, 1)
		return nil
	}

	publisher.Subscribe("test_event", handler1)
	publisher.Subscribe("test_event", handler2)
	publisher.Subscribe("test_event", handler3)

	event := &mockDomainEvent{
		eventType: "test_event",
	}

	err := publisher.Publish(context.Background(), event)

	assert.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&handler1Called))
	assert.Equal(t, int32(1), atomic.LoadInt32(&handler2Called))
	assert.Equal(t, int32(1), atomic.LoadInt32(&handler3Called))
}

func TestPublisher_Publish_HandlerError(t *testing.T) {
	publisher := createTestPublisher()

	expectedError := errors.New("handler failed")

	// Handler that always fails
	handler := func(ctx context.Context, event events.DomainEvent) error {
		return expectedError
	}

	publisher.Subscribe("test_event", handler)

	event := &mockDomainEvent{
		eventType: "test_event",
	}

	err := publisher.Publish(context.Background(), event)

	assert.Error(t, err)
	assert.Equal(t, expectedError, err)
}

func TestPublisher_Publish_MixedHandlerResults(t *testing.T) {
	publisher := createTestPublisher()

	expectedError := errors.New("handler 2 failed")

	// Mix of successful and failing handlers
	handler1 := func(ctx context.Context, event events.DomainEvent) error {
		return nil // Success
	}
	handler2 := func(ctx context.Context, event events.DomainEvent) error {
		return expectedError // Failure
	}
	handler3 := func(ctx context.Context, event events.DomainEvent) error {
		return nil // Success
	}

	publisher.Subscribe("test_event", handler1)
	publisher.Subscribe("test_event", handler2)
	publisher.Subscribe("test_event", handler3)

	event := &mockDomainEvent{
		eventType: "test_event",
	}

	err := publisher.Publish(context.Background(), event)

	// Should return the first error encountered
	assert.Error(t, err)
	assert.Equal(t, expectedError, err)
}

func TestPublisher_Publish_ConcurrentExecution(t *testing.T) {
	publisher := createTestPublisher()

	// Use a barrier to ensure handlers run concurrently
	const numHandlers = 5
	var startBarrier, endBarrier sync.WaitGroup
	startBarrier.Add(numHandlers)
	endBarrier.Add(numHandlers)

	var executionOrder []int
	var orderMutex sync.Mutex

	// Create handlers that wait for each other to start
	for i := 0; i < numHandlers; i++ {
		handlerID := i
		handler := func(ctx context.Context, event events.DomainEvent) error {
			startBarrier.Done()
			startBarrier.Wait() // Wait for all handlers to start

			orderMutex.Lock()
			executionOrder = append(executionOrder, handlerID)
			orderMutex.Unlock()

			endBarrier.Done()
			return nil
		}
		publisher.Subscribe("test_event", handler)
	}

	event := &mockDomainEvent{
		eventType: "test_event",
	}

	err := publisher.Publish(context.Background(), event)

	assert.NoError(t, err)
	assert.Equal(t, numHandlers, len(executionOrder))

	// All handlers should have executed (order doesn't matter due to concurrency)
	handlersSeen := make(map[int]bool)
	for _, id := range executionOrder {
		handlersSeen[id] = true
	}
	assert.Equal(t, numHandlers, len(handlersSeen))
}

func TestPublisher_PublishAsync_Success(t *testing.T) {
	publisher := createTestPublisher()

	var handlerCalled int32
	var receivedEvent events.DomainEvent
	var wg sync.WaitGroup
	wg.Add(1)

	handler := func(ctx context.Context, event events.DomainEvent) error {
		defer wg.Done()
		atomic.AddInt32(&handlerCalled, 1)
		receivedEvent = event
		return nil
	}

	publisher.Subscribe("test_event", handler)

	event := &mockDomainEvent{
		eventType: "test_event",
	}

	// PublishAsync should return immediately
	publisher.PublishAsync(context.Background(), event)

	// Wait for async handler to complete
	wg.Wait()

	assert.Equal(t, int32(1), atomic.LoadInt32(&handlerCalled))
	assert.Equal(t, event, receivedEvent)
}

func TestPublisher_PublishAsync_HandlerError(t *testing.T) {
	publisher := createTestPublisher()

	var handlerCalled int32
	var wg sync.WaitGroup
	wg.Add(1)

	handler := func(ctx context.Context, event events.DomainEvent) error {
		defer wg.Done()
		atomic.AddInt32(&handlerCalled, 1)
		return errors.New("async handler failed")
	}

	publisher.Subscribe("test_event", handler)

	event := &mockDomainEvent{
		eventType: "test_event",
	}

	// PublishAsync should not return error even if handler fails
	publisher.PublishAsync(context.Background(), event)

	// Wait for async handler to complete
	wg.Wait()

	assert.Equal(t, int32(1), atomic.LoadInt32(&handlerCalled))
}

func TestPublisher_GetHandlerCount(t *testing.T) {
	publisher := createTestPublisher()

	// Initially no handlers
	assert.Equal(t, 0, publisher.GetHandlerCount("test_event"))

	// Add handlers
	handler1 := func(ctx context.Context, event events.DomainEvent) error { return nil }
	handler2 := func(ctx context.Context, event events.DomainEvent) error { return nil }

	publisher.Subscribe("test_event", handler1)
	assert.Equal(t, 1, publisher.GetHandlerCount("test_event"))

	publisher.Subscribe("test_event", handler2)
	assert.Equal(t, 2, publisher.GetHandlerCount("test_event"))

	// Different event type should have 0 handlers
	assert.Equal(t, 0, publisher.GetHandlerCount("other_event"))
}

func TestPublisher_ThreadSafety(t *testing.T) {
	publisher := createTestPublisher()

	// Test concurrent subscribe and publish operations
	const numGoroutines = 10
	const numOperations = 100

	var wg sync.WaitGroup

	// Concurrent subscribes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				handler := func(ctx context.Context, event events.DomainEvent) error {
					return nil
				}
				publisher.Subscribe("concurrent_event", handler)
			}
		}(i)
	}

	// Concurrent publishes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				event := &mockDomainEvent{
					eventType: "concurrent_event",
				}
				_ = publisher.Publish(context.Background(), event)
			}
		}()
	}

	// Wait for all operations to complete
	wg.Wait()

	// Verify final state is consistent
	finalCount := publisher.GetHandlerCount("concurrent_event")
	expectedCount := numGoroutines * numOperations
	assert.Equal(t, expectedCount, finalCount)
}

func TestPublisher_ContextCancellation(t *testing.T) {
	publisher := createTestPublisher()

	// Handler that respects context cancellation
	handlerStarted := make(chan struct{})
	handlerFinished := make(chan struct{})

	handler := func(ctx context.Context, event events.DomainEvent) error {
		close(handlerStarted)
		select {
		case <-ctx.Done():
			close(handlerFinished)
			return ctx.Err()
		case <-time.After(1 * time.Second):
			close(handlerFinished)
			return nil
		}
	}

	publisher.Subscribe("cancel_test", handler)

	ctx, cancel := context.WithCancel(context.Background())

	event := &mockDomainEvent{
		eventType: "cancel_test",
	}

	// Start publishing in goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- publisher.Publish(ctx, event)
	}()

	// Wait for handler to start, then cancel
	<-handlerStarted
	cancel()

	// Wait for handler to finish
	<-handlerFinished

	// Verify error was returned
	err := <-errChan
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}
