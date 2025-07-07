package state

import (
	"context"

	"github.com/Azure/container-kit/pkg/mcp/application/core"

	crypto_rand "crypto/rand"
	"encoding/binary"
	"fmt"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors/codes"
	"github.com/Azure/container-kit/pkg/mcp/domain/session"
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

// UnifiedStateManager manages all state in the system
type UnifiedStateManager struct {
	sessionManager  *session.SessionManager
	stateProviders  map[StateType]StateProvider
	observers       []StateObserver
	validators      map[StateType]StateValidator
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

// RegisterObserver registers a state observer and returns an unregister function
func (m *UnifiedStateManager) RegisterObserver(observer StateObserver) func() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.observers = append(m.observers, observer)
	m.logger.Debug().Msg("Registered state observer")

	return func() {
		m.unregisterObserver(observer)
	}
}

// unregisterObserver removes an observer from the list
func (m *UnifiedStateManager) unregisterObserver(observer StateObserver) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, obs := range m.observers {
		if obs == observer {
			m.observers[i] = m.observers[len(m.observers)-1]
			m.observers = m.observers[:len(m.observers)-1]
			m.logger.Debug().Msg("Unregistered state observer")
			return
		}
	}
}

// RegisterValidator registers a state validator
func (m *UnifiedStateManager) RegisterValidator(stateType StateType, validator StateValidator) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.validators[stateType] = validator
	m.logger.Info().Str("state_type", string(stateType)).Msg("Registered state validator")
}

// GetState retrieves state of a specific type
func (m *UnifiedStateManager) GetState(ctx context.Context, stateType StateType, id string) (interface{}, error) {
	m.mu.RLock()
	provider, exists := m.stateProviders[stateType]
	m.mu.RUnlock()

	if !exists {
		systemErr := errors.SystemError(
			codes.SYSTEM_ERROR,
			fmt.Sprintf("No provider registered for state type: %s", stateType),
			nil,
		)
		systemErr.Context["state_type"] = string(stateType)
		systemErr.Context["component"] = "unified_state_manager"
		systemErr.Suggestions = append(systemErr.Suggestions, "Register a state provider for this state type before use")
		return nil, systemErr
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
		systemErr := errors.SystemError(
			codes.SYSTEM_ERROR,
			fmt.Sprintf("No provider registered for state type: %s", stateType),
			nil,
		)
		systemErr.Context["state_type"] = string(stateType)
		systemErr.Context["component"] = "unified_state_manager"
		systemErr.Suggestions = append(systemErr.Suggestions, "Register a state provider for this state type before use")
		return systemErr
	}

	if validator != nil {
		if err := validator.ValidateState(ctx, stateType, state); err != nil {
			return errors.NewError().
				Code(codes.VALIDATION_FAILED).
				Message("State validation failed").
				Cause(err).
				Context("state_type", string(stateType)).
				Context("state_id", id).
				Context("component", "unified_state_manager").
				Suggestion("Check state data format and required fields").
				Build()
		}
	}

	oldState, _ := provider.GetState(ctx, id)

	if err := provider.SetState(ctx, id, state); err != nil {
		return err
	}

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
		systemErr := errors.SystemError(
			codes.SYSTEM_ERROR,
			fmt.Sprintf("No provider registered for state type: %s", stateType),
			nil,
		)
		systemErr.Context["state_type"] = string(stateType)
		systemErr.Context["component"] = "unified_state_manager"
		systemErr.Suggestions = append(systemErr.Suggestions, "Register a state provider for this state type before use")
		return systemErr
	}

	oldState, _ := provider.GetState(ctx, id)

	if err := provider.DeleteState(ctx, id); err != nil {
		return err
	}

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
	state, err := m.sessionManager.GetSessionConcrete(sessionID)
	if err != nil {
		return nil, err
	}
	if state != nil {
		return state.ToCoreSessionState(), nil
	}
	return nil, errors.NewError().
		Code(codes.VALIDATION_FAILED).
		Message("State is not of type *SessionState").
		Context("session_id", sessionID).
		Context("component", "unified_state_manager").
		Suggestion("Ensure session state is properly initialized").
		Build()
}

// UpdateSessionState updates session state with validation
func (m *UnifiedStateManager) UpdateSessionState(ctx context.Context, sessionID string, updates func(*core.SessionState) error) error {
	return m.sessionManager.UpdateSession(ctx, sessionID, func(current *session.SessionState) error {
		coreState := current.ToCoreSessionState()
		err := updates(coreState)
		if err != nil {
			return err
		}
		return nil
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
		return fmt.Sprintf("evt_%d", time.Now().UnixNano())
	}
	randomInt := binary.BigEndian.Uint64(randomBytes[:])
	return fmt.Sprintf("evt_%d_%d", time.Now().UnixNano(), randomInt)
}
