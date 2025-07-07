package containerization

import (
	"context"
	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/domain/session"
	"github.com/Azure/container-kit/pkg/mcp/services"
	"github.com/google/uuid"
)

// SessionManagerAdapter adapts UnifiedSessionManager to ServiceContainer
type SessionManagerAdapter struct {
	sessionManager session.UnifiedSessionManager
	logger         *slog.Logger
}

// NewSessionManagerAdapter creates a new adapter
func NewSessionManagerAdapter(sessionManager session.UnifiedSessionManager, logger *slog.Logger) services.ServiceContainer {
	return &SessionManagerAdapter{
		sessionManager: sessionManager,
		logger:         logger,
	}
}

// SessionStore returns a SessionStore implementation
func (a *SessionManagerAdapter) SessionStore() services.SessionStore {
	return &sessionStoreAdapter{manager: a.sessionManager, logger: a.logger}
}

// SessionState returns a SessionState implementation
func (a *SessionManagerAdapter) SessionState() services.SessionState {
	return &sessionStateAdapter{manager: a.sessionManager, logger: a.logger}
}

// BuildExecutor returns nil as it's not available in this adapter
func (a *SessionManagerAdapter) BuildExecutor() services.BuildExecutor {
	return nil
}

// ToolRegistry returns nil as it's not available in this adapter
func (a *SessionManagerAdapter) ToolRegistry() services.ToolRegistry {
	return nil
}

// WorkflowExecutor returns nil as it's not available in this adapter
func (a *SessionManagerAdapter) WorkflowExecutor() services.WorkflowExecutor {
	return nil
}

// Scanner returns nil as it's not available in this adapter
func (a *SessionManagerAdapter) Scanner() services.Scanner {
	return nil
}

// ConfigValidator returns nil as it's not available in this adapter
func (a *SessionManagerAdapter) ConfigValidator() services.ConfigValidator {
	return nil
}

// ErrorReporter returns nil as it's not available in this adapter
func (a *SessionManagerAdapter) ErrorReporter() services.ErrorReporter {
	return nil
}

// Close closes the adapter (no-op in this implementation)
func (a *SessionManagerAdapter) Close() error {
	return nil
}

// sessionStoreAdapter adapts UnifiedSessionManager to SessionStore
type sessionStoreAdapter struct {
	manager session.UnifiedSessionManager
	logger  *slog.Logger
}

func (s *sessionStoreAdapter) Create(ctx context.Context, metadata map[string]interface{}) (string, error) {
	// Generate a new session ID
	sessionID := uuid.New().String()

	// Create session using the manager
	_, err := s.manager.CreateSession(ctx, sessionID)
	if err != nil {
		return "", err
	}

	// Update with metadata if provided
	if len(metadata) > 0 {
		err = s.manager.UpdateSession(ctx, sessionID, func(sess *session.SessionState) error {
			sess.Metadata = metadata
			return nil
		})
		if err != nil {
			return "", err
		}
	}

	return sessionID, nil
}

func (s *sessionStoreAdapter) Get(ctx context.Context, sessionID string) (*api.Session, error) {
	sess, err := s.manager.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	// Convert to api.Session
	return &api.Session{
		ID:        sess.SessionID,
		Metadata:  sess.Metadata,
		CreatedAt: sess.CreatedAt,
		UpdatedAt: sess.UpdatedAt,
	}, nil
}

func (s *sessionStoreAdapter) Update(ctx context.Context, sessionID string, data map[string]interface{}) error {
	return s.manager.UpdateSession(ctx, sessionID, func(sess *session.SessionState) error {
		sess.Metadata = data
		return nil
	})
}

func (s *sessionStoreAdapter) Delete(ctx context.Context, sessionID string) error {
	return s.manager.DeleteSession(ctx, sessionID)
}

// sessionStateAdapter adapts UnifiedSessionManager to SessionState
type sessionStateAdapter struct {
	manager session.UnifiedSessionManager
	logger  *slog.Logger
}

func (s *sessionStateAdapter) SaveState(ctx context.Context, sessionID string, state map[string]interface{}) error {
	return s.manager.UpdateSession(ctx, sessionID, func(sess *session.SessionState) error {
		if sess.Metadata == nil {
			sess.Metadata = make(map[string]interface{})
		}
		// Merge the state into metadata
		for k, v := range state {
			sess.Metadata[k] = v
		}
		return nil
	})
}

func (s *sessionStateAdapter) LoadState(ctx context.Context, sessionID string) (map[string]interface{}, error) {
	sess, err := s.manager.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	if sess.Metadata == nil {
		return make(map[string]interface{}), nil
	}

	return sess.Metadata, nil
}

func (s *sessionStateAdapter) SaveCheckpoint(ctx context.Context, sessionID string, data interface{}) error {
	return s.manager.UpdateSession(ctx, sessionID, func(sess *session.SessionState) error {
		if sess.Metadata == nil {
			sess.Metadata = make(map[string]interface{})
		}
		sess.Metadata["_checkpoint"] = data
		return nil
	})
}

func (s *sessionStateAdapter) LoadCheckpoint(ctx context.Context, sessionID string) (interface{}, error) {
	sess, err := s.manager.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	if sess.Metadata == nil {
		return nil, nil
	}

	return sess.Metadata["_checkpoint"], nil
}
