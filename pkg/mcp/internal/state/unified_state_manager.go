package state

import (
	"context"

	"github.com/Azure/container-kit/pkg/mcp/core"

	crypto_rand "crypto/rand"
	"encoding/binary"
	"fmt"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/internal/session"
	"github.com/rs/zerolog"
)

// StateType represents different types of state in the system
type StateType string

const (
	StateTypeSession      StateType = "session"
	StateTypeWorkflow     StateType = "workflow"
	StateTypeConversation StateType = "conversation"
	StateTypeTool         StateType = "tool"
	StateTypeGlobal       StateType = "global"
)

// StateScope represents the scope of state
type StateScope string

const (
	StateScopeSession StateScope = "session"
	StateScopeUser    StateScope = "user"
	StateScopeGlobal  StateScope = "global"
)

// StateEvent represents a state change event
type StateEvent struct {
	ID        string
	Type      StateEventType
	StateType StateType
	StateID   string
	OldValue  interface{}
	NewValue  interface{}
	Metadata  map[string]interface{}
	Timestamp time.Time
}

// StateEventType represents types of state events
type StateEventType string

const (
	StateEventCreated   StateEventType = "created"
	StateEventUpdated   StateEventType = "updated"
	StateEventDeleted   StateEventType = "deleted"
	StateEventMigrated  StateEventType = "migrated"
	StateEventSynced    StateEventType = "synced"
	StateEventValidated StateEventType = "validated"
)

// StateObserver is notified of state changes
type StateObserver interface {
	OnStateChange(event *StateEvent)
}

// StateValidator validates state changes
type StateValidator interface {
	ValidateState(ctx context.Context, stateType StateType, state interface{}) error
}

// StateMigrator handles state migrations
type StateMigrator interface {
	MigrateState(ctx context.Context, stateType StateType, fromVersion, toVersion string, state interface{}) (interface{}, error)
}

// UnifiedStateManager manages all state in the system
type UnifiedStateManager struct {
	sessionManager  *session.SessionManager
	stateProviders  map[StateType]StateProvider
	observers       []StateObserver
	validators      map[StateType]StateValidator
	migrators       map[StateType]StateMigrator
	eventStore      *StateEventStore
	syncCoordinator *StateSyncCoordinator
	mu              sync.RWMutex
	logger          zerolog.Logger
}

// StateProvider provides access to a specific type of state
type StateProvider interface {
	GetState(ctx context.Context, id string) (interface{}, error)
	SetState(ctx context.Context, id string, state interface{}) error
	DeleteState(ctx context.Context, id string) error
	ListStates(ctx context.Context) ([]string, error)
}

// NewUnifiedStateManager creates a new unified state manager
func NewUnifiedStateManager(sessionManager *session.SessionManager, logger zerolog.Logger) *UnifiedStateManager {
	return &UnifiedStateManager{
		sessionManager:  sessionManager,
		stateProviders:  make(map[StateType]StateProvider),
		validators:      make(map[StateType]StateValidator),
		migrators:       make(map[StateType]StateMigrator),
		eventStore:      NewStateEventStore(logger),
		syncCoordinator: NewStateSyncCoordinator(logger),
		logger:          logger.With().Str("component", "unified_state_manager").Logger(),
	}
}

// RegisterStateProvider registers a provider for a state type
func (m *UnifiedStateManager) RegisterStateProvider(stateType StateType, provider StateProvider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stateProviders[stateType] = provider
	m.logger.Info().Str("state_type", string(stateType)).Msg("Registered state provider")
}

// RegisterObserver registers a state observer
func (m *UnifiedStateManager) RegisterObserver(observer StateObserver) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.observers = append(m.observers, observer)
	m.logger.Debug().Msg("Registered state observer")
}

// RegisterValidator registers a state validator
func (m *UnifiedStateManager) RegisterValidator(stateType StateType, validator StateValidator) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.validators[stateType] = validator
	m.logger.Info().Str("state_type", string(stateType)).Msg("Registered state validator")
}

// RegisterMigrator registers a state migrator
func (m *UnifiedStateManager) RegisterMigrator(stateType StateType, migrator StateMigrator) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.migrators[stateType] = migrator
	m.logger.Info().Str("state_type", string(stateType)).Msg("Registered state migrator")
}

// GetState retrieves state of a specific type
func (m *UnifiedStateManager) GetState(ctx context.Context, stateType StateType, id string) (interface{}, error) {
	m.mu.RLock()
	provider, exists := m.stateProviders[stateType]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no provider registered for state type: %s", stateType)
	}

	return provider.GetState(ctx, id)
}

// SetState updates state of a specific type
func (m *UnifiedStateManager) SetState(ctx context.Context, stateType StateType, id string, state interface{}) error {
	m.mu.RLock()
	provider, exists := m.stateProviders[stateType]
	validator := m.validators[stateType]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("no provider registered for state type: %s", stateType)
	}

	// Validate state if validator exists
	if validator != nil {
		if err := validator.ValidateState(ctx, stateType, state); err != nil {
			return fmt.Errorf("state validation failed: %w", err)
		}
	}

	// Get old state for event
	oldState, _ := provider.GetState(ctx, id)

	// Set new state
	if err := provider.SetState(ctx, id, state); err != nil {
		return err
	}

	// Notify observers
	event := &StateEvent{
		ID:        generateEventID(),
		Type:      StateEventUpdated,
		StateType: stateType,
		StateID:   id,
		OldValue:  oldState,
		NewValue:  state,
		Timestamp: time.Now(),
	}

	m.notifyObservers(event)
	m.eventStore.StoreEvent(event)

	return nil
}

