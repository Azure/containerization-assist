package session

import (
	"context"
	"regexp"
	"strings"

	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
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
func (lm *LabelManager) AddLabels(ctx context.Context, sessionID string, labels ...string) error {
	lm.logger.Debug().
		Str("session_id", sessionID).
		Strs("labels", labels).
		Msg("Adding labels to session")

	// Validate labels
	for _, label := range labels {
		if err := lm.validator.ValidateLabel(label); err != nil {
			return errors.NewError().Messagef("invalid label %q: %v", label, err).WithLocation(

			// Get session
			).Build()
		}
	}

	session, err := lm.sessionManager.GetSessionConcrete(sessionID)
	if err != nil {
		return errors.NewError().Messagef("failed to get session: %v", err).WithLocation(

		// Add labels (avoiding duplicates)
		).Build()
	}

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
	err = lm.sessionManager.UpdateSession(ctx, sessionID, func(s *SessionState) error {
		s.Labels = session.Labels
		return nil
	})
	if err != nil {
		return errors.NewError().Messagef("failed to save session: %v", err).Build()
	}

	lm.logger.Info().
		Str("session_id", sessionID).
		Strs("added_labels", labels).
		Int("total_labels", len(session.Labels)).
		Msg("Successfully added labels to session")

	return nil
}

// RemoveLabels removes labels from a session
func (lm *LabelManager) RemoveLabels(ctx context.Context, sessionID string, labels ...string) error {
	lm.logger.Debug().
		Str("session_id", sessionID).
		Strs("labels", labels).
		Msg("Removing labels from session")

	// Get session
	session, err := lm.sessionManager.GetSessionConcrete(sessionID)
	if err != nil {
		return errors.NewError().Messagef("failed to get session: %v", err).WithLocation(

		// Create map of labels to remove
		).Build()
	}

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
	err = lm.sessionManager.UpdateSession(ctx, sessionID, func(s *SessionState) error {
		s.Labels = session.Labels
		return nil
	})
	if err != nil {
		return errors.NewError().Messagef("failed to save session: %v", err).Build()
	}

	lm.logger.Info().
		Str("session_id", sessionID).
		Strs("removed_labels", labels).
		Int("remaining_labels", len(session.Labels)).
		Msg("Successfully removed labels from session")

	return nil
}

// SetLabels sets the complete label set for a session (replaces existing)
func (lm *LabelManager) SetLabels(ctx context.Context, sessionID string, labels []string) error {
	lm.logger.Debug().
		Str("session_id", sessionID).
		Strs("labels", labels).
		Msg("Setting labels for session")

	// Validate all labels
	for _, label := range labels {
		if err := lm.validator.ValidateLabel(label); err != nil {
			return errors.NewError().Messagef("invalid label %q: %v", label, err).WithLocation(

			// Get session
			).Build()
		}
	}

	session, err := lm.sessionManager.GetSessionConcrete(sessionID)
	if err != nil {
		return errors.NewError().Messagef("failed to get session: %v", err).WithLocation(

		// Set labels (removing duplicates)
		).Build()
	}

	uniqueLabels := lm.removeDuplicates(labels)
	session.Labels = uniqueLabels

	// Save session
	err = lm.sessionManager.UpdateSession(ctx, sessionID, func(s *SessionState) error {
		s.Labels = session.Labels
		return nil
	})
	if err != nil {
		return errors.NewError().Messagef("failed to save session: %v", err).Build()
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
		return nil, errors.NewError().Messagef("failed to get session: %v", err).Build()
	}

	return session.Labels, nil
}

// SetK8sLabels sets Kubernetes labels for a session
func (lm *LabelManager) SetK8sLabels(ctx context.Context, sessionID string, labels map[string]string) error {
	lm.logger.Debug().
		Str("session_id", sessionID).
		Interface("k8s_labels", labels).
		Msg("Setting K8s labels for session")

	// Validate K8s labels
	for key, value := range labels {
		if err := lm.validator.ValidateK8sLabel(key, value); err != nil {
			return errors.NewError().Messagef("invalid K8s label %q=%q: %v", key, value, err).WithLocation(

			// Get session
			).Build()
		}
	}

	session, err := lm.sessionManager.GetSessionConcrete(sessionID)
	if err != nil {
		return errors.NewError().Messagef("failed to get session: %v", err).WithLocation(

		// Initialize K8sLabels if nil
		).Build()
	}

	if session.K8sLabels == nil {
		session.K8sLabels = make(map[string]string)
	}

	// Set K8s labels
	for key, value := range labels {
		session.K8sLabels[key] = value
	}

	// Save session
	err = lm.sessionManager.UpdateSession(ctx, sessionID, func(s *SessionState) error {
		s.K8sLabels = session.K8sLabels
		return nil
	})
	if err != nil {
		return errors.NewError().Messagef("failed to save session: %v", err).Build()
	}

	lm.logger.Info().
		Str("session_id", sessionID).
		Interface("k8s_labels", labels).
		Msg("Successfully set K8s labels for session")

	return nil
}

// AddK8sLabel adds a single Kubernetes label to a session
func (lm *LabelManager) AddK8sLabel(ctx context.Context, sessionID string, key, value string) error {
	return lm.SetK8sLabels(ctx, sessionID, map[string]string{key: value})
}

// RemoveK8sLabel removes a Kubernetes label from a session
func (lm *LabelManager) RemoveK8sLabel(ctx context.Context, sessionID string, key string) error {
	lm.logger.Debug().
		Str("session_id", sessionID).
		Str("key", key).
		Msg("Removing K8s label from session")

	// Get session
	session, err := lm.sessionManager.GetSessionConcrete(sessionID)
	if err != nil {
		return errors.NewError().Messagef("failed to get session: %v", err).WithLocation(

		// Remove K8s label
		).Build()
	}

	if session.K8sLabels != nil {
		delete(session.K8sLabels, key)
	}

	// Save session
	err = lm.sessionManager.UpdateSession(ctx, sessionID, func(s *SessionState) error {
		s.K8sLabels = session.K8sLabels
		return nil
	})
	if err != nil {
		return errors.NewError().Messagef("failed to save session: %v", err).Build()
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
		return nil, errors.NewError().Messagef("failed to get session: %v", err).Build()
	}

	if session.K8sLabels == nil {
		return make(map[string]string), nil
	}

	return session.K8sLabels, nil
}

// ValidateLabel validates a session label
func (v *LabelValidator) ValidateLabel(label string) error {
	if len(label) == 0 {
		return errors.NewError().Messagef("label cannot be empty").Build()
	}

	if len(label) > v.MaxLabelLength {
		return errors.NewError().Messagef("label exceeds maximum length of %d characters", v.MaxLabelLength).WithLocation(

		// Check if label is forbidden
		).Build()
	}

	for _, forbidden := range v.ForbiddenLabels {
		if label == forbidden {
			return errors.NewError().Messagef("label %q is forbidden", label).WithLocation(

			// Check reserved prefixes
			).Build()
		}
	}

	for _, reserved := range v.ReservedPrefixes {
		if strings.HasPrefix(label, reserved) {
			return errors.NewError().Messagef("label uses reserved prefix %q", reserved).WithLocation(

			// Check pattern validation for specific labels
			).Build()
		}
	}

	if strings.Contains(label, "/") {
		parts := strings.SplitN(label, "/", 2)
		if len(parts) == 2 {
			prefix := parts[0]
			value := parts[1]

			if pattern, exists := v.LabelPatterns[prefix]; exists {
				if !pattern.MatchString(value) {
					return errors.NewError().Messagef("label value %q does not match required pattern for prefix %q", value, prefix).Build()
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
		return errors.NewError().Messagef("K8s label key cannot be empty").Build()
	}

	if len(key) > v.MaxLabelLength {
		return errors.NewError().Messagef("K8s label key exceeds maximum length of %d characters", v.MaxLabelLength).WithLocation(

		// Validate value
		).Build()
	}

	if len(value) > v.MaxValueLength {
		return errors.NewError().Messagef("K8s label value exceeds maximum length of %d characters", v.MaxValueLength).WithLocation(

		// Check Kubernetes label naming conventions (simplified)
		).Build()
	}

	k8sLabelRegex := regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9\-_\.]*[a-zA-Z0-9])?)?$`)
	if !k8sLabelRegex.MatchString(key) {
		return errors.NewError().Messagef("K8s label key %q does not follow Kubernetes naming conventions", key).Build()
	}

	if value != "" && !k8sLabelRegex.MatchString(value) {
		return errors.NewError().Messagef("K8s label value %q does not follow Kubernetes naming conventions", value).Build(

		// removeDuplicates removes duplicate labels from a slice
		)
	}

	return nil
}

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
