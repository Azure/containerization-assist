package session

import (
	"context"
	"sort"
	"strings"

	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// AddSessionLabel implements UnifiedSessionManager interface
func (sm *SessionManager) AddSessionLabel(ctx context.Context, sessionID, label string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return errors.NewError().Messagef("session not found: %s", sessionID).Build()
	}

	// Normalize label
	label = strings.TrimSpace(strings.ToLower(label))
	if label == "" {
		return errors.NewError().Message("label cannot be empty").Build()
	}

	// Check if label already exists
	for _, existing := range session.Labels {
		if existing == label {
			return nil // Label already exists
		}
	}

	// Add label
	session.Labels = append(session.Labels, label)
	sort.Strings(session.Labels) // Keep labels sorted

	// Save to persistent store
	if err := sm.store.Save(ctx, sessionID, session); err != nil {
		sm.logger.Warn("Failed to save session after adding label", "error", err, "session_id", sessionID)
	}

	sm.logger.Info("Added label to session", "session_id", sessionID, "label", label)
	return nil
}

// AddSessionLabelLegacy adds a label to a session (legacy interface)
func (sm *SessionManager) AddSessionLabelLegacy(sessionID, label string) error {
	return sm.AddSessionLabel(context.Background(), sessionID, label)
}

// RemoveSessionLabel implements UnifiedSessionManager interface
func (sm *SessionManager) RemoveSessionLabel(ctx context.Context, sessionID, label string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return errors.NewError().Messagef("session not found: %s", sessionID).Build()
	}

	// Normalize label
	label = strings.TrimSpace(strings.ToLower(label))

	// Remove label
	newLabels := []string{}
	found := false
	for _, existing := range session.Labels {
		if existing != label {
			newLabels = append(newLabels, existing)
		} else {
			found = true
		}
	}

	if !found {
		return errors.NewError().Messagef("label not found: %s", label).Build()
	}

	session.Labels = newLabels

	// Save to persistent store
	if err := sm.store.Save(ctx, sessionID, session); err != nil {
		sm.logger.Warn("Failed to save session after removing label", "error", err, "session_id", sessionID)
	}

	sm.logger.Info("Removed label from session", "session_id", sessionID, "label", label)
	return nil
}

// RemoveSessionLabelLegacy removes a label from a session (legacy interface)
func (sm *SessionManager) RemoveSessionLabelLegacy(sessionID, label string) error {
	return sm.RemoveSessionLabel(context.Background(), sessionID, label)
}

// GetSessionsByLabel implements UnifiedSessionManager interface
func (sm *SessionManager) GetSessionsByLabel(ctx context.Context, label string) ([]*SessionData, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// Normalize label
	label = strings.TrimSpace(strings.ToLower(label))

	sessions := []*SessionData{}
	for _, session := range sm.sessions {
		for _, sessionLabel := range session.Labels {
			if sessionLabel == label {
				sessions = append(sessions, &SessionData{
					ID:           session.ID,
					CreatedAt:    session.CreatedAt,
					UpdatedAt:    session.UpdatedAt,
					WorkspaceDir: session.WorkspaceDir,
					Metadata:     session.Metadata,
					Status:       session.Status,
					Labels:       session.Labels,
					DiskUsage:    session.DiskUsage,
				})
				break
			}
		}
	}

	// Sort by creation time, newest first
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].CreatedAt.After(sessions[j].CreatedAt)
	})

	return sessions, nil
}

// GetSessionsByLabelLegacy returns sessions by label (legacy interface)
func (sm *SessionManager) GetSessionsByLabelLegacy(label string) ([]*SessionData, error) {
	return sm.GetSessionsByLabel(context.Background(), label)
}

// GetAllLabels returns all unique labels across sessions
func (sm *SessionManager) GetAllLabels() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	labelSet := make(map[string]bool)
	for _, session := range sm.sessions {
		for _, label := range session.Labels {
			labelSet[label] = true
		}
	}

	labels := make([]string, 0, len(labelSet))
	for label := range labelSet {
		labels = append(labels, label)
	}

	sort.Strings(labels)
	return labels
}

// UpdateSessionLabels replaces all labels for a session
func (sm *SessionManager) UpdateSessionLabels(sessionID string, labels []string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return errors.NewError().Messagef("session not found: %s", sessionID).Build()
	}

	// Normalize and deduplicate labels
	labelSet := make(map[string]bool)
	for _, label := range labels {
		normalized := strings.TrimSpace(strings.ToLower(label))
		if normalized != "" {
			labelSet[normalized] = true
		}
	}

	// Convert back to sorted slice
	newLabels := make([]string, 0, len(labelSet))
	for label := range labelSet {
		newLabels = append(newLabels, label)
	}
	sort.Strings(newLabels)

	session.Labels = newLabels

	// Save to persistent store
	if err := sm.store.Save(context.Background(), sessionID, session); err != nil {
		sm.logger.Warn("Failed to save session after updating labels", "error", err, "session_id", sessionID)
	}

	sm.logger.Info("Updated session labels",
		"session_id", sessionID,
		"labels", newLabels)
	return nil
}

// ClearSessionLabels removes all labels from a session
func (sm *SessionManager) ClearSessionLabels(sessionID string) error {
	return sm.UpdateSessionLabels(sessionID, []string{})
}
