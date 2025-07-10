package persistence

import (
	"context"
	"sync"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/application/services"
	"github.com/Azure/container-kit/pkg/mcp/domain"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// MemoryStore is a simple in-memory implementation of SessionStore
type MemoryStore struct {
	sessions map[string]*api.Session
	mu       sync.RWMutex
}

// NewMemoryStore creates a new in-memory session store
func NewMemoryStore() services.SessionStore {
	return &MemoryStore{
		sessions: make(map[string]*api.Session),
	}
}

// Create creates a new session
func (m *MemoryStore) Create(_ context.Context, session *api.Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.sessions[session.ID]; exists {
		return errors.NewError().
			Code(errors.CodeAlreadyExists).
			Type(errors.ErrTypeConflict).
			Severity(errors.SeverityMedium).
			Messagef("session already exists: %s", session.ID).
			WithLocation().
			Build()
	}

	// Deep copy the session to avoid external modifications
	sessionCopy := *session
	m.sessions[session.ID] = &sessionCopy
	return nil
}

// Get retrieves a session by ID
func (m *MemoryStore) Get(_ context.Context, sessionID string) (*api.Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, errors.NewError().
			Code(errors.CodeNotFound).
			Type(errors.ErrTypeNotFound).
			Severity(errors.SeverityMedium).
			Messagef("session not found: %s", sessionID).
			WithLocation().
			Build()
	}

	// Return a copy to avoid external modifications
	sessionCopy := *session
	return &sessionCopy, nil
}

// Delete removes a session from memory
func (m *MemoryStore) Delete(ctx context.Context, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.sessions, sessionID)
	return nil
}

// Update updates an existing session
func (m *MemoryStore) Update(_ context.Context, session *api.Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.sessions[session.ID]; !exists {
		return errors.NewError().
			Code(errors.CodeNotFound).
			Type(errors.ErrTypeNotFound).
			Severity(errors.SeverityMedium).
			Messagef("session not found: %s", session.ID).
			WithLocation().
			Build()
	}

	// Deep copy the session to avoid external modifications
	sessionCopy := *session
	m.sessions[session.ID] = &sessionCopy
	return nil
}

// List returns all sessions
func (m *MemoryStore) List(_ context.Context) ([]*api.Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*api.Session, 0, len(m.sessions))
	for _, session := range m.sessions {
		// Return copies to avoid external modifications
		sessionCopy := *session
		sessions = append(sessions, &sessionCopy)
	}
	return sessions, nil
}

// Save stores a session in memory (legacy method for backward compatibility)
func (m *MemoryStore) Save(ctx context.Context, sessionID string, _ *domain.SessionState) error {
	// Convert domain.SessionState to api.Session
	apiSession := &api.Session{
		ID: sessionID,
		// Add other field mappings as needed
	}
	return m.Update(ctx, apiSession)
}

// Load retrieves a session from memory (legacy method for backward compatibility)
func (m *MemoryStore) Load(ctx context.Context, sessionID string) (*domain.SessionState, error) {
	apiSession, err := m.Get(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	// Convert api.Session to domain.SessionState
	return &domain.SessionState{
		SessionID: apiSession.ID,
		// Add other field mappings as needed
	}, nil
}

// Close is a no-op for memory store
func (m *MemoryStore) Close() error {
	return nil
}
