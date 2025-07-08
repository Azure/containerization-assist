package session

import (
	"context"
	"sync"

	errors "github.com/Azure/container-kit/pkg/mcp/errors"
)

// SessionService provides session management without global state
type SessionService struct {
	mu           sync.RWMutex
	storeFactory StoreFactory
}

// NewSessionService creates a new session service
func NewSessionService() *SessionService {
	return &SessionService{}
}

// SetStoreFactory sets the store factory for this service
func (s *SessionService) SetStoreFactory(factory StoreFactory) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.storeFactory = factory
}

// GetStoreFactory returns the configured store factory
func (s *SessionService) GetStoreFactory() StoreFactory {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.storeFactory
}

// CreateSessionManager creates a new SessionManager using the configured store factory
func (s *SessionService) CreateSessionManager(config SessionManagerConfig) (*SessionManager, error) {
	s.mu.RLock()
	factory := s.storeFactory
	s.mu.RUnlock()

	if err := createWorkspaceDir(config.WorkspaceDir); err != nil {
		return nil, err
	}

	var store SessionStore
	if config.StorePath != "" {
		if factory == nil {
			return nil, errors.NewError().Message("no store factory configured").Build()
		}
		boltStore, err := factory(context.Background(), config.StorePath)
		if err != nil {
			return nil, errors.NewError().Message("failed to create session store").Cause(err).Build()
		}
		store = boltStore
	} else {
		// Create a simple in-memory store
		store = NewMemoryStore()
	}

	sm := &SessionManager{
		sessions:     make(map[string]*SessionState),
		workspaceDir: config.WorkspaceDir,
		maxSessions:  config.MaxSessions,
		sessionTTL:   config.SessionTTL,
		store:        store,
		logger:       config.Logger.With("component", "session_manager"),
	}

	return sm, nil
}

// createWorkspaceDir creates the workspace directory if it doesn't exist
func createWorkspaceDir(workspaceDir string) error {
	return errors.NewError().Message("workspace directory creation not implemented").Build()
}
