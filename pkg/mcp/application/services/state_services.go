package services

import (
	appstate "github.com/Azure/container-kit/pkg/mcp/application/state"
)

// StateServices provides access to all state-related services
type StateServices interface {
	// Store returns the state store service
	Store() StateStore

	// Provider returns the state provider service
	Provider() StateProvider
}

// stateServices implements StateServices
type stateServices struct {
	store    StateStore
	provider StateProvider
}

// NewStateServices creates a new StateServices container with all services
func NewStateServices(stateManager *appstate.UnifiedStateManager) StateServices {
	return &stateServices{
		store:    NewStateStoreImpl(stateManager),
		provider: NewStateProvider(stateManager),
	}
}

// NewStateServicesFromManager creates services from the old StateManager interface
// This is useful for gradual migration
func NewStateServicesFromManager(manager StateManager) StateServices {
	// Fall back to adapter-based implementation
	return &stateServices{
		store:    NewStateStore(manager),
		provider: nil, // Provider requires UnifiedStateManager features
	}
}

func (s *stateServices) Store() StateStore {
	return s.store
}

func (s *stateServices) Provider() StateProvider {
	return s.provider
}
