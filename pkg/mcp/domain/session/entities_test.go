package session

import (
	"testing"
	"time"
)

func TestSession_IsActive(t *testing.T) {
	session := &Session{
		ID:     "test-session",
		Status: SessionStatusActive,
	}

	if !session.IsActive() {
		t.Error("expected session to be active")
	}

	session.Status = SessionStatusInactive
	if session.IsActive() {
		t.Error("expected session to not be active")
	}
}

func TestSession_CanTransitionTo(t *testing.T) {
	tests := []struct {
		name           string
		currentStatus  SessionStatus
		targetStatus   SessionStatus
		expectedResult bool
	}{
		{"active to completed", SessionStatusActive, SessionStatusCompleted, true},
		{"active to failed", SessionStatusActive, SessionStatusFailed, true},
		{"active to suspended", SessionStatusActive, SessionStatusSuspended, true},
		{"active to deleted", SessionStatusActive, SessionStatusDeleted, false},
		{"inactive to active", SessionStatusInactive, SessionStatusActive, true},
		{"deleted to any", SessionStatusDeleted, SessionStatusActive, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &Session{Status: tt.currentStatus}
			result := session.CanTransitionTo(tt.targetStatus)
			if result != tt.expectedResult {
				t.Errorf("expected %v, got %v", tt.expectedResult, result)
			}
		})
	}
}

func TestSession_Validate(t *testing.T) {
	validSession := &Session{
		ID:        "test-session",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Status:    SessionStatusActive,
		Type:      SessionTypeInteractive,
		Resources: SessionResources{
			MaxExecutions: 10,
			Timeout:       time.Hour,
		},
	}

	errors := validSession.Validate()
	if len(errors) != 0 {
		t.Errorf("expected no validation errors, got %d: %v", len(errors), errors)
	}

	// Test missing ID
	invalidSession := &Session{
		CreatedAt: time.Now(),
		Status:    SessionStatusActive,
		Type:      SessionTypeInteractive,
	}

	errors = invalidSession.Validate()
	if len(errors) == 0 {
		t.Error("expected validation errors for missing ID")
	}

	found := false
	for _, err := range errors {
		if err.Code == "MISSING_ID" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected MISSING_ID validation error")
	}
}

func TestSession_IsExpired(t *testing.T) {
	// Session with timeout
	session := &Session{
		ID:        "test-session",
		CreatedAt: time.Now().Add(-2 * time.Hour),
		Resources: SessionResources{
			Timeout: time.Hour,
		},
	}

	if !session.IsExpired() {
		t.Error("expected session to be expired")
	}

	// Session without timeout
	session.Resources.Timeout = 0
	if session.IsExpired() {
		t.Error("expected session to not be expired when no timeout is set")
	}
}

func TestSession_AddHistoryEntry(t *testing.T) {
	session := &Session{
		ID:    "test-session",
		State: make(map[string]interface{}),
	}

	session.AddHistoryEntry("test-action", map[string]interface{}{"key": "value"})

	history, exists := session.State["history"]
	if !exists {
		t.Error("expected history to exist in session state")
	}

	entries := history.([]HistoryEntry)
	if len(entries) != 1 {
		t.Errorf("expected 1 history entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.Action != "test-action" {
		t.Errorf("expected action 'test-action', got '%s'", entry.Action)
	}
}
