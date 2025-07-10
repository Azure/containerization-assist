// Package session contains pure business entities and rules for session management.
// This package has no external dependencies and represents the core domain.
package session

import (
	"time"
)

// Session represents a user session entity
type Session struct {
	ID        string                 `json:"id"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
	Status    SessionStatus          `json:"status"`
	Type      SessionType            `json:"type"`
	Metadata  map[string]interface{} `json:"metadata"`
	State     map[string]interface{} `json:"state"`
	Resources SessionResources       `json:"resources,omitempty"`
}

// SessionType defines the type of session
type SessionType string

const (
	SessionTypeInteractive SessionType = "interactive"
	SessionTypeWorkflow    SessionType = "workflow"
	SessionTypeBatch       SessionType = "batch"
	SessionTypeAPI         SessionType = "api"
)

// SessionStatus represents the current status of a session
type SessionStatus string

const (
	SessionStatusActive    SessionStatus = "active"
	SessionStatusInactive  SessionStatus = "inactive"
	SessionStatusCompleted SessionStatus = "completed"
	SessionStatusFailed    SessionStatus = "failed"
	SessionStatusSuspended SessionStatus = "suspended"
	SessionStatusDeleted   SessionStatus = "deleted"
)

// SessionResources defines resource limits for a session
type SessionResources struct {
	MaxMemory     string        `json:"max_memory,omitempty"`
	MaxCPU        string        `json:"max_cpu,omitempty"`
	MaxStorage    string        `json:"max_storage,omitempty"`
	MaxExecutions int           `json:"max_executions,omitempty"`
	Timeout       time.Duration `json:"timeout,omitempty"`
}

// SessionSummary provides a summary view of a session
type SessionSummary struct {
	ID        string        `json:"id"`
	Status    SessionStatus `json:"status"`
	Type      SessionType   `json:"type"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

// HistoryEntry represents an action in session history
type HistoryEntry struct {
	ID        string                 `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	Action    string                 `json:"action"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Error     string                 `json:"error,omitempty"`
}

// IsActive returns true if the session is in an active state
func (s *Session) IsActive() bool {
	return s.Status == SessionStatusActive
}

// IsCompleted returns true if the session has completed (successfully or with failure)
func (s *Session) IsCompleted() bool {
	return s.Status == SessionStatusCompleted || s.Status == SessionStatusFailed
}

// CanBeActivated returns true if the session can be activated
func (s *Session) CanBeActivated() bool {
	return s.Status == SessionStatusInactive || s.Status == SessionStatusSuspended
}

// AddHistoryEntry adds an entry to the session history
func (s *Session) AddHistoryEntry(action string, details map[string]interface{}) {
	if s.State == nil {
		s.State = make(map[string]interface{})
	}

	history, exists := s.State["history"]
	if !exists {
		history = []HistoryEntry{}
	}

	entries := history.([]HistoryEntry)
	entries = append(entries, HistoryEntry{
		ID:        generateID(),
		Timestamp: time.Now(),
		Action:    action,
		Details:   details,
	})

	s.State["history"] = entries
	s.UpdatedAt = time.Now()
}

// generateID generates a simple ID for history entries
func generateID() string {
	return time.Now().Format("20060102150405.000000")
}
