package persistence

import (
	"context"
	"sync"

	"github.com/Azure/container-kit/pkg/mcp/application/services"
	"github.com/Azure/container-kit/pkg/mcp/domain"
)

// MemoryStore is a simple in-memory implementation of SessionStore
type MemoryStore struct {
	sessions map[string]*domain.SessionState
	mu       sync.RWMutex
}

// NewMemoryStore creates a new in-memory session store
func NewMemoryStore() services.SessionStore {
	return &MemoryStore{
		sessions: make(map[string]*domain.SessionState),
	}
}

// Save stores a session in memory
func (m *MemoryStore) Save(ctx context.Context, sessionID string, session *domain.SessionState) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Deep copy the session to avoid external modifications
	sessionCopy := *session
	m.sessions[sessionID] = &sessionCopy
	return nil
}

// Load retrieves a session from memory
func (m *MemoryStore) Load(ctx context.Context, sessionID string) (*domain.SessionState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, nil
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

// List returns all session IDs
func (m *MemoryStore) List(ctx context.Context) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	return ids, nil
}

// Close is a no-op for memory store
func (m *MemoryStore) Close() error {
	return nil
}
