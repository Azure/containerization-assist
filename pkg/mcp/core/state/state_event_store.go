package state

import (
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// StateEventStore stores and retrieves state change events
type StateEventStore struct {
	events     map[string][]*StateEvent // key: stateType:stateID
	eventsByID map[string]*StateEvent
	mu         sync.RWMutex
	maxEvents  int
	retention  time.Duration
	logger     zerolog.Logger
}

// NewStateEventStore creates a new state event store
func NewStateEventStore(logger zerolog.Logger) *StateEventStore {
	store := &StateEventStore{
		events:     make(map[string][]*StateEvent),
		eventsByID: make(map[string]*StateEvent),
		maxEvents:  1000, // Keep last 1000 events per state
		retention:  24 * time.Hour,
		logger:     logger.With().Str("component", "state_event_store").Logger(),
	}

	// Start cleanup routine
	go store.cleanupRoutine()

	return store
}

// StoreEvent stores a state change event
func (s *StateEventStore) StoreEvent(event *StateEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := s.makeKey(event.StateType, event.StateID)

	// Add to events list
	if _, exists := s.events[key]; !exists {
		s.events[key] = make([]*StateEvent, 0)
	}
	s.events[key] = append(s.events[key], event)

	// Trim if too many events
	if len(s.events[key]) > s.maxEvents {
		// Remove oldest events
		removeCount := len(s.events[key]) - s.maxEvents
		for i := 0; i < removeCount; i++ {
			delete(s.eventsByID, s.events[key][i].ID)
		}
		s.events[key] = s.events[key][removeCount:]
	}

	// Add to ID index
	s.eventsByID[event.ID] = event

	s.logger.Debug().
		Str("event_id", event.ID).
		Str("event_type", string(event.Type)).
		Str("state_type", string(event.StateType)).
		Str("state_id", event.StateID).
		Msg("Stored state event")
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

	// Return most recent events up to limit
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
		return nil, fmt.Errorf("event not found: %s", eventID)
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
		// Find first event that should be kept
		keepIndex := 0
		for i, event := range eventList {
			if event.Timestamp.After(cutoff) {
				keepIndex = i
				break
			}
		}

		// Remove old events from ID index
		for i := 0; i < keepIndex; i++ {
			delete(s.eventsByID, eventList[i].ID)
			removedCount++
		}

		// Update event list
		if keepIndex > 0 {
			s.events[key] = eventList[keepIndex:]
		}

		// Remove key if no events left
		if len(s.events[key]) == 0 {
			delete(s.events, key)
		}
	}

	if removedCount > 0 {
		s.logger.Info().Int("removed_count", removedCount).Msg("Cleaned up old state events")
	}
}
