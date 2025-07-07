package session

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// SessionValidator provides simple validation for session states
type SessionValidator struct {
	maxSessions       int
	maxDiskPerSession int64
	totalDiskLimit    int64
	sessionTTL        time.Duration
}

// NewSessionValidator creates a new session validator
func NewSessionValidator(maxSessions int, maxDiskPerSession, totalDiskLimit int64, sessionTTL time.Duration) *SessionValidator {
	return &SessionValidator{
		maxSessions:       maxSessions,
		maxDiskPerSession: maxDiskPerSession,
		totalDiskLimit:    totalDiskLimit,
		sessionTTL:        sessionTTL,
	}
}

// ValidateSessionState validates a session state
func (v *SessionValidator) ValidateSessionState(ctx context.Context, state *SessionState) error {
	// Basic validation
	if state == nil {
		return errors.NewError().Messagef("session state cannot be nil").WithLocation(

		// Validate session ID
		).Build()
	}

	if err := v.validateSessionID(state.SessionID); err != nil {
		return err
	}

	// Validate timestamps
	if state.CreatedAt.IsZero() {
		return errors.NewError().Messagef("creation timestamp is required").Build()
	}
	if state.ExpiresAt.Before(time.Now()) {
		return errors.NewError().Messagef("session has expired").WithLocation(

		// Validate workspace
		).Build()
	}

	if state.WorkspaceDir != "" {
		if err := v.validateWorkspace(state.WorkspaceDir); err != nil {
			return err
		}
	}

	// Validate disk usage
	if state.DiskUsage > v.maxDiskPerSession {
		return errors.NewError().Messagef("disk usage exceeds limit of %d bytes", v.maxDiskPerSession).Build(

		// ValidateSessionCreation validates session creation parameters
		)
	}

	return nil
}

func (v *SessionValidator) ValidateSessionCreation(userID string, currentSessions int) error {
	if userID == "" {
		return errors.NewError().Messagef("user ID is required").Build()
	}

	if currentSessions >= v.maxSessions {
		return errors.NewError().Messagef("maximum of %d sessions reached", v.maxSessions).Build(

		// ValidateDiskUsage validates disk usage across all sessions
		)
	}

	return nil
}

func (v *SessionValidator) ValidateDiskUsage(totalUsage int64) error {
	if totalUsage > v.totalDiskLimit {
		return errors.NewError().Messagef("total disk usage %d exceeds limit %d", totalUsage, v.totalDiskLimit).Build(

		// validateSessionID validates a session ID format
		)
	}
	return nil
}

func (v *SessionValidator) validateSessionID(sessionID string) error {
	if sessionID == "" {
		return errors.NewError().Messagef("session ID is required").Build()
	}

	if len(sessionID) < 8 {
		return errors.NewError().Messagef("session ID too short").WithLocation(

		// Basic character validation
		).Build()
	}

	for _, c := range sessionID {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-') {
			return errors.NewError().Messagef("invalid character in session ID").Build()
		}
	}

	return nil
}

// validateWorkspace validates workspace directory
func (v *SessionValidator) validateWorkspace(workspaceDir string) error {
	if workspaceDir == "" {
		return errors.NewError().Messagef("workspace directory is required").WithLocation(

		// Check if absolute path
		).Build()
	}

	if !filepath.IsAbs(workspaceDir) {
		return errors.NewError().Messagef("workspace directory must be an absolute path").WithLocation(

		// Check if exists
		).Build()
	}

	info, err := os.Stat(workspaceDir)
	if err != nil {
		if os.IsNotExist(err) {
			return errors.NewError().Messagef("workspace directory does not exist").Build()
		}
		return errors.NewError().Message("failed to stat workspace directory").Cause(err).WithLocation(

		// Check if directory
		).Build()
	}

	if !info.IsDir() {
		return errors.NewError().Messagef("workspace path is not a directory").WithLocation(

		// Basic path traversal check
		).Build()
	}

	if strings.Contains(workspaceDir, "..") {
		return errors.NewError().Messagef("security error: path traversal detected in workspace directory").Build(

		// ValidateImageReference validates an image reference
		)
	}

	return nil
}

func (v *SessionValidator) ValidateImageReference(imageRef string) error {
	if imageRef == "" {
		return errors.NewError().Messagef("image reference is required").WithLocation(

		// Basic format check
		).Build()
	}

	parts := strings.Split(imageRef, ":")
	if len(parts) > 2 {
		return errors.NewError().Messagef("invalid image reference format").WithLocation(

		// Validate repository name
		).Build()
	}

	repo := parts[0]
	if repo == "" {
		return errors.NewError().Messagef("repository name is required").WithLocation(

		// Basic character validation for repository
		).Build()
	}

	for _, c := range repo {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
			c == '/' || c == '-' || c == '_' || c == '.') {
			return errors.NewError().Messagef("invalid character in repository name").Build()
		}
	}

	return nil
}

// ValidateManifestPath validates a Kubernetes manifest path
func (v *SessionValidator) ValidateManifestPath(path string) error {
	if path == "" {
		return errors.NewError().Messagef("manifest path is required").WithLocation(

		// Check extension
		).Build()
	}

	if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
		return errors.NewError().Messagef("manifest must be a YAML file").WithLocation(

		// Check if exists
		).Build()
	}

	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return errors.NewError().Messagef("manifest file does not exist").Build()
		}
		return errors.NewError().Message("failed to stat manifest file").Cause(err).Build(

		// IsSessionExpired checks if a session has expired
		)
	}

	return nil
}

func (v *SessionValidator) IsSessionExpired(session *SessionState) bool {
	return time.Now().After(session.ExpiresAt)
}

// CalculateSessionExpiry calculates expiry time for a new session
func (v *SessionValidator) CalculateSessionExpiry() time.Time {
	return time.Now().Add(v.sessionTTL)
}
