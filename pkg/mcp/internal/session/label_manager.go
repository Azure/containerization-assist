package session

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	"github.com/rs/zerolog"
)

// LabelManager provides label management operations for sessions
type LabelManager struct {
	sessionManager *SessionManager
	validator      *LabelValidator
	logger         zerolog.Logger
}

// LabelValidator validates labels according to Kubernetes standards and custom rules
type LabelValidator struct {
	// Kubernetes label validation (RFC 1123)
	MaxLabelLength   int      // 63 characters
	MaxValueLength   int      // 63 characters
	AllowedPrefixes  []string // Allowed prefixes like "workflow.", "app."
	ReservedPrefixes []string // Reserved prefixes like "kubernetes.io/"

	// Custom validation rules
	RequiredLabels  []string                  // Labels that must be present
	ForbiddenLabels []string                  // Labels that are not allowed
	LabelPatterns   map[string]*regexp.Regexp // Pattern validation for specific labels
}

// NewLabelManager creates a new label manager
func NewLabelManager(sessionManager *SessionManager, logger zerolog.Logger) *LabelManager {
	validator := &LabelValidator{
		MaxLabelLength:   63,
		MaxValueLength:   63,
		AllowedPrefixes:  []string{"workflow.", "app.", "env.", "repo.", "tool.", "progress.", "status."},
		ReservedPrefixes: []string{"kubernetes.io/", "k8s.io/"},
		RequiredLabels:   []string{},
		ForbiddenLabels:  []string{},
		LabelPatterns:    make(map[string]*regexp.Regexp),
	}

	// Add standard label patterns
	validator.LabelPatterns["workflow.stage"] = regexp.MustCompile(`^(analysis|build|deploy|completed|failed)$`)
	validator.LabelPatterns["env"] = regexp.MustCompile(`^(dev|test|staging|prod|production)$`)
	validator.LabelPatterns["progress"] = regexp.MustCompile(`^(0|25|50|75|100)$`)

	return &LabelManager{
		sessionManager: sessionManager,
		validator:      validator,
		logger:         logger.With().Str("component", "label_manager").Logger(),
	}
}

// AddLabels adds labels to a session
func (lm *LabelManager) AddLabels(sessionID string, labels ...string) error {
	lm.logger.Debug().
		Str("session_id", sessionID).
		Strs("labels", labels).
		Msg("Adding labels to session")

	// Validate labels
	for _, label := range labels {
		if err := lm.validator.ValidateLabel(label); err != nil {
			return types.NewRichError("INVALID_LABEL", fmt.Sprintf("invalid label %q: %v", label, err), "validation_error")
		}
	}

	// Get session
	session, err := lm.sessionManager.GetSessionConcrete(sessionID)
	if err != nil {
		return types.NewRichError("SESSION_NOT_FOUND", fmt.Sprintf("failed to get session: %v", err), "session_error")
	}

	// Add labels (avoiding duplicates)
	existingLabels := make(map[string]bool)
	for _, existing := range session.Labels {
		existingLabels[existing] = true
	}

	for _, label := range labels {
		if !existingLabels[label] {
			session.Labels = append(session.Labels, label)
		}
	}

	// Save session
	err = lm.sessionManager.UpdateSession(sessionID, func(s interface{}) {
		if state, ok := s.(*SessionState); ok {
			state.Labels = session.Labels
		}
	})
	if err != nil {
		return types.NewRichError("SESSION_SAVE_FAILED", fmt.Sprintf("failed to save session: %v", err), "session_error")
	}

	lm.logger.Info().
		Str("session_id", sessionID).
		Strs("added_labels", labels).
		Int("total_labels", len(session.Labels)).
		Msg("Successfully added labels to session")

	return nil
}

// RemoveLabels removes labels from a session
func (lm *LabelManager) RemoveLabels(sessionID string, labels ...string) error {
	lm.logger.Debug().
		Str("session_id", sessionID).
		Strs("labels", labels).
		Msg("Removing labels from session")

	// Get session
	session, err := lm.sessionManager.GetSessionConcrete(sessionID)
	if err != nil {
		return types.NewRichError("SESSION_NOT_FOUND", fmt.Sprintf("failed to get session: %v", err), "session_error")
	}

	// Create map of labels to remove
	toRemove := make(map[string]bool)
	for _, label := range labels {
		toRemove[label] = true
	}

	// Filter out labels to remove
	var newLabels []string
	for _, existing := range session.Labels {
		if !toRemove[existing] {
			newLabels = append(newLabels, existing)
		}
	}

	session.Labels = newLabels

	// Save session
	err = lm.sessionManager.UpdateSession(sessionID, func(s interface{}) {
		if state, ok := s.(*SessionState); ok {
			state.Labels = session.Labels
		}
	})
	if err != nil {
		return types.NewRichError("SESSION_SAVE_FAILED", fmt.Sprintf("failed to save session: %v", err), "session_error")
	}

	lm.logger.Info().
		Str("session_id", sessionID).
		Strs("removed_labels", labels).
		Int("remaining_labels", len(session.Labels)).
		Msg("Successfully removed labels from session")

	return nil
}

