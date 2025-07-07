package session

import (
	"context"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/rs/zerolog"
)

// SessionResult represents the result of a session operation
type SessionResult struct {
	Session      *SessionState // Use local session.SessionState type
	IsNew        bool
	IsResumed    bool
	ReplacedFrom string
}

// StandardizedSessionManager provides standardized session management
type StandardizedSessionManager struct {
	sessionManager UnifiedSessionManager
	logger         zerolog.Logger
	toolName       string
}

// NewStandardizedSessionManager creates a new standardized session manager
func NewStandardizedSessionManager(sessionManager UnifiedSessionManager, logger zerolog.Logger, toolName string) *StandardizedSessionManager {
	return &StandardizedSessionManager{
		sessionManager: sessionManager,
		logger:         logger,
		toolName:       toolName,
	}
}

// NewStandardizedSessionManagerUnified creates a new standardized session manager with unified session manager
func NewStandardizedSessionManagerUnified(sessionManager UnifiedSessionManager, logger zerolog.Logger, toolName string) *StandardizedSessionManager {
	return &StandardizedSessionManager{
		sessionManager: sessionManager,
		logger:         logger,
		toolName:       toolName,
	}
}

// GetOrCreateSession gets or creates a session
func (s *StandardizedSessionManager) GetOrCreateSession(sessionID string) (*SessionResult, error) {
	if sessionID == "" {
		return s.createNewSession()
	}

	session, err := s.sessionManager.GetSession(context.Background(), sessionID)
	if err != nil {
		return nil, errors.NewError().Message("failed to get session").Cause(err).Build()
	}
	return &SessionResult{
		Session: session,
		IsNew:   false,
	}, nil
}

// createNewSession creates a new session
func (s *StandardizedSessionManager) createNewSession() (*SessionResult, error) {
	sessionID := generateSessionID()
	session, err := s.sessionManager.GetOrCreateSession(context.Background(), sessionID)
	if err != nil {
		return nil, errors.NewError().Message("failed to create session").Cause(err).Build()
	}
	return &SessionResult{
		Session: session,
		IsNew:   true,
	}, nil
}

// NewRichSessionError creates a rich session error
func NewRichSessionError(operation, sessionID string, err ...error) *errors.RichError {
	builder := errors.NewError().
		Code(errors.CodeInternalError).
		Type(errors.ErrTypeSession).
		Severity(errors.SeverityMedium).
		Context("operation", operation).
		Context("session_id", sessionID)

	if len(err) > 0 && err[0] != nil {
		builder = builder.
			Messagef("Operation '%s' failed for session %s: %v", operation, sessionID, err[0]).
			Cause(err[0])
	} else {
		builder = builder.
			Messagef("Operation '%s' failed for session %s", operation, sessionID)
	}

	return builder.
		WithLocation().
		Build()
}

// NewRichValidationError creates a rich validation error
func NewRichValidationError(field, message string, value interface{}) *errors.RichError {
	return errors.ToolValidationError("", field, message, "VALIDATION_ERROR", value)
}

// ValidateLocalPath validates a local file path
func ValidateLocalPath(path string, requireExists bool) error {
	if path == "" {
		return errors.NewError().Messagef("path cannot be empty").WithLocation(

		// Add more validation as needed
		).Build()
	}

	return nil
}

// StandardizedSessionValidationMixin provides session validation utilities
type StandardizedSessionValidationMixin struct {
	*StandardizedSessionManager
}

// NewStandardizedSessionValidationMixin creates a validation mixin
// Deprecated: Use NewStandardizedSessionValidationMixinUnified instead
func NewStandardizedSessionValidationMixin(sessionManager interface{}, logger zerolog.Logger, toolName string) *StandardizedSessionValidationMixin {
	// Check if it's UnifiedSessionManager
	if unifiedManager, ok := sessionManager.(UnifiedSessionManager); ok {
		return &StandardizedSessionValidationMixin{
			StandardizedSessionManager: NewStandardizedSessionManager(unifiedManager, logger, toolName),
		}
	} else {
		// Fallback - this shouldn't happen in normal usage
		logger.Error().Msg("Invalid session manager type provided to StandardizedSessionValidationMixin")
		return nil
	}
}

// NewStandardizedSessionValidationMixinUnified creates a validation mixin using unified session manager
func NewStandardizedSessionValidationMixinUnified(sessionManager UnifiedSessionManager, logger zerolog.Logger, toolName string) *StandardizedSessionValidationMixin {
	return &StandardizedSessionValidationMixin{
		StandardizedSessionManager: NewStandardizedSessionManagerUnified(sessionManager, logger, toolName),
	}
}

// ValidateSessionInArgs validates session ID in arguments
func (m *StandardizedSessionValidationMixin) ValidateSessionInArgs(sessionID string, required bool) error {
	if required && sessionID == "" {
		return errors.NewError().Messagef("session_id is required").Build(

		// GetOrCreateSessionForTool gets or creates a session for a tool
		)
	}
	return nil
}

func (m *StandardizedSessionValidationMixin) GetOrCreateSessionForTool(sessionID string) (*SessionResult, error) {
	return m.GetOrCreateSession(sessionID)
}

// UpdateToolExecutionMetadata updates tool execution metadata in the session
func (m *StandardizedSessionValidationMixin) UpdateToolExecutionMetadata(session interface{}, result interface{}) error {
	// This is a simplified implementation - in practice you'd update the session state
	// with the provided result for tracking tool execution details
	return nil
}
