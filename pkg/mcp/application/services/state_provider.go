package services

import (
	"context"
	"fmt"

	"github.com/Azure/container-kit/pkg/mcp/application/state"
)

// StateProvider provides state observation and subscription capabilities
type StateProvider interface {
	// GetCurrent returns the current application state
	GetCurrent(ctx context.Context) (*ApplicationState, error)

	// Subscribe registers a callback for state changes
	Subscribe(ctx context.Context, callback StateChangeCallback) error

	// Checkpoint creates a state checkpoint
	Checkpoint(ctx context.Context) error

	// GetHistory returns state history for a key
	GetHistory(ctx context.Context, key string) ([]StateHistoryEntry, error)
}

// ApplicationState represents the current state of the application
type ApplicationState struct {
	ConversationStates map[string]interface{} `json:"conversation_states"`
	WorkflowSessions   map[string]interface{} `json:"workflow_sessions"`
	ActiveTools        []string               `json:"active_tools"`
	LastCheckpoint     string                 `json:"last_checkpoint"`
}

// StateChangeCallback is called when state changes
type StateChangeCallback func(key string, oldState, newState interface{})

// StateHistoryEntry represents a historical state entry
type StateHistoryEntry struct {
	Timestamp string      `json:"timestamp"`
	Key       string      `json:"key"`
	State     interface{} `json:"state"`
	Action    string      `json:"action"`
}

// stateProvider implements StateProvider
type stateProvider struct {
	stateManager *state.UnifiedStateManager
	callbacks    []StateChangeCallback
}

// NewStateProvider creates a new StateProvider service
func NewStateProvider(stateManager *state.UnifiedStateManager) StateProvider {
	return &stateProvider{
		stateManager: stateManager,
		callbacks:    make([]StateChangeCallback, 0),
	}
}

func (s *stateProvider) GetCurrent(_ context.Context) (*ApplicationState, error) {
	// Get current state from UnifiedStateManager
	appState := &ApplicationState{
		ConversationStates: make(map[string]interface{}),
		WorkflowSessions:   make(map[string]interface{}),
		ActiveTools:        []string{},
		LastCheckpoint:     "",
	}

	// Populate from state manager
	// This would need actual implementation based on UnifiedStateManager methods

	return appState, nil
}

func (s *stateProvider) Subscribe(_ context.Context, callback StateChangeCallback) error {
	s.callbacks = append(s.callbacks, callback)

	// Register with UnifiedStateManager's observer pattern
	observer := &callbackObserver{
		callback: callback,
		id:       fmt.Sprintf("callback_%d", len(s.callbacks)),
	}
	s.stateManager.RegisterObserver(observer)

	return nil
}

func (s *stateProvider) Checkpoint(_ context.Context) error {
	// UnifiedStateManager doesn't have CreateCheckpoint method
	// This would need to be implemented based on the actual checkpoint mechanism
	return nil
}

func (s *stateProvider) GetHistory(ctx context.Context, key string) ([]StateHistoryEntry, error) {
	// Get history from UnifiedStateManager
	// GetStateHistory needs context, state type, state ID and limit
	events, err := s.stateManager.GetStateHistory(ctx, state.StateTypeGlobal, key, 100)
	if err != nil {
		return nil, err
	}

	entries := make([]StateHistoryEntry, 0, len(events))
	for _, event := range events {
		entries = append(entries, StateHistoryEntry{
			Timestamp: event.Timestamp.Format("2006-01-02T15:04:05Z"),
			Key:       event.StateID,
			State:     event.NewValue,
			Action:    string(event.Type),
		})
	}

	return entries, nil
}

// callbackObserver adapts StateChangeCallback to StateObserver interface
type callbackObserver struct {
	callback StateChangeCallback
	id       string
}

func (c *callbackObserver) OnStateChange(event *state.StateEvent) error {
	if event.EventType == state.StateEventUpdated {
		c.callback(event.StateID, event.OldValue, event.NewValue)
	}
	return nil
}

func (c *callbackObserver) GetID() string {
	if c.id == "" {
		c.id = fmt.Sprintf("callback_observer_%p", c)
	}
	return c.id
}

func (c *callbackObserver) IsActive() bool {
	return true
}
