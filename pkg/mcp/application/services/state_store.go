package services

import (
	"context"

	appstate "github.com/Azure/container-kit/pkg/mcp/application/state"
)

// StateStore handles state persistence operations
type StateStore interface {
	// Save persists state with a key
	Save(ctx context.Context, key string, state interface{}) error

	// Load retrieves state by key
	Load(ctx context.Context, key string) (interface{}, error)

	// Delete removes state by key
	Delete(ctx context.Context, key string) error

	// List returns all state keys with optional prefix filter
	List(ctx context.Context, prefix string) ([]string, error)
}

// stateStore implements StateStore
type stateStore struct {
	manager StateManager
}

// NewStateStore creates a new StateStore service
func NewStateStore(manager StateManager) StateStore {
	return &stateStore{
		manager: manager,
	}
}

func (s *stateStore) Save(ctx context.Context, key string, state interface{}) error {
	return s.manager.SaveState(ctx, key, state)
}

func (s *stateStore) Load(ctx context.Context, key string) (interface{}, error) {
	var state interface{}
	err := s.manager.GetState(ctx, key, &state)
	if err != nil {
		return nil, err
	}
	return state, nil
}

func (s *stateStore) Delete(ctx context.Context, key string) error {
	return s.manager.DeleteState(ctx, key)
}

func (s *stateStore) List(_ context.Context, _ string) ([]string, error) {
	// This would need to be implemented in the actual StateManager
	// For now, return empty list
	return []string{}, nil
}

// stateStoreImpl provides a concrete implementation of StateStore
// that doesn't depend on the old StateManager interface
type stateStoreImpl struct {
	stateManager *appstate.UnifiedStateManager
}

// NewStateStoreImpl creates a new StateStore with concrete implementation
func NewStateStoreImpl(stateManager *appstate.UnifiedStateManager) StateStore {
	return &stateStoreImpl{
		stateManager: stateManager,
	}
}

func (s *stateStoreImpl) Save(ctx context.Context, key string, st interface{}) error {
	// Use SetState method which seems to be the save method
	return s.stateManager.SetState(ctx, appstate.StateTypeGlobal, key, st)
}

func (s *stateStoreImpl) Load(ctx context.Context, key string) (interface{}, error) {
	// Implement using UnifiedStateManager methods
	return s.stateManager.GetState(ctx, appstate.StateTypeGlobal, key)
}

func (s *stateStoreImpl) Delete(ctx context.Context, key string) error {
	// Implement using UnifiedStateManager methods
	return s.stateManager.DeleteState(ctx, appstate.StateTypeGlobal, key)
}

func (s *stateStoreImpl) List(_ context.Context, _ string) ([]string, error) {
	// UnifiedStateManager doesn't have a ListStates method
	// This would need to be implemented based on actual storage
	return []string{}, nil
}
