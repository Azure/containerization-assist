package utils

import (
	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/internal/types"
)

// PreferenceStore provides user preference management functionality
// This is a minimal implementation to support conversation handler functionality
type PreferenceStore struct {
	logger *slog.Logger
}

// NewPreferenceStore creates a new preference store
func NewPreferenceStore(dbPath string, logger *slog.Logger, configPath string) (*PreferenceStore, error) {
	return &PreferenceStore{
		logger: logger,
	}, nil
}

// ApplyPreferencesToSession applies user preferences to a session
func (ps *PreferenceStore) ApplyPreferencesToSession(userID string, preferences *types.UserPreferences) error {
	// Minimal implementation - no-op for now
	// In a full implementation, this would load preferences from storage
	ps.logger.Debug("Applying user preferences", "user_id", userID)
	return nil
}

// Close closes the preference store
func (ps *PreferenceStore) Close() error {
	return nil
}
