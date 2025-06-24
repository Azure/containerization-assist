package interfaces

import (
	"time"
)

// SessionData represents session information for management tools
type SessionData struct {
	ID           string                 `json:"id"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
	ExpiresAt    time.Time              `json:"expires_at"`
	CurrentStage string                 `json:"current_stage"`
	Metadata     map[string]interface{} `json:"metadata"`
	IsActive     bool                   `json:"is_active"`
	LastAccess   time.Time              `json:"last_access"`
}

// SessionManagerStats represents statistics about session management
type SessionManagerStats struct {
	TotalSessions   int     `json:"total_sessions"`
	ActiveSessions  int     `json:"active_sessions"`
	ExpiredSessions int     `json:"expired_sessions"`
	AverageAge      float64 `json:"average_age_hours"`
	OldestSession   string  `json:"oldest_session_id"`
	NewestSession   string  `json:"newest_session_id"`
}