// DeleteState removes state of a specific type
func (m *UnifiedStateManager) DeleteState(ctx context.Context, stateType StateType, id string) error {
	m.mu.RLock()
	provider, exists := m.stateProviders[stateType]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("no provider registered for state type: %s", stateType)
	}

	// Get old state for event
	oldState, _ := provider.GetState(ctx, id)

	if err := provider.DeleteState(ctx, id); err != nil {
		return err
	}

	// Notify observers
	event := &StateEvent{
		ID:        generateEventID(),
		Type:      StateEventDeleted,
		StateType: stateType,
		StateID:   id,
		OldValue:  oldState,
		Timestamp: time.Now(),
	}

	m.notifyObservers(event)
	m.eventStore.StoreEvent(event)

	return nil
}

// GetSessionState gets session state with type safety
func (m *UnifiedStateManager) GetSessionState(_ context.Context, sessionID string) (*core.SessionState, error) {
	state, err := m.sessionManager.GetSession(sessionID)
	if err != nil {
		return nil, err
	}
	if sessionState, ok := state.(*core.SessionState); ok {
		return sessionState, nil
	}
	return nil, fmt.Errorf("state is not of type *SessionState")
}

// UpdateSessionState updates session state with validation
func (m *UnifiedStateManager) UpdateSessionState(_ context.Context, sessionID string, updates func(*core.SessionState) error) error {
	// Use the session manager's UpdateSession method
	return m.sessionManager.UpdateSession(sessionID, func(current interface{}) {
		if sessionState, ok := current.(*core.SessionState); ok {
			// Apply updates - ignore error for now as the interface doesn't support returning errors
			_ = updates(sessionState)
		}
	})
}

// CreateStateTransaction creates a transaction for atomic state updates
func (m *UnifiedStateManager) CreateStateTransaction(ctx context.Context) *StateTransaction {
	return &StateTransaction{
		manager:    m,
		operations: make([]StateOperation, 0),
		ctx:        ctx,
	}
}

// SyncStates synchronizes states across providers
func (m *UnifiedStateManager) SyncStates(ctx context.Context, sourceType, targetType StateType, mapping StateMapping) error {
	return m.syncCoordinator.SyncStates(ctx, m, sourceType, targetType, mapping)
}

// GetStateHistory retrieves state change history
func (m *UnifiedStateManager) GetStateHistory(_ context.Context, stateType StateType, stateID string, limit int) ([]*StateEvent, error) {
	return m.eventStore.GetEvents(stateType, stateID, limit)
}

// MigrateState migrates state to a new version
func (m *UnifiedStateManager) MigrateState(ctx context.Context, stateType StateType, id string, fromVersion, toVersion string) error {
	m.mu.RLock()
	provider := m.stateProviders[stateType]
	migrator := m.migrators[stateType]
	m.mu.RUnlock()

	if provider == nil {
		return fmt.Errorf("no provider registered for state type: %s", stateType)
	}

	if migrator == nil {
		return fmt.Errorf("no migrator registered for state type: %s", stateType)
	}

	// Get current state
	currentState, err := provider.GetState(ctx, id)
	if err != nil {
		return err
	}

	// Migrate state
	migratedState, err := migrator.MigrateState(ctx, stateType, fromVersion, toVersion, currentState)
	if err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	// Save migrated state
	if err := provider.SetState(ctx, id, migratedState); err != nil {
		return err
	}

	// Notify observers
	event := &StateEvent{
		ID:        generateEventID(),
		Type:      StateEventMigrated,
		StateType: stateType,
		StateID:   id,
		OldValue:  currentState,
		NewValue:  migratedState,
		Metadata: map[string]interface{}{
			"from_version": fromVersion,
			"to_version":   toVersion,
		},
		Timestamp: time.Now(),
	}

	m.notifyObservers(event)
	m.eventStore.StoreEvent(event)

	return nil
}

// notifyObservers notifies all registered observers of a state change
func (m *UnifiedStateManager) notifyObservers(event *StateEvent) {
	m.mu.RLock()
	observers := make([]StateObserver, len(m.observers))
	copy(observers, m.observers)
	m.mu.RUnlock()

	for _, observer := range observers {
		go func(o StateObserver) {
			defer func() {
				if r := recover(); r != nil {
					m.logger.Error().Interface("panic", r).Msg("Observer panicked during notification")
				}
			}()
			o.OnStateChange(event)
		}(observer)
	}
}

// generateEventID generates a unique event ID
func generateEventID() string {
	var randomBytes [8]byte
	if _, err := crypto_rand.Read(randomBytes[:]); err != nil {
		// Fallback to timestamp only
		return fmt.Sprintf("evt_%d", time.Now().UnixNano())
	}
	randomInt := binary.BigEndian.Uint64(randomBytes[:])
	return fmt.Sprintf("evt_%d_%d", time.Now().UnixNano(), randomInt)
}
