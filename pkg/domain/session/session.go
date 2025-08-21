// Package session provides domain models and interfaces for session management
package session

import (
	"context"
	"errors"
	"time"
)

// Session represents a session value object (immutable, no I/O)
type Session struct {
	ID        string                 `json:"id"`
	UserID    string                 `json:"user_id"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
	ExpiresAt time.Time              `json:"expires_at"`
	Status    Status                 `json:"status"`
	Stage     string                 `json:"stage,omitempty"`
	Labels    map[string]string      `json:"labels,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// Status represents session status
type Status string

const (
	StatusActive    Status = "active"
	StatusExpired   Status = "expired"
	StatusSuspended Status = "suspended"
)

// Common errors
var (
	ErrSessionNotFound = errors.New("session not found")
)

// Summary represents a lightweight session summary
type Summary struct {
	ID     string            `json:"id"`
	UserID string            `json:"user_id"`
	Status Status            `json:"status"`
	Labels map[string]string `json:"labels,omitempty"`
}

// Stats represents session statistics
type Stats struct {
	ActiveSessions int `json:"active_sessions"`
	TotalSessions  int `json:"total_sessions"`
	MaxSessions    int `json:"max_sessions"`
}

// Store defines the interface for session persistence operations
type Store interface {
	// Create stores a new session
	Create(ctx context.Context, session Session) error

	// Get retrieves a session by ID
	Get(ctx context.Context, id string) (Session, error)

	// Update modifies an existing session
	Update(ctx context.Context, session Session) error

	// Delete removes a session
	Delete(ctx context.Context, id string) error

	// List returns all sessions, optionally filtered
	List(ctx context.Context, filters ...Filter) ([]Session, error)

	// Exists checks if a session exists
	Exists(ctx context.Context, id string) (bool, error)

	// Cleanup removes expired sessions
	Cleanup(ctx context.Context) (int, error)

	// Stats returns storage statistics
	Stats(ctx context.Context) (Stats, error)
}

// Filter represents a session filter
type Filter interface {
	Apply(session Session) bool
}

// StatusFilter filters sessions by status
type StatusFilter struct {
	Status Status
}

func (f StatusFilter) Apply(session Session) bool {
	return session.Status == f.Status
}

func NewSession(id, userID string, ttl time.Duration) Session {
	now := time.Now()
	return Session{
		ID:        id,
		UserID:    userID,
		CreatedAt: now,
		UpdatedAt: now,
		ExpiresAt: now.Add(ttl),
		Status:    StatusActive,
		Labels:    make(map[string]string),
		Metadata:  make(map[string]interface{}),
	}
}

// IsExpired returns true if the session has expired
func (s Session) IsExpired() bool {
	return s.ExpiresAt.Before(time.Now())
}

// IsActive returns true if the session is active and not expired
func (s Session) IsActive() bool {
	return s.Status == StatusActive && !s.IsExpired()
}

// WithStatus returns a copy of the session with updated status
func (s Session) WithStatus(status Status) Session {
	s.Status = status
	s.UpdatedAt = time.Now()
	return s
}

// WithExpiration returns a copy of the session with updated expiration
func (s Session) WithExpiration(expiresAt time.Time) Session {
	s.ExpiresAt = expiresAt
	s.UpdatedAt = time.Now()
	return s
}

// WithLabel returns a copy of the session with an added/updated label
func (s Session) WithLabel(key, value string) Session {
	if s.Labels == nil {
		s.Labels = make(map[string]string)
	}
	s.Labels[key] = value
	s.UpdatedAt = time.Now()
	return s
}

// WithMetadata returns a copy of the session with added/updated metadata
func (s Session) WithMetadata(key string, value interface{}) Session {
	if s.Metadata == nil {
		s.Metadata = make(map[string]interface{})
	}
	s.Metadata[key] = value
	s.UpdatedAt = time.Now()
	return s
}

// ToSummary returns a summary representation of the session
func (s Session) ToSummary() Summary {
	return Summary{
		ID:     s.ID,
		UserID: s.UserID,
		Status: s.Status,
		Labels: s.Labels,
	}
}
