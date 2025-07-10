package appstate

import (
	"fmt"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	errorcodes "github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/logging"
)

// StateEventStore stores and retrieves state change events
type StateEventStore struct {
	events     map[string][]*StateEvent
	eventsByID map[string]*StateEvent
	mu         sync.RWMutex
	maxEvents  int
	retention  time.Duration
	logger     logging.Standards
}

// NewStateEventStore creates a new state event store
func NewStateEventStore(logger logging.Standards) *StateEventStore {
	store := &StateEventStore{
		events:     make(map[string][]*StateEvent),
		eventsByID: make(map[string]*StateEvent),
		maxEvents:  1000,
		retention:  24 * time.Hour,
		logger:     logger.WithComponent("state_event_store"),
	}

	go store.cleanupRoutine()

	return store
}

// StoreEvent stores a state change event
func (s *StateEventStore) StoreEvent(event *StateEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := s.makeKey(event.StateType, event.StateID)

	if _, exists := s.events[key]; !exists {
		s.events[key] = make([]*StateEvent, 0)
	}
	s.events[key] = append(s.events[key], event)

	if len(s.events[key]) > s.maxEvents {
		removeCount := len(s.events[key]) - s.maxEvents
		for i := 0; i < removeCount; i++ {
			delete(s.eventsByID, s.events[key][i].ID)
		}
		s.events[key] = s.events[key][removeCount:]
	}

	s.eventsByID[event.ID] = event

	s.logger.Debug("Stored state event",
		"event_id", event.ID,
		"event_type", string(event.Type),
		"state_type", string(event.StateType),
		"state_id", event.StateID)
}

// GetEvents retrieves events for a specific state
func (s *StateEventStore) GetEvents(stateType StateType, stateID string, limit int) ([]*StateEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := s.makeKey(stateType, stateID)
	events, exists := s.events[key]
	if !exists {
		return []*StateEvent{}, nil
	}

	start := len(events) - limit
	if start < 0 {
		start = 0
	}

	result := make([]*StateEvent, len(events)-start)
	copy(result, events[start:])

	return result, nil
}

// GetEventByID retrieves an event by its ID
func (s *StateEventStore) GetEventByID(eventID string) (*StateEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	event, exists := s.eventsByID[eventID]
	if !exists {
		return nil, errors.NewError().
			Code(errorcodes.VALIDATION_FAILED).
			Message(fmt.Sprintf("Event not found: %s", eventID)).
			Context("event_id", eventID).
			Context("component", "state_event_store").
			Suggestion("Check event ID or search for events by state type and ID").
			Build()
	}

	return event, nil
}

// GetEventsSince retrieves all events since a given timestamp
func (s *StateEventStore) GetEventsSince(since time.Time) ([]*StateEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*StateEvent, 0)
	for _, eventList := range s.events {
		for _, event := range eventList {
			if event.Timestamp.After(since) {
				result = append(result, event)
			}
		}
	}

	return result, nil
}

// GetEventsByType retrieves events of a specific type
func (s *StateEventStore) GetEventsByType(eventType StateEventType, limit int) ([]*StateEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*StateEvent, 0)
	for _, eventList := range s.events {
		for _, event := range eventList {
			if event.Type == eventType {
				result = append(result, event)
				if len(result) >= limit {
					return result, nil
				}
			}
		}
	}

	return result, nil
}

// makeKey creates a key for the events map
func (s *StateEventStore) makeKey(stateType StateType, stateID string) string {
	return fmt.Sprintf("%s:%s", stateType, stateID)
}

// cleanupRoutine periodically removes old events
func (s *StateEventStore) cleanupRoutine() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		s.cleanup()
	}
}

// cleanup removes events older than retention period
func (s *StateEventStore) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-s.retention)
	removedCount := 0

	for key, eventList := range s.events {
		keepIndex := 0
		for i, event := range eventList {
			if event.Timestamp.After(cutoff) {
				keepIndex = i
				break
			}
		}

		for i := 0; i < keepIndex; i++ {
			delete(s.eventsByID, eventList[i].ID)
			removedCount++
		}

		if keepIndex > 0 {
			s.events[key] = eventList[keepIndex:]
		}

		if len(s.events[key]) == 0 {
			delete(s.events, key)
		}
	}

	if removedCount > 0 {
		s.logger.Info("Cleaned up old state events", "removed_count", removedCount)
	}
}