// SetLabels sets the complete label set for a session (replaces existing)
func (lm *LabelManager) SetLabels(sessionID string, labels []string) error {
	lm.logger.Debug().
		Str("session_id", sessionID).
		Strs("labels", labels).
		Msg("Setting labels for session")

	// Validate all labels
	for _, label := range labels {
		if err := lm.validator.ValidateLabel(label); err != nil {
			return types.NewRichError("INVALID_LABEL", fmt.Sprintf("invalid label %q: %v", label, err), "validation_error")
		}
	}

	// Get session
	session, err := lm.sessionManager.GetSessionConcrete(sessionID)
	if err != nil {
		return types.NewRichError("SESSION_NOT_FOUND", fmt.Sprintf("failed to get session: %v", err), "session_error")
	}

	// Set labels (removing duplicates)
	uniqueLabels := lm.removeDuplicates(labels)
	session.Labels = uniqueLabels

	// Save session
	err = lm.sessionManager.UpdateSession(sessionID, func(s interface{}) {
		if state, ok := s.(*SessionState); ok {
			state.Labels = session.Labels
		}
	})
	if err != nil {
		return types.NewRichError("SESSION_SAVE_FAILED", fmt.Sprintf("failed to save session: %v", err), "session_error")
	}

	lm.logger.Info().
		Str("session_id", sessionID).
		Strs("labels", uniqueLabels).
		Msg("Successfully set labels for session")

	return nil
}

// GetLabels retrieves labels for a session
func (lm *LabelManager) GetLabels(sessionID string) ([]string, error) {
	session, err := lm.sessionManager.GetSessionConcrete(sessionID)
	if err != nil {
		return nil, types.NewRichError("SESSION_NOT_FOUND", fmt.Sprintf("failed to get session: %v", err), "session_error")
	}

	return session.Labels, nil
}

// SetK8sLabels sets Kubernetes labels for a session
func (lm *LabelManager) SetK8sLabels(sessionID string, labels map[string]string) error {
	lm.logger.Debug().
		Str("session_id", sessionID).
		Interface("k8s_labels", labels).
		Msg("Setting K8s labels for session")

	// Validate K8s labels
	for key, value := range labels {
		if err := lm.validator.ValidateK8sLabel(key, value); err != nil {
			return types.NewRichError("INVALID_K8S_LABEL", fmt.Sprintf("invalid K8s label %q=%q: %v", key, value, err), "validation_error")
		}
	}

	// Get session
	session, err := lm.sessionManager.GetSessionConcrete(sessionID)
	if err != nil {
		return types.NewRichError("SESSION_NOT_FOUND", fmt.Sprintf("failed to get session: %v", err), "session_error")
	}

	// Initialize K8sLabels if nil
	if session.K8sLabels == nil {
		session.K8sLabels = make(map[string]string)
	}

	// Set K8s labels
	for key, value := range labels {
		session.K8sLabels[key] = value
	}

	// Save session
	err = lm.sessionManager.UpdateSession(sessionID, func(s interface{}) {
		if state, ok := s.(*SessionState); ok {
			state.K8sLabels = session.K8sLabels
		}
	})
	if err != nil {
		return types.NewRichError("SESSION_SAVE_FAILED", fmt.Sprintf("failed to save session: %v", err), "session_error")
	}

	lm.logger.Info().
		Str("session_id", sessionID).
		Interface("k8s_labels", labels).
		Msg("Successfully set K8s labels for session")

	return nil
}

// AddK8sLabel adds a single Kubernetes label to a session
func (lm *LabelManager) AddK8sLabel(sessionID string, key, value string) error {
	return lm.SetK8sLabels(sessionID, map[string]string{key: value})
}

