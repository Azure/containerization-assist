// Package session contains business rules for session management
package session

import (
	"fmt"
	"time"
)

// ValidationError represents a session validation error
type ValidationError struct {
	Field   string
	Message string
	Code    string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("session validation error: %s - %s", e.Field, e.Message)
}

// Validate performs domain-level validation on a session
func (s *Session) Validate() []ValidationError {
	var errors []ValidationError

	// ID is required
	if s.ID == "" {
		errors = append(errors, ValidationError{
			Field:   "id",
			Message: "session ID is required",
			Code:    "MISSING_ID",
		})
	}

	// CreatedAt must not be zero
	if s.CreatedAt.IsZero() {
		errors = append(errors, ValidationError{
			Field:   "created_at",
			Message: "created_at timestamp is required",
			Code:    "MISSING_CREATED_AT",
		})
	}

	// UpdatedAt must not be before CreatedAt
	if !s.UpdatedAt.IsZero() && s.UpdatedAt.Before(s.CreatedAt) {
		errors = append(errors, ValidationError{
			Field:   "updated_at",
			Message: "updated_at cannot be before created_at",
			Code:    "INVALID_UPDATED_AT",
		})
	}

	// Status must be valid
	if !s.isValidStatus() {
		errors = append(errors, ValidationError{
			Field:   "status",
			Message: "invalid session status",
			Code:    "INVALID_STATUS",
		})
	}

	// Type must be valid
	if !s.isValidType() {
		errors = append(errors, ValidationError{
			Field:   "type",
			Message: "invalid session type",
			Code:    "INVALID_TYPE",
		})
	}

	// Resource limits validation
	if err := s.validateResources(); err != nil {
		errors = append(errors, *err)
	}

	return errors
}

// isValidStatus checks if the session status is valid
func (s *Session) isValidStatus() bool {
	validStatuses := []SessionStatus{
		SessionStatusActive,
		SessionStatusInactive,
		SessionStatusCompleted,
		SessionStatusFailed,
		SessionStatusSuspended,
		SessionStatusDeleted,
	}

	for _, status := range validStatuses {
		if s.Status == status {
			return true
		}
	}
	return false
}

// isValidType checks if the session type is valid
func (s *Session) isValidType() bool {
	validTypes := []SessionType{
		SessionTypeInteractive,
		SessionTypeWorkflow,
		SessionTypeBatch,
		SessionTypeAPI,
	}

	for _, sessionType := range validTypes {
		if s.Type == sessionType {
			return true
		}
	}
	return false
}

// validateResources validates resource limits
func (s *Session) validateResources() *ValidationError {
	if s.Resources.MaxExecutions < 0 {
		return &ValidationError{
			Field:   "resources.max_executions",
			Message: "max_executions cannot be negative",
			Code:    "INVALID_MAX_EXECUTIONS",
		}
	}

	if s.Resources.Timeout < 0 {
		return &ValidationError{
			Field:   "resources.timeout",
			Message: "timeout cannot be negative",
			Code:    "INVALID_TIMEOUT",
		}
	}

	return nil
}

// Business Rules

// CanTransitionTo checks if a session can transition to the target status
func (s *Session) CanTransitionTo(targetStatus SessionStatus) bool {
	switch s.Status {
	case SessionStatusActive:
		return targetStatus == SessionStatusCompleted ||
			targetStatus == SessionStatusFailed ||
			targetStatus == SessionStatusSuspended

	case SessionStatusInactive:
		return targetStatus == SessionStatusActive ||
			targetStatus == SessionStatusDeleted

	case SessionStatusSuspended:
		return targetStatus == SessionStatusActive ||
			targetStatus == SessionStatusFailed ||
			targetStatus == SessionStatusDeleted

	case SessionStatusCompleted, SessionStatusFailed:
		return targetStatus == SessionStatusDeleted

	case SessionStatusDeleted:
		return false // No transitions from deleted

	default:
		return false
	}
}

// IsExpired checks if the session has exceeded its timeout
func (s *Session) IsExpired() bool {
	if s.Resources.Timeout <= 0 {
		return false // No timeout set
	}

	return time.Since(s.CreatedAt) > s.Resources.Timeout
}

// ShouldAutoCleanup determines if a session should be automatically cleaned up
func (s *Session) ShouldAutoCleanup() bool {
	// Sessions older than 30 days in completed/failed state should be cleaned up
	if s.Status == SessionStatusCompleted || s.Status == SessionStatusFailed {
		return time.Since(s.UpdatedAt) > 30*24*time.Hour
	}

	// Deleted sessions older than 7 days should be cleaned up
	if s.Status == SessionStatusDeleted {
		return time.Since(s.UpdatedAt) > 7*24*time.Hour
	}

	return false
}