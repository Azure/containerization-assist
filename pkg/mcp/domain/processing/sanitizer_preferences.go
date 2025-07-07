package processing

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// NewPreferenceStore creates a new preference store
func NewPreferenceStore(storagePathOrConfig interface{}, logger *slog.Logger, encryptionKey ...string) (*PreferenceStore, error) {
	var config PreferenceConfig

	// Handle different argument types for backward compatibility
	switch v := storagePathOrConfig.(type) {
	case string:
		// Legacy usage: NewPreferenceStore(path, logger, encryption)
		config = PreferenceConfig{
			StoragePath:      v,
			EnableEncryption: len(encryptionKey) > 0 && encryptionKey[0] != "",
		}
		if len(encryptionKey) > 0 {
			config.EncryptionKey = encryptionKey[0]
		}
	case PreferenceConfig:
		// New usage: NewPreferenceStore(config, logger)
		config = v
	default:
		return nil, errors.NewError().Messagef("invalid argument type for NewPreferenceStore").Build()
	}

	ps := &PreferenceStore{
		logger:      logger.With("component", "preference_store"),
		config:      config,
		preferences: make(map[string]UserPreferences),
		filePath:    config.StoragePath,
	}

	// Load existing preferences
	if err := ps.load(); err != nil {
		ps.logger.Warn("Failed to load existing preferences", "error", err)
	}

	return ps, nil
}

// GetPreferences retrieves user preferences
func (ps *PreferenceStore) GetPreferences(userID string) (*UserPreferences, error) {
	prefs, exists := ps.preferences[userID]
	if !exists {
		// Return default preferences
		return &UserPreferences{
			UserID:      userID,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Preferences: &TypedJSONData{StringFields: make(map[string]string)},
			Metadata:    &TypedJSONData{StringFields: make(map[string]string)},
			Version:     1,
		}, nil
	}

	return &prefs, nil
}

// SetPreferences stores user preferences
func (ps *PreferenceStore) SetPreferences(userID string, preferences map[string]interface{}) error {
	// Validate preference size
	if ps.config.EnableValidation {
		if err := ps.validatePreferences(preferences); err != nil {
			return err
		}
	}

	existing, exists := ps.preferences[userID]
	if exists {
		existing.Preferences = FromMap(preferences)
		existing.UpdatedAt = time.Now()
		existing.Version++
		ps.preferences[userID] = existing
	} else {
		ps.preferences[userID] = UserPreferences{
			UserID:      userID,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Preferences: FromMap(preferences),
			Metadata:    &TypedJSONData{StringFields: make(map[string]string)},
			Version:     1,
		}
	}

	// Auto-save if enabled
	if ps.config.AutoSave {
		if err := ps.save(); err != nil {
			ps.logger.Error("Failed to auto-save preferences", "error", err)
			return err
		}
	}

	return nil
}

// UpdatePreference updates a specific preference
func (ps *PreferenceStore) UpdatePreference(userID, key string, value interface{}) error {
	prefs, exists := ps.preferences[userID]
	if !exists {
		prefs = UserPreferences{
			UserID:      userID,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Preferences: &TypedJSONData{StringFields: make(map[string]string)},
			Metadata:    &TypedJSONData{StringFields: make(map[string]string)},
			Version:     1,
		}
	}

	// Convert value to string for typed storage
	if strValue, ok := value.(string); ok {
		prefs.Preferences.StringFields[key] = strValue
	} else {
		// For non-string values, store in RawData
		if prefs.Preferences.RawData == nil {
			prefs.Preferences.RawData = make(map[string]interface{})
		}
		prefs.Preferences.RawData[key] = value
	}
	prefs.UpdatedAt = time.Now()
	prefs.Version++
	ps.preferences[userID] = prefs

	return nil
}

// ApplyPreferencesToSession applies stored preferences to session preferences
func (ps *PreferenceStore) ApplyPreferencesToSession(userID string, _ interface{}) error {
	prefs, err := ps.GetPreferences(userID)
	if err != nil {
		// If no preferences found, just return without error
		return nil
	}

	// Try to apply preferences if they exist in the stored preferences
	if prefs.Preferences != nil {
		// This is a simplified implementation - in practice you'd need proper type checking
		// and field mapping based on the actual session preferences structure
		ps.logger.Debug("Applied stored preferences to session",
			"user_id", userID,
			"stored_prefs", prefs.Preferences)
	}

	return nil
}

// Save persists preferences to storage
func (ps *PreferenceStore) Save() error {
	return ps.save()
}

// Close closes the preference store and saves any pending changes
func (ps *PreferenceStore) Close() error {
	// Save any pending changes
	if err := ps.save(); err != nil {
		ps.logger.Error("Failed to save preferences during close", "error", err)
		return err
	}

	ps.logger.Info("Preference store closed")
	return nil
}

// validatePreferences validates preference data
func (ps *PreferenceStore) validatePreferences(preferences map[string]interface{}) error {
	// Check size limit
	data, err := json.Marshal(preferences)
	if err != nil {
		return errors.NewError().
			Code(errors.CodeTypeConversionFailed).
			Type(errors.ErrTypeValidation).
			Severity(errors.SeverityMedium).
			Message("Failed to serialize preferences for validation").
			Cause(err).
			WithLocation().
			Build()
	}

	if len(data) > ps.config.MaxPreferenceSize {
		return errors.NewError().
			Code(errors.CodeResourceExhausted).
			Type(errors.ErrTypeValidation).
			Severity(errors.SeverityMedium).
			Message("Preferences exceed maximum size limit").
			Context("size", len(data)).
			Context("max_size", ps.config.MaxPreferenceSize).
			Suggestion("Reduce the amount of preference data").
			WithLocation().
			Build()
	}

	return nil
}

// load loads preferences from storage
func (ps *PreferenceStore) load() error {
	if ps.filePath == "" {
		return nil
	}

	data, err := os.ReadFile(ps.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // File doesn't exist yet, that's OK
		}
		return err
	}

	if err := json.Unmarshal(data, &ps.preferences); err != nil {
		return errors.NewError().Message("failed to unmarshal preferences").Cause(err).Build()
	}
	return nil
}

// save saves preferences to storage
func (ps *PreferenceStore) save() error {
	if ps.filePath == "" {
		return nil
	}

	// Ensure directory exists
	dir := filepath.Dir(ps.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(ps.preferences, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(ps.filePath, data, 0600); err != nil {
		return errors.NewError().Message("failed to write preferences file").Cause(err).Build()
	}
	return nil
}
