package orchestration

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// DefaultCommunicationBridge implements the CommunicationBridge interface
type DefaultCommunicationBridge struct {
	logger              zerolog.Logger
	messageHandlers     map[string]MessageHandler
	messageQueue        map[string][]ToolMessage // toolName -> messages
	deliveryAttempts    map[string]int           // messageID -> attempts
	maxDeliveryAttempts int
	mutex               sync.RWMutex
	subscribers         map[string][]MessageSubscriber
	eventStream         chan BridgeEvent
	ctx                 context.Context
	cancel              context.CancelFunc
}

// MessageSubscriber represents a subscriber to tool messages
type MessageSubscriber struct {
	ID      string
	Filter  MessageFilter
	Handler MessageHandler
	Options SubscriberOptions
}

// MessageFilter defines criteria for message filtering
type MessageFilter struct {
	SourceTool  string
	TargetTool  string
	MessageType string
	Pattern     string
}

// SubscriberOptions configures message subscription
type SubscriberOptions struct {
	BufferSize   int
	RetryOnError bool
	MaxRetries   int
	RetryDelay   time.Duration
	DeadLetter   bool
}

// BridgeEvent represents events in the communication bridge
type BridgeEvent struct {
	Type      string      `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// NewDefaultCommunicationBridge creates a new communication bridge
func NewDefaultCommunicationBridge(logger zerolog.Logger) *DefaultCommunicationBridge {
	ctx, cancel := context.WithCancel(context.Background())

	bridge := &DefaultCommunicationBridge{
		logger:              logger.With().Str("component", "communication_bridge").Logger(),
		messageHandlers:     make(map[string]MessageHandler),
		messageQueue:        make(map[string][]ToolMessage),
		deliveryAttempts:    make(map[string]int),
		maxDeliveryAttempts: 3,
		subscribers:         make(map[string][]MessageSubscriber),
		eventStream:         make(chan BridgeEvent, 100),
		ctx:                 ctx,
		cancel:              cancel,
	}

	// Start background processing
	go bridge.processEvents()

	return bridge
}

// SendMessage sends a message from one tool to another
func (dcb *DefaultCommunicationBridge) SendMessage(ctx context.Context, from, to string, message ToolMessage) error {
	dcb.mutex.Lock()
	defer dcb.mutex.Unlock()

	// Validate message
	if err := dcb.validateMessage(message); err != nil {
		return fmt.Errorf("invalid message: %w", err)
	}

	// Set metadata
	message.From = from
	message.To = to
	if message.Timestamp.IsZero() {
		message.Timestamp = time.Now()
	}

	dcb.logger.Debug().
		Str("from", from).
		Str("to", to).
		Str("message_id", message.ID).
		Str("type", message.Type).
		Msg("Sending message")

	// Check if target tool has a registered handler
	if handler, exists := dcb.messageHandlers[to]; exists {
		// Direct delivery
		if err := dcb.deliverMessage(ctx, handler, message); err != nil {
			dcb.logger.Error().
				Err(err).
				Str("from", from).
				Str("to", to).
				Str("message_id", message.ID).
				Msg("Direct message delivery failed")

			// Queue for retry
			dcb.queueMessage(to, message)
		} else {
			// Publish successful delivery event
			dcb.publishEvent(BridgeEvent{
				Type:      "message_delivered",
				Timestamp: time.Now(),
				Data: map[string]interface{}{
					"message": message,
					"method":  "direct",
				},
			})
		}
	} else {
		// Queue message for later delivery
		dcb.queueMessage(to, message)

		dcb.logger.Debug().
			Str("to", to).
			Str("message_id", message.ID).
			Msg("Tool handler not registered, message queued")
	}

	// Notify subscribers
	dcb.notifySubscribers(message)

	return nil
}

// RegisterHandler registers a message handler for a tool
func (dcb *DefaultCommunicationBridge) RegisterHandler(toolName string, handler MessageHandler) error {
	dcb.mutex.Lock()
	defer dcb.mutex.Unlock()

	dcb.messageHandlers[toolName] = handler

	dcb.logger.Info().
		Str("tool_name", toolName).
		Msg("Message handler registered")

	// Process any queued messages
	if messages, exists := dcb.messageQueue[toolName]; exists {
		go dcb.processQueuedMessages(toolName, messages, handler)
		delete(dcb.messageQueue, toolName)
	}

	return nil
}

// GetPendingMessages retrieves pending messages for a tool
func (dcb *DefaultCommunicationBridge) GetPendingMessages(toolName string) ([]ToolMessage, error) {
	dcb.mutex.RLock()
	defer dcb.mutex.RUnlock()

	messages, exists := dcb.messageQueue[toolName]
	if !exists {
		return []ToolMessage{}, nil
	}

	// Return copy of messages
	result := make([]ToolMessage, len(messages))
	copy(result, messages)

	return result, nil
}

// Subscribe subscribes to messages with filtering
func (dcb *DefaultCommunicationBridge) Subscribe(subscriber MessageSubscriber) error {
	dcb.mutex.Lock()
	defer dcb.mutex.Unlock()

	key := dcb.getSubscriptionKey(subscriber.Filter)
	dcb.subscribers[key] = append(dcb.subscribers[key], subscriber)

	dcb.logger.Info().
		Str("subscriber_id", subscriber.ID).
		Str("filter_key", key).
		Msg("Message subscriber registered")

	return nil
}

// Unsubscribe removes a message subscriber
func (dcb *DefaultCommunicationBridge) Unsubscribe(subscriberID string) error {
	dcb.mutex.Lock()
	defer dcb.mutex.Unlock()

	for key, subscribers := range dcb.subscribers {
		for i, sub := range subscribers {
			if sub.ID == subscriberID {
				// Remove subscriber
				dcb.subscribers[key] = append(subscribers[:i], subscribers[i+1:]...)

				dcb.logger.Info().
					Str("subscriber_id", subscriberID).
					Msg("Message subscriber removed")

				return nil
			}
		}
	}

	return fmt.Errorf("subscriber %s not found", subscriberID)
}

// BroadcastMessage broadcasts a message to all subscribers
func (dcb *DefaultCommunicationBridge) BroadcastMessage(ctx context.Context, message ToolMessage) error {
	dcb.mutex.RLock()
	defer dcb.mutex.RUnlock()

	message.Timestamp = time.Now()

	dcb.logger.Debug().
		Str("message_id", message.ID).
		Str("type", message.Type).
		Msg("Broadcasting message")

	// Notify all matching subscribers
	dcb.notifySubscribers(message)

	// Publish broadcast event
	dcb.publishEvent(BridgeEvent{
		Type:      "message_broadcast",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"message": message,
		},
	})

	return nil
}

// GetEventStream returns the event stream channel
func (dcb *DefaultCommunicationBridge) GetEventStream() <-chan BridgeEvent {
	return dcb.eventStream
}

// Close shuts down the communication bridge
func (dcb *DefaultCommunicationBridge) Close() error {
	dcb.cancel()
	close(dcb.eventStream)
	return nil
}

// validateMessage validates a tool message
func (dcb *DefaultCommunicationBridge) validateMessage(message ToolMessage) error {
	if message.ID == "" {
		return fmt.Errorf("message ID is required")
	}
	if message.Type == "" {
		return fmt.Errorf("message type is required")
	}
	return nil
}

// deliverMessage delivers a message to a handler
func (dcb *DefaultCommunicationBridge) deliverMessage(ctx context.Context, handler MessageHandler, message ToolMessage) error {
	// Create timeout context
	deliveryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Track delivery attempts
	dcb.deliveryAttempts[message.ID]++

	// Deliver message
	if err := handler(deliveryCtx, message); err != nil {
		return fmt.Errorf("handler failed: %w", err)
	}

	// Remove from delivery attempts tracking
	delete(dcb.deliveryAttempts, message.ID)

	return nil
}

// queueMessage queues a message for later delivery
func (dcb *DefaultCommunicationBridge) queueMessage(toolName string, message ToolMessage) {
	if dcb.messageQueue[toolName] == nil {
		dcb.messageQueue[toolName] = []ToolMessage{}
	}
	dcb.messageQueue[toolName] = append(dcb.messageQueue[toolName], message)

	// Limit queue size to prevent memory issues
	maxQueueSize := 100
	if len(dcb.messageQueue[toolName]) > maxQueueSize {
		// Remove oldest message
		dcb.messageQueue[toolName] = dcb.messageQueue[toolName][1:]

		dcb.logger.Warn().
			Str("tool_name", toolName).
			Int("queue_size", len(dcb.messageQueue[toolName])).
			Msg("Message queue size limit exceeded, dropped oldest message")
	}
}

// processQueuedMessages processes queued messages for a tool
func (dcb *DefaultCommunicationBridge) processQueuedMessages(toolName string, messages []ToolMessage, handler MessageHandler) {
	ctx := context.Background()

	for _, message := range messages {
		if err := dcb.deliverMessage(ctx, handler, message); err != nil {
			dcb.logger.Error().
				Err(err).
				Str("tool_name", toolName).
				Str("message_id", message.ID).
				Msg("Failed to deliver queued message")

			// Check delivery attempts
			if dcb.deliveryAttempts[message.ID] >= dcb.maxDeliveryAttempts {
				dcb.logger.Error().
					Str("message_id", message.ID).
					Int("attempts", dcb.deliveryAttempts[message.ID]).
					Msg("Message delivery failed after max attempts, dropping")

				delete(dcb.deliveryAttempts, message.ID)

				// Publish dead letter event
				dcb.publishEvent(BridgeEvent{
					Type:      "message_dead_letter",
					Timestamp: time.Now(),
					Data: map[string]interface{}{
						"message": message,
						"reason":  "max_delivery_attempts_exceeded",
					},
				})
			} else {
				// Re-queue for retry
				dcb.mutex.Lock()
				dcb.queueMessage(toolName, message)
				dcb.mutex.Unlock()
			}
		} else {
			dcb.publishEvent(BridgeEvent{
				Type:      "message_delivered",
				Timestamp: time.Now(),
				Data: map[string]interface{}{
					"message": message,
					"method":  "queued",
				},
			})
		}
	}
}

// notifySubscribers notifies relevant subscribers about a message
func (dcb *DefaultCommunicationBridge) notifySubscribers(message ToolMessage) {
	for _, subscribers := range dcb.subscribers {
		for _, subscriber := range subscribers {
			if dcb.messageMatchesFilter(message, subscriber.Filter) {
				go dcb.notifySubscriber(subscriber, message)
			}
		}
	}
}

// notifySubscriber notifies a single subscriber
func (dcb *DefaultCommunicationBridge) notifySubscriber(subscriber MessageSubscriber, message ToolMessage) {
	ctx := context.Background()

	var err error
	attempts := 0
	maxAttempts := subscriber.Options.MaxRetries + 1

	for attempts < maxAttempts {
		err = subscriber.Handler(ctx, message)
		if err == nil {
			// Success
			return
		}

		attempts++
		if !subscriber.Options.RetryOnError || attempts >= maxAttempts {
			break
		}

		// Wait before retry
		time.Sleep(subscriber.Options.RetryDelay)
	}

	dcb.logger.Error().
		Err(err).
		Str("subscriber_id", subscriber.ID).
		Str("message_id", message.ID).
		Int("attempts", attempts).
		Msg("Failed to notify subscriber")

	// Send to dead letter if configured
	if subscriber.Options.DeadLetter {
		dcb.publishEvent(BridgeEvent{
			Type:      "subscriber_dead_letter",
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"subscriber_id": subscriber.ID,
				"message":       message,
				"error":         err.Error(),
				"attempts":      attempts,
			},
		})
	}
}

// messageMatchesFilter checks if a message matches a filter
func (dcb *DefaultCommunicationBridge) messageMatchesFilter(message ToolMessage, filter MessageFilter) bool {
	if filter.SourceTool != "" && filter.SourceTool != message.From {
		return false
	}
	if filter.TargetTool != "" && filter.TargetTool != message.To {
		return false
	}
	if filter.MessageType != "" && filter.MessageType != message.Type {
		return false
	}
	if filter.Pattern != "" {
		// Simple pattern matching
		if payloadStr, ok := message.Payload.(string); ok {
			if !contains(payloadStr, filter.Pattern) {
				return false
			}
		}
	}
	return true
}

// getSubscriptionKey generates a key for subscription filtering
func (dcb *DefaultCommunicationBridge) getSubscriptionKey(filter MessageFilter) string {
	return fmt.Sprintf("%s:%s:%s", filter.SourceTool, filter.TargetTool, filter.MessageType)
}

// publishEvent publishes an event to the event stream
func (dcb *DefaultCommunicationBridge) publishEvent(event BridgeEvent) {
	select {
	case dcb.eventStream <- event:
	default:
		// Event stream full, drop event
		dcb.logger.Warn().
			Str("event_type", event.Type).
			Msg("Event stream full, dropping event")
	}
}

// processEvents processes events in the background
func (dcb *DefaultCommunicationBridge) processEvents() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-dcb.ctx.Done():
			return
		case <-ticker.C:
			// Periodic maintenance
			dcb.performMaintenance()
		}
	}
}

// performMaintenance performs periodic maintenance tasks
func (dcb *DefaultCommunicationBridge) performMaintenance() {
	dcb.mutex.Lock()
	defer dcb.mutex.Unlock()

	// Clean up old delivery attempts
	for messageID, attempts := range dcb.deliveryAttempts {
		// This is a simple cleanup - in a real implementation,
		// you'd track message timestamps
		if attempts > dcb.maxDeliveryAttempts {
			delete(dcb.deliveryAttempts, messageID)
		}
	}

	// Report queue sizes
	totalQueued := 0
	for toolName, messages := range dcb.messageQueue {
		queueSize := len(messages)
		totalQueued += queueSize

		if queueSize > 10 {
			dcb.logger.Warn().
				Str("tool_name", toolName).
				Int("queue_size", queueSize).
				Msg("Large message queue detected")
		}
	}

	if totalQueued > 0 {
		dcb.logger.Debug().
			Int("total_queued", totalQueued).
			Int("tools_with_queue", len(dcb.messageQueue)).
			Msg("Message queue status")
	}
}

// GetStats returns communication bridge statistics
func (dcb *DefaultCommunicationBridge) GetStats() map[string]interface{} {
	dcb.mutex.RLock()
	defer dcb.mutex.RUnlock()

	stats := map[string]interface{}{
		"registered_handlers": len(dcb.messageHandlers),
		"active_subscribers":  dcb.countSubscribers(),
		"queued_messages":     dcb.countQueuedMessages(),
		"delivery_attempts":   len(dcb.deliveryAttempts),
	}

	// Per-tool queue sizes
	queueSizes := make(map[string]int)
	for toolName, messages := range dcb.messageQueue {
		queueSizes[toolName] = len(messages)
	}
	stats["queue_sizes"] = queueSizes

	return stats
}

// countSubscribers counts total subscribers
func (dcb *DefaultCommunicationBridge) countSubscribers() int {
	total := 0
	for _, subscribers := range dcb.subscribers {
		total += len(subscribers)
	}
	return total
}

// countQueuedMessages counts total queued messages
func (dcb *DefaultCommunicationBridge) countQueuedMessages() int {
	total := 0
	for _, messages := range dcb.messageQueue {
		total += len(messages)
	}
	return total
}
