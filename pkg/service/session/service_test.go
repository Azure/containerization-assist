package session

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainsession "github.com/Azure/containerization-assist/pkg/domain/session"
)

type MockSessionStore struct {
	sessions   map[string]domainsession.Session
	shouldFail bool
}

func NewMockSessionStore() *MockSessionStore {
	return &MockSessionStore{
		sessions: make(map[string]domainsession.Session),
	}
}

func (m *MockSessionStore) Create(ctx context.Context, session domainsession.Session) error {
	if m.shouldFail {
		return assert.AnError
	}
	m.sessions[session.ID] = session
	return nil
}

func (m *MockSessionStore) Get(ctx context.Context, sessionID string) (domainsession.Session, error) {
	if m.shouldFail {
		return domainsession.Session{}, assert.AnError
	}
	session, exists := m.sessions[sessionID]
	if !exists {
		return domainsession.Session{}, domainsession.ErrSessionNotFound
	}
	return session, nil
}

func (m *MockSessionStore) Update(ctx context.Context, session domainsession.Session) error {
	if m.shouldFail {
		return assert.AnError
	}
	m.sessions[session.ID] = session
	return nil
}

func (m *MockSessionStore) Delete(ctx context.Context, sessionID string) error {
	if m.shouldFail {
		return assert.AnError
	}
	delete(m.sessions, sessionID)
	return nil
}

func (m *MockSessionStore) List(ctx context.Context, filters ...domainsession.Filter) ([]domainsession.Session, error) {
	if m.shouldFail {
		return nil, assert.AnError
	}
	var sessions []domainsession.Session
	for _, session := range m.sessions {
		include := true
		for _, filter := range filters {
			if !filter.Apply(session) {
				include = false
				break
			}
		}
		if include {
			sessions = append(sessions, session)
		}
	}
	return sessions, nil
}

func (m *MockSessionStore) Exists(ctx context.Context, sessionID string) (bool, error) {
	if m.shouldFail {
		return false, assert.AnError
	}
	_, exists := m.sessions[sessionID]
	return exists, nil
}

func (m *MockSessionStore) Cleanup(ctx context.Context) (int, error) {
	if m.shouldFail {
		return 0, assert.AnError
	}
	removed := 0
	now := time.Now()
	for id, session := range m.sessions {
		if session.ExpiresAt.Before(now) {
			delete(m.sessions, id)
			removed++
		}
	}
	return removed, nil
}

func (m *MockSessionStore) Stats(ctx context.Context) (domainsession.Stats, error) {
	if m.shouldFail {
		return domainsession.Stats{}, assert.AnError
	}
	active := 0
	for _, session := range m.sessions {
		if session.IsActive() {
			active++
		}
	}
	return domainsession.Stats{
		ActiveSessions: active,
		TotalSessions:  len(m.sessions),
		MaxSessions:    100,
	}, nil
}

func TestNewService(t *testing.T) {
	mockStore := NewMockSessionStore()
	logger := slog.Default()
	defaultTTL := 30 * time.Minute

	service := NewService(mockStore, logger, defaultTTL)

	assert.NotNil(t, service)
	assert.Equal(t, mockStore, service.store)
	assert.Equal(t, defaultTTL, service.defaultTTL)
}

func TestService_GetOrCreate(t *testing.T) {
	mockStore := NewMockSessionStore()
	logger := slog.Default()
	service := NewService(mockStore, logger, 30*time.Minute)

	sessionID := "test-session"
	session, err := service.GetOrCreate(context.Background(), sessionID)

	require.NoError(t, err)
	assert.NotNil(t, session)
	assert.Equal(t, sessionID, session.SessionID)
}

func TestService_Get_ExistingSession(t *testing.T) {
	mockStore := NewMockSessionStore()
	logger := slog.Default()
	service := NewService(mockStore, logger, 30*time.Minute)

	// Create a session first
	sessionID := "existing-session"
	_, err := service.GetOrCreate(context.Background(), sessionID)
	require.NoError(t, err)

	// Now get it
	retrieved, err := service.Get(context.Background(), sessionID)
	require.NoError(t, err)
	assert.Equal(t, sessionID, retrieved.SessionID)
}

func TestService_Get_NonExistentSession(t *testing.T) {
	mockStore := NewMockSessionStore()
	logger := slog.Default()
	service := NewService(mockStore, logger, 30*time.Minute)

	_, err := service.Get(context.Background(), "non-existent")
	assert.Error(t, err)
}

func TestService_Update(t *testing.T) {
	mockStore := NewMockSessionStore()
	logger := slog.Default()
	service := NewService(mockStore, logger, 30*time.Minute)

	// Create a session first
	sessionID := "update-session"
	_, err := service.GetOrCreate(context.Background(), sessionID)
	require.NoError(t, err)

	// Update the session
	err = service.Update(context.Background(), sessionID, func(s *SessionState) error {
		s.Stage = "updated"
		return nil
	})
	assert.NoError(t, err)
}

func TestService_List(t *testing.T) {
	mockStore := NewMockSessionStore()
	logger := slog.Default()
	service := NewService(mockStore, logger, 30*time.Minute)

	// Create some sessions
	_, err := service.GetOrCreate(context.Background(), "session1")
	require.NoError(t, err)
	_, err = service.GetOrCreate(context.Background(), "session2")
	require.NoError(t, err)

	sessions, err := service.List(context.Background())
	require.NoError(t, err)
	assert.Len(t, sessions, 2)
}

func TestService_Stats(t *testing.T) {
	mockStore := NewMockSessionStore()
	logger := slog.Default()
	service := NewService(mockStore, logger, 30*time.Minute)

	stats := service.Stats()
	assert.NotNil(t, stats)
}

func TestService_Cleanup(t *testing.T) {
	mockStore := NewMockSessionStore()
	logger := slog.Default()
	service := NewService(mockStore, logger, 30*time.Minute)

	err := service.Cleanup(context.Background())
	assert.NoError(t, err)
}

func TestService_Stop(t *testing.T) {
	mockStore := NewMockSessionStore()
	logger := slog.Default()
	service := NewService(mockStore, logger, 30*time.Minute)

	err := service.Stop(context.Background())
	assert.NoError(t, err)
}

func TestMockSessionStore_FailureScenarios(t *testing.T) {
	mockStore := NewMockSessionStore()
	mockStore.shouldFail = true
	logger := slog.Default()
	service := NewService(mockStore, logger, 30*time.Minute)

	_, err := service.GetOrCreate(context.Background(), "test")
	assert.Error(t, err)

	_, err = service.Get(context.Background(), "test")
	assert.Error(t, err)

	err = service.Update(context.Background(), "test", func(s *SessionState) error {
		return nil
	})
	assert.Error(t, err)

	_, err = service.List(context.Background())
	assert.Error(t, err)
}
