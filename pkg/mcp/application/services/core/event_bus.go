package core

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// EventType represents the type of event
type EventType string

const (
	EventTypeToolRequestStarted   EventType = "tool_request_started"
	EventTypeToolRequestCompleted EventType = "tool_request_completed"
	EventTypeToolRequestFailed    EventType = "tool_request_failed"
	EventTypeWorkflowStarted      EventType = "workflow_started"
	EventTypeWorkflowCompleted    EventType = "workflow_completed"
	EventTypeWorkflowFailed       EventType = "workflow_failed"
	EventTypeStageStarted         EventType = "stage_started"
	EventTypeStageCompleted       EventType = "stage_completed"
	EventTypeStageFailed          EventType = "stage_failed"
	EventTypeContextShared        EventType = "context_shared"
	EventTypeContextCleared       EventType = "context_cleared"
	EventTypeCircuitBreakerOpened EventType = "circuit_breaker_opened"
	EventTypeCircuitBreakerClosed EventType = "circuit_breaker_closed"
)

// Event represents an event in the system
type Event struct {
	ID        string                 `json:"id"`
	Type      EventType              `json:"type"`
	Source    string                 `json:"source"`
	Data      map[string]interface{} `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
	SessionID string                 `json:"session_id,omitempty"`
}

// EventHandler is a function that handles events
type EventHandler func(ctx context.Context, event Event) error

// EventSubscription represents a subscription to events
type EventSubscription struct {
	ID           string       `json:"id"`
	EventType    EventType    `json:"event_type"`
	Handler      EventHandler `json:"-"`
	CreatedAt    time.Time    `json:"created_at"`
	Active       bool         `json:"active"`
	HandledCount int64        `json:"handled_count"`
	ErrorCount   int64        `json:"error_count"`
}

// EventBus provides pub/sub functionality for system events
type EventBus struct {
	subscriptions map[EventType][]*EventSubscription
	eventHistory  []Event
	mutex         sync.RWMutex
	logger        zerolog.Logger
	maxHistory    int
	closed        bool
	workers       int
	eventChan     chan Event
	workerCtx     context.Context
	workerCancel  context.CancelFunc
	wg            sync.WaitGroup
}

// NewEventBus creates a new event bus
func NewEventBus(logger zerolog.Logger) *EventBus {
	ctx, cancel := context.WithCancel(context.Background())

	bus := &EventBus{
		subscriptions: make(map[EventType][]*EventSubscription),
		eventHistory:  make([]Event, 0),
		logger:        logger.With().Str("component", "event_bus").Logger(),
		maxHistory:    1000,
		workers:       5,
		eventChan:     make(chan Event, 100),
		workerCtx:     ctx,
		workerCancel:  cancel,
	}

	bus.startWorkers()

	return bus
}

// startWorkers starts event processing workers
func (eb *EventBus) startWorkers() {
	for i := 0; i < eb.workers; i++ {
		eb.wg.Add(1)
		go eb.worker(i)
	}
}

// worker processes events from the event channel
func (eb *EventBus) worker(workerID int) {
	defer eb.wg.Done()

	eb.logger.Debug().Int("worker_id", workerID).Msg("Event bus worker started")

	for {
		select {
		case <-eb.workerCtx.Done():
			eb.logger.Debug().Int("worker_id", workerID).Msg("Event bus worker stopping")
			return
		case event := <-eb.eventChan:
			eb.processEvent(workerID, event)
		}
	}
}

// processEvent processes a single event
func (eb *EventBus) processEvent(workerID int, event Event) {
	eb.mutex.RLock()
	subscriptions := eb.subscriptions[event.Type]
	eb.mutex.RUnlock()

	if len(subscriptions) == 0 {
		return
	}

	eb.logger.Debug().
		Int("worker_id", workerID).
		Str("event_id", event.ID).
		Str("event_type", string(event.Type)).
		Int("subscribers", len(subscriptions)).
		Msg("Processing event")

	for _, subscription := range subscriptions {
		if !subscription.Active {
			continue
		}

		handlerCtx, cancel := context.WithTimeout(eb.workerCtx, 30*time.Second)
		if err := subscription.Handler(handlerCtx, event); err != nil {
			eb.mutex.Lock()
			subscription.ErrorCount++
			eb.mutex.Unlock()

			eb.logger.Error().
				Err(err).
				Str("subscription_id", subscription.ID).
				Str("event_id", event.ID).
				Str("event_type", string(event.Type)).
				Msg("Event handler failed")
		} else {
			eb.mutex.Lock()
			subscription.HandledCount++
			eb.mutex.Unlock()
		}

		cancel()
	}
}

// Publish publishes an event to all subscribers
func (eb *EventBus) Publish(eventType EventType, data map[string]interface{}) {
	if eb.closed {
		eb.logger.Warn().Str("event_type", string(eventType)).Msg("Cannot publish event - event bus is closed")
		return
	}

	event := Event{
		ID:        eb.generateEventID(),
		Type:      eventType,
		Source:    "communication_manager",
		Data:      data,
		Timestamp: time.Now(),
	}

	if sessionID, ok := data["session_id"].(string); ok {
		event.SessionID = sessionID
	}
	eb.mutex.Lock()
	eb.eventHistory = append(eb.eventHistory, event)
	if len(eb.eventHistory) > eb.maxHistory {
		eb.eventHistory = eb.eventHistory[1:]
	}
	eb.mutex.Unlock()

	eb.logger.Debug().
		Str("event_id", event.ID).
		Str("event_type", string(eventType)).
		Msg("Publishing event")

	select {
	case eb.eventChan <- event:
	default:
		eb.logger.Warn().
			Str("event_id", event.ID).
			Str("event_type", string(eventType)).
			Msg("Event channel full - dropping event")
	}
}

// Subscribe subscribes to events of a specific type
func (eb *EventBus) Subscribe(eventType EventType, handler EventHandler) string {
	if eb.closed {
		eb.logger.Warn().Str("event_type", string(eventType)).Msg("Cannot subscribe - event bus is closed")
		return ""
	}

	subscription := &EventSubscription{
		ID:        eb.generateSubscriptionID(),
		EventType: eventType,
		Handler:   handler,
		CreatedAt: time.Now(),
		Active:    true,
	}

	eb.mutex.Lock()
	eb.subscriptions[eventType] = append(eb.subscriptions[eventType], subscription)
	eb.mutex.Unlock()

	eb.logger.Info().
		Str("subscription_id", subscription.ID).
		Str("event_type", string(eventType)).
		Msg("New event subscription created")

	return subscription.ID
}

// Unsubscribe removes a subscription
func (eb *EventBus) Unsubscribe(subscriptionID string) bool {
	eb.mutex.Lock()
	defer eb.mutex.Unlock()

	for eventType, subscriptions := range eb.subscriptions {
		for i, subscription := range subscriptions {
			if subscription.ID == subscriptionID {
				subscription.Active = false
				eb.subscriptions[eventType] = append(subscriptions[:i], subscriptions[i+1:]...)

				eb.logger.Info().
					Str("subscription_id", subscriptionID).
					Str("event_type", string(eventType)).
					Msg("Event subscription removed")
				return true
			}
		}
	}

	return false
}

// GetEventHistory returns recent event history
func (eb *EventBus) GetEventHistory(limit int) []Event {
	eb.mutex.RLock()
	defer eb.mutex.RUnlock()

	if limit <= 0 || limit > len(eb.eventHistory) {
		result := make([]Event, len(eb.eventHistory))
		copy(result, eb.eventHistory)
		return result
	}

	start := len(eb.eventHistory) - limit
	result := make([]Event, limit)
	copy(result, eb.eventHistory[start:])
	return result
}

// GetEventsByType returns events of a specific type from history
func (eb *EventBus) GetEventsByType(eventType EventType, limit int) []Event {
	eb.mutex.RLock()
	defer eb.mutex.RUnlock()

	var results []Event
	count := 0

	for i := len(eb.eventHistory) - 1; i >= 0 && (limit <= 0 || count < limit); i-- {
		if eb.eventHistory[i].Type == eventType {
			results = append([]Event{eb.eventHistory[i]}, results...)
			count++
		}
	}

	return results
}

// GetSubscriptions returns current subscriptions
func (eb *EventBus) GetSubscriptions() map[EventType][]*EventSubscription {
	eb.mutex.RLock()
	defer eb.mutex.RUnlock()

	result := make(map[EventType][]*EventSubscription)
	for eventType, subscriptions := range eb.subscriptions {
		result[eventType] = make([]*EventSubscription, len(subscriptions))
		copy(result[eventType], subscriptions)
	}

	return result
}

// GetSubscriptionStats returns subscription statistics
func (eb *EventBus) GetSubscriptionStats() map[EventType]SubscriptionStats {
	eb.mutex.RLock()
	defer eb.mutex.RUnlock()

	stats := make(map[EventType]SubscriptionStats)

	for eventType, subscriptions := range eb.subscriptions {
		stat := SubscriptionStats{
			EventType:        eventType,
			TotalSubscribers: len(subscriptions),
		}

		for _, sub := range subscriptions {
			if sub.Active {
				stat.ActiveSubscribers++
			}
			stat.TotalEventsHandled += sub.HandledCount
			stat.TotalErrors += sub.ErrorCount
		}

		stats[eventType] = stat
	}

	return stats
}

// SubscriptionStats represents statistics for event subscriptions
type SubscriptionStats struct {
	EventType          EventType `json:"event_type"`
	TotalSubscribers   int       `json:"total_subscribers"`
	ActiveSubscribers  int       `json:"active_subscribers"`
	TotalEventsHandled int64     `json:"total_events_handled"`
	TotalErrors        int64     `json:"total_errors"`
}

// PublishWorkflowEvent publishes workflow-related events
func (eb *EventBus) PublishWorkflowEvent(eventType EventType, workflowID, sessionID string, data map[string]interface{}) {
	if data == nil {
		data = make(map[string]interface{})
	}

	data["workflow_id"] = workflowID
	data["session_id"] = sessionID

	eb.Publish(eventType, data)
}

// PublishStageEvent publishes stage-related events
func (eb *EventBus) PublishStageEvent(eventType EventType, stageID, workflowID, sessionID string, data map[string]interface{}) {
	if data == nil {
		data = make(map[string]interface{})
	}

	data["stage_id"] = stageID
	data["workflow_id"] = workflowID
	data["session_id"] = sessionID

	eb.Publish(eventType, data)
}

// PublishContextEvent publishes context-related events
func (eb *EventBus) PublishContextEvent(eventType EventType, sessionID, contextType string, data map[string]interface{}) {
	if data == nil {
		data = make(map[string]interface{})
	}

	data["session_id"] = sessionID
	data["context_type"] = contextType

	eb.Publish(eventType, data)
}

// Close gracefully shuts down the event bus
func (eb *EventBus) Close() {
	eb.mutex.Lock()
	if eb.closed {
		eb.mutex.Unlock()
		return
	}
	eb.closed = true
	eb.mutex.Unlock()

	eb.logger.Info().Msg("Shutting down event bus")

	eb.workerCancel()
	close(eb.eventChan)
	eb.wg.Wait()

	eb.logger.Info().Msg("Event bus shutdown complete")
}

// generateEventID generates a unique event ID
func (eb *EventBus) generateEventID() string {
	return fmt.Sprintf("event_%d", time.Now().UnixNano())
}

// generateSubscriptionID generates a unique subscription ID
func (eb *EventBus) generateSubscriptionID() string {
	return fmt.Sprintf("sub_%d", time.Now().UnixNano())
}

// String converts event to string representation
func (e Event) String() string {
	return fmt.Sprintf("Event{ID: %s, Type: %s, Source: %s, Timestamp: %s}",
		e.ID, e.Type, e.Source, e.Timestamp.Format(time.RFC3339))
}