// RemoveK8sLabel removes a Kubernetes label from a session
func (lm *LabelManager) RemoveK8sLabel(sessionID string, key string) error {
	lm.logger.Debug().
		Str("session_id", sessionID).
		Str("key", key).
		Msg("Removing K8s label from session")

	// Get session
	session, err := lm.sessionManager.GetSessionConcrete(sessionID)
	if err != nil {
		return types.NewRichError("SESSION_NOT_FOUND", fmt.Sprintf("failed to get session: %v", err), "session_error")
	}

	// Remove K8s label
	if session.K8sLabels != nil {
		delete(session.K8sLabels, key)
	}

	// Save session
	err = lm.sessionManager.UpdateSession(sessionID, func(s interface{}) {
		if state, ok := s.(*SessionState); ok {
			state.Labels = session.Labels
		}
	})
	if err != nil {
		return types.NewRichError("SESSION_SAVE_FAILED", fmt.Sprintf("failed to save session: %v", err), "session_error")
	}

	lm.logger.Info().
		Str("session_id", sessionID).
		Str("removed_key", key).
		Msg("Successfully removed K8s label from session")

	return nil
}

// GetK8sLabels retrieves Kubernetes labels for a session
func (lm *LabelManager) GetK8sLabels(sessionID string) (map[string]string, error) {
	session, err := lm.sessionManager.GetSessionConcrete(sessionID)
	if err != nil {
		return nil, types.NewRichError("SESSION_NOT_FOUND", fmt.Sprintf("failed to get session: %v", err), "session_error")
	}

	if session.K8sLabels == nil {
		return make(map[string]string), nil
	}

	return session.K8sLabels, nil
}

// ValidateLabel validates a session label
func (v *LabelValidator) ValidateLabel(label string) error {
	if len(label) == 0 {
		return types.NewRichError("EMPTY_LABEL", "label cannot be empty", "validation_error")
	}

	if len(label) > v.MaxLabelLength {
		return types.NewRichError("LABEL_TOO_LONG", fmt.Sprintf("label exceeds maximum length of %d characters", v.MaxLabelLength), "validation_error")
	}

	// Check if label is forbidden
	for _, forbidden := range v.ForbiddenLabels {
		if label == forbidden {
			return types.NewRichError("FORBIDDEN_LABEL", fmt.Sprintf("label %q is forbidden", label), "validation_error")
		}
	}

	// Check reserved prefixes
	for _, reserved := range v.ReservedPrefixes {
		if strings.HasPrefix(label, reserved) {
			return types.NewRichError("RESERVED_LABEL_PREFIX", fmt.Sprintf("label uses reserved prefix %q", reserved), "validation_error")
		}
	}

	// Check pattern validation for specific labels
	if strings.Contains(label, "/") {
		parts := strings.SplitN(label, "/", 2)
		if len(parts) == 2 {
			prefix := parts[0]
			value := parts[1]

			if pattern, exists := v.LabelPatterns[prefix]; exists {
				if !pattern.MatchString(value) {
					return types.NewRichError("INVALID_LABEL_PATTERN", fmt.Sprintf("label value %q does not match required pattern for prefix %q", value, prefix), "validation_error")
				}
			}
		}
	}

	return nil
}

// ValidateK8sLabel validates a Kubernetes label key-value pair
func (v *LabelValidator) ValidateK8sLabel(key, value string) error {
	// Validate key
	if len(key) == 0 {
		return types.NewRichError("EMPTY_K8S_LABEL_KEY", "K8s label key cannot be empty", "validation_error")
	}

	if len(key) > v.MaxLabelLength {
		return types.NewRichError("K8S_LABEL_KEY_TOO_LONG", fmt.Sprintf("K8s label key exceeds maximum length of %d characters", v.MaxLabelLength), "validation_error")
	}

	// Validate value
	if len(value) > v.MaxValueLength {
		return types.NewRichError("K8S_LABEL_VALUE_TOO_LONG", fmt.Sprintf("K8s label value exceeds maximum length of %d characters", v.MaxValueLength), "validation_error")
	}

	// Check Kubernetes label naming conventions (simplified)
	k8sLabelRegex := regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9\-_\.]*[a-zA-Z0-9])?)?$`)
	if !k8sLabelRegex.MatchString(key) {
		return types.NewRichError("INVALID_K8S_LABEL_KEY_FORMAT", fmt.Sprintf("K8s label key %q does not follow Kubernetes naming conventions", key), "validation_error")
	}

	if value != "" && !k8sLabelRegex.MatchString(value) {
		return types.NewRichError("INVALID_K8S_LABEL_VALUE_FORMAT", fmt.Sprintf("K8s label value %q does not follow Kubernetes naming conventions", value), "validation_error")
	}

	return nil
}

// removeDuplicates removes duplicate labels from a slice
func (lm *LabelManager) removeDuplicates(labels []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, label := range labels {
		if !seen[label] {
			seen[label] = true
			result = append(result, label)
		}
	}

	return result
}
