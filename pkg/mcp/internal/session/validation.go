package session

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/Azure/container-kit/pkg/mcp/validation/core"
	"github.com/Azure/container-kit/pkg/mcp/validation/validators"
)

// SessionValidator provides unified validation for session states and operations
type SessionValidator struct {
	*validators.BaseValidatorImpl
	workspaceValidator *validators.SecurityValidator
	formatValidator    *validators.FormatValidator
	maxSessions        int
	maxDiskPerSession  int64
	totalDiskLimit     int64
	sessionTTL         time.Duration
}

// NewSessionValidator creates a new session validator
func NewSessionValidator(maxSessions int, maxDiskPerSession, totalDiskLimit int64, sessionTTL time.Duration) *SessionValidator {
	return &SessionValidator{
		BaseValidatorImpl:  validators.NewBaseValidator("session", "1.0.0", []string{"session", "state", "workspace"}),
		workspaceValidator: validators.NewSecurityValidator(),
		formatValidator:    validators.NewFormatValidator(),
		maxSessions:        maxSessions,
		maxDiskPerSession:  maxDiskPerSession,
		totalDiskLimit:     totalDiskLimit,
		sessionTTL:         sessionTTL,
	}
}

// ValidateSessionState validates a complete session state
func (s *SessionValidator) ValidateSessionState(ctx context.Context, state *SessionState, options *core.ValidationOptions) *core.ValidationResult {
	startTime := time.Now()
	result := &core.ValidationResult{
		Valid:    true,
		Errors:   make([]*core.ValidationError, 0),
		Warnings: make([]*core.ValidationWarning, 0),
		Metadata: core.ValidationMetadata{
			ValidatedAt:      startTime,
			ValidatorName:    "session-state-validator",
			ValidatorVersion: "1.0.0",
			Context:          make(map[string]interface{}),
		},
		Suggestions: make([]string, 0),
	}

	if state == nil {
		result.AddError(&core.ValidationError{
			Code:     "SESSION_STATE_NULL",
			Message:  "Session state cannot be null",
			Type:     core.ErrTypeValidation,
			Severity: core.SeverityCritical,
		})
		return result
	}

	// Validate session ID
	s.validateSessionID(state.SessionID, result)

	// Validate workspace directory
	s.validateWorkspaceDirectory(state.WorkspaceDir, result)

	// Validate timestamps
	s.validateTimestamps(state, result)

	// Validate repository information
	s.validateRepositoryInfo(state, result)

	// Validate image reference
	s.validateImageReference(state.ImageRef, result)

	// Validate dockerfile state
	s.validateDockerfileState(state.Dockerfile, result)

	// Validate security scan data
	s.validateSecurityScan(state.SecurityScan, result)

	// Validate Kubernetes manifests
	s.validateKubernetesManifests(state.K8sManifests, result)

	// Validate disk usage
	s.validateDiskUsage(state, result)

	// Validate active jobs
	s.validateActiveJobs(state.ActiveJobs, result)

	// Validate metadata
	s.validateMetadata(state.Metadata, result)

	// Calculate final score and duration
	s.calculateValidationScore(result)
	result.Duration = time.Since(startTime)

	return result
}

// ValidateSessionCreationArgs validates session creation arguments
func (s *SessionValidator) ValidateSessionCreationArgs(ctx context.Context, args interface{}, options *core.ValidationOptions) *core.ValidationResult {
	result := &core.ValidationResult{
		Valid:    true,
		Errors:   make([]*core.ValidationError, 0),
		Warnings: make([]*core.ValidationWarning, 0),
		Metadata: core.ValidationMetadata{
			ValidatedAt:      time.Now(),
			ValidatorName:    "session-creation-validator",
			ValidatorVersion: "1.0.0",
			Context:          make(map[string]interface{}),
		},
		Suggestions: make([]string, 0),
	}

	// Type check arguments
	switch v := args.(type) {
	case map[string]interface{}:
		s.validateCreationArgsMap(v, result, options)
	case SessionManagerConfig:
		s.validateCreationConfig(v, result, options)
	default:
		result.AddError(&core.ValidationError{
			Code:     "INVALID_SESSION_ARGS",
			Message:  fmt.Sprintf("Expected session creation arguments, got %T", args),
			Type:     core.ErrTypeValidation,
			Severity: core.SeverityHigh,
		})
	}

	return result
}

// ValidateSessionManager validates session manager state and configuration
func (s *SessionValidator) ValidateSessionManager(ctx context.Context, manager *SessionManager, options *core.ValidationOptions) *core.ValidationResult {
	result := &core.ValidationResult{
		Valid:    true,
		Errors:   make([]*core.ValidationError, 0),
		Warnings: make([]*core.ValidationWarning, 0),
		Metadata: core.ValidationMetadata{
			ValidatedAt:      time.Now(),
			ValidatorName:    "session-manager-validator",
			ValidatorVersion: "1.0.0",
			Context:          make(map[string]interface{}),
		},
		Suggestions: make([]string, 0),
	}

	if manager == nil {
		result.AddError(&core.ValidationError{
			Code:     "SESSION_MANAGER_NULL",
			Message:  "Session manager cannot be null",
			Type:     core.ErrTypeValidation,
			Severity: core.SeverityCritical,
		})
		return result
	}

	// Validate workspace directory exists and is accessible
	s.validateWorkspaceAccess(manager.workspaceDir, result)

	// Validate session limits
	s.validateSessionLimits(len(manager.sessions), manager.maxSessions, result)

	// Validate disk usage across all sessions
	s.validateTotalDiskUsage(manager, result)

	// Validate session TTL configuration
	s.validateSessionTTL(manager.sessionTTL, result)

	// Validate individual sessions
	s.validateAllSessions(manager.sessions, result)

	return result
}

// Helper validation methods

func (s *SessionValidator) validateSessionID(sessionID string, result *core.ValidationResult) {
	if sessionID == "" {
		result.AddError(&core.ValidationError{
			Code:     "EMPTY_SESSION_ID",
			Message:  "Session ID cannot be empty",
			Type:     core.ErrTypeValidation,
			Severity: core.SeverityCritical,
			Field:    "session_id",
		})
		return
	}

	// Validate session ID format (hex string)
	if len(sessionID) != 32 {
		result.AddWarning(&core.ValidationWarning{
			ValidationError: &core.ValidationError{
				Code:     "INVALID_SESSION_ID_LENGTH",
				Message:  fmt.Sprintf("Session ID should be 32 characters, got %d", len(sessionID)),
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityMedium,
				Field:    "session_id",
			},
		})
	}

	// Check for valid hex characters
	for _, char := range sessionID {
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')) {
			result.AddError(&core.ValidationError{
				Code:     "INVALID_SESSION_ID_FORMAT",
				Message:  "Session ID must be a valid hexadecimal string",
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityHigh,
				Field:    "session_id",
			})
			break
		}
	}
}

func (s *SessionValidator) validateWorkspaceDirectory(workspaceDir string, result *core.ValidationResult) {
	if workspaceDir == "" {
		result.AddError(&core.ValidationError{
			Code:     "EMPTY_WORKSPACE_DIR",
			Message:  "Workspace directory cannot be empty",
			Type:     core.ErrTypeValidation,
			Severity: core.SeverityCritical,
			Field:    "workspace_dir",
		})
		return
	}

	// Check if path is absolute
	if !filepath.IsAbs(workspaceDir) {
		result.AddError(&core.ValidationError{
			Code:     "RELATIVE_WORKSPACE_PATH",
			Message:  "Workspace directory must be an absolute path",
			Type:     core.ErrTypeValidation,
			Severity: core.SeverityHigh,
			Field:    "workspace_dir",
		})
	}

	// Security check for dangerous paths
	dangerousPaths := []string{"/", "/root", "/etc", "/usr", "/var", "/bin", "/sbin"}
	for _, dangerous := range dangerousPaths {
		if workspaceDir == dangerous || strings.HasPrefix(workspaceDir, dangerous+"/") {
			result.AddError(&core.ValidationError{
				Code:     "DANGEROUS_WORKSPACE_PATH",
				Message:  fmt.Sprintf("Workspace path %s is in a dangerous system directory", workspaceDir),
				Type:     core.ErrTypeSecurity,
				Severity: core.SeverityCritical,
				Field:    "workspace_dir",
			})
		}
	}

	// Check for directory traversal patterns
	if strings.Contains(workspaceDir, "..") {
		result.AddWarning(&core.ValidationWarning{
			ValidationError: &core.ValidationError{
				Code:     "DIRECTORY_TRAVERSAL_PATTERN",
				Message:  "Workspace path contains directory traversal pattern",
				Type:     core.ErrTypeSecurity,
				Severity: core.SeverityMedium,
				Field:    "workspace_dir",
			},
		})
	}
}

func (s *SessionValidator) validateTimestamps(state *SessionState, result *core.ValidationResult) {
	now := time.Now()

	// Check if creation time is in the future
	if state.CreatedAt.After(now) {
		result.AddError(&core.ValidationError{
			Code:     "FUTURE_CREATION_TIME",
			Message:  "Session creation time cannot be in the future",
			Type:     core.ErrTypeValidation,
			Severity: core.SeverityHigh,
			Field:    "created_at",
		})
	}

	// Check if last accessed is before creation
	if state.LastAccessed.Before(state.CreatedAt) {
		result.AddError(&core.ValidationError{
			Code:     "INVALID_ACCESS_TIME",
			Message:  "Last accessed time cannot be before creation time",
			Type:     core.ErrTypeValidation,
			Severity: core.SeverityHigh,
			Field:    "last_accessed",
		})
	}

	// Check if expires at is before creation
	if state.ExpiresAt.Before(state.CreatedAt) {
		result.AddError(&core.ValidationError{
			Code:     "INVALID_EXPIRY_TIME",
			Message:  "Expiry time cannot be before creation time",
			Type:     core.ErrTypeValidation,
			Severity: core.SeverityHigh,
			Field:    "expires_at",
		})
	}

	// Warn if session has expired
	if state.ExpiresAt.Before(now) {
		result.AddWarning(&core.ValidationWarning{
			ValidationError: &core.ValidationError{
				Code:     "SESSION_EXPIRED",
				Message:  fmt.Sprintf("Session expired %v ago", now.Sub(state.ExpiresAt)),
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityMedium,
				Field:    "expires_at",
			},
		})
	}

	// Check for reasonable session duration
	sessionAge := now.Sub(state.CreatedAt)
	if sessionAge > 30*24*time.Hour { // 30 days
		result.AddWarning(&core.ValidationWarning{
			ValidationError: &core.ValidationError{
				Code:     "LONG_RUNNING_SESSION",
				Message:  fmt.Sprintf("Session has been running for %v, consider cleanup", sessionAge),
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityLow,
				Field:    "created_at",
			},
		})
	}
}

func (s *SessionValidator) validateRepositoryInfo(state *SessionState, result *core.ValidationResult) {
	// Validate repository path
	if state.RepoPath != "" {
		if !filepath.IsAbs(state.RepoPath) {
			result.AddWarning(&core.ValidationWarning{
				ValidationError: &core.ValidationError{
					Code:     "RELATIVE_REPO_PATH",
					Message:  "Repository path should be absolute",
					Type:     core.ErrTypeValidation,
					Severity: core.SeverityLow,
					Field:    "repo_path",
				},
			})
		}

		// Check if repo path is within workspace
		if state.WorkspaceDir != "" && !strings.HasPrefix(state.RepoPath, state.WorkspaceDir) {
			result.AddWarning(&core.ValidationWarning{
				ValidationError: &core.ValidationError{
					Code:     "REPO_OUTSIDE_WORKSPACE",
					Message:  "Repository path is outside workspace directory",
					Type:     core.ErrTypeSecurity,
					Severity: core.SeverityMedium,
					Field:    "repo_path",
				},
			})
		}
	}

	// Validate repository URL format if provided
	if state.RepoURL != "" {
		if !strings.HasPrefix(state.RepoURL, "http://") && !strings.HasPrefix(state.RepoURL, "https://") && !strings.HasPrefix(state.RepoURL, "git@") {
			result.AddWarning(&core.ValidationWarning{
				ValidationError: &core.ValidationError{
					Code:     "INVALID_REPO_URL_FORMAT",
					Message:  "Repository URL format may be invalid",
					Type:     core.ErrTypeValidation,
					Severity: core.SeverityLow,
					Field:    "repo_url",
				},
			})
		}
	}
}

func (s *SessionValidator) validateImageReference(imageRef types.ImageReference, result *core.ValidationResult) {
	if imageRef.Registry == "" && imageRef.Repository == "" && imageRef.Tag == "" {
		// Empty image reference is ok
		return
	}

	// Validate repository name
	if imageRef.Repository == "" {
		result.AddError(&core.ValidationError{
			Code:     "EMPTY_IMAGE_REPOSITORY",
			Message:  "Image repository cannot be empty when image reference is specified",
			Type:     core.ErrTypeValidation,
			Severity: core.SeverityHigh,
			Field:    "image_ref.repository",
		})
	}

	// Validate tag
	if imageRef.Tag == "" {
		result.AddWarning(&core.ValidationWarning{
			ValidationError: &core.ValidationError{
				Code:     "MISSING_IMAGE_TAG",
				Message:  "Image tag is missing, 'latest' will be used",
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityLow,
				Field:    "image_ref.tag",
			},
		})
	}

	// Warn about using 'latest' tag
	if imageRef.Tag == "latest" {
		result.AddWarning(&core.ValidationWarning{
			ValidationError: &core.ValidationError{
				Code:     "LATEST_TAG_WARNING",
				Message:  "Using 'latest' tag may lead to inconsistent deployments",
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityLow,
				Field:    "image_ref.tag",
			},
		})
	}
}

func (s *SessionValidator) validateDockerfileState(dockerfile DockerfileState, result *core.ValidationResult) {
	if dockerfile.Content == "" {
		result.AddWarning(&core.ValidationWarning{
			ValidationError: &core.ValidationError{
				Code:     "EMPTY_DOCKERFILE_CONTENT",
				Message:  "Dockerfile content is empty",
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityMedium,
				Field:    "dockerfile.content",
			},
		})
	}

	// Validate build state consistency
	if dockerfile.Built && dockerfile.ImageID == "" {
		result.AddWarning(&core.ValidationWarning{
			ValidationError: &core.ValidationError{
				Code:     "BUILT_WITHOUT_IMAGE_ID",
				Message:  "Dockerfile marked as built but has no image ID",
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityMedium,
				Field:    "dockerfile.image_id",
			},
		})
	}

	// Validate push state consistency
	if dockerfile.Pushed && !dockerfile.Built {
		result.AddError(&core.ValidationError{
			Code:     "PUSHED_WITHOUT_BUILD",
			Message:  "Dockerfile marked as pushed but not built",
			Type:     core.ErrTypeValidation,
			Severity: core.SeverityHigh,
			Field:    "dockerfile.pushed",
		})
	}

	// Validate size for built images
	if dockerfile.Built && dockerfile.Size <= 0 {
		result.AddWarning(&core.ValidationWarning{
			ValidationError: &core.ValidationError{
				Code:     "MISSING_IMAGE_SIZE",
				Message:  "Built image has no size information",
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityLow,
				Field:    "dockerfile.size",
			},
		})
	}

	// Warn about very large images
	if dockerfile.Size > 2*1024*1024*1024 { // 2GB
		result.AddWarning(&core.ValidationWarning{
			ValidationError: &core.ValidationError{
				Code:     "LARGE_IMAGE_SIZE",
				Message:  fmt.Sprintf("Image size is very large: %d bytes", dockerfile.Size),
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityLow,
				Field:    "dockerfile.size",
			},
		})
	}
}

func (s *SessionValidator) validateSecurityScan(scan *SecurityScanSummary, result *core.ValidationResult) {
	if scan == nil {
		return
	}

	// Validate scan timestamps
	if scan.ScannedAt.IsZero() {
		result.AddWarning(&core.ValidationWarning{
			ValidationError: &core.ValidationError{
				Code:     "MISSING_SCAN_TIMESTAMP",
				Message:  "Security scan timestamp is missing",
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityLow,
				Field:    "security_scan.scanned_at",
			},
		})
	}

	// Check for old scans
	if !scan.ScannedAt.IsZero() && time.Since(scan.ScannedAt) > 7*24*time.Hour {
		result.AddWarning(&core.ValidationWarning{
			ValidationError: &core.ValidationError{
				Code:     "OUTDATED_SECURITY_SCAN",
				Message:  fmt.Sprintf("Security scan is %v old, consider rescanning", time.Since(scan.ScannedAt)),
				Type:     core.ErrTypeSecurity,
				Severity: core.SeverityMedium,
				Field:    "security_scan.scanned_at",
			},
		})
	}

	// Validate scan success
	if !scan.Success {
		result.AddError(&core.ValidationError{
			Code:     "SECURITY_SCAN_FAILED",
			Message:  "Security scan failed",
			Type:     core.ErrTypeSecurity,
			Severity: core.SeverityHigh,
			Field:    "security_scan.success",
		})
	}

	// Validate vulnerability counts
	if scan.Summary.Critical > 0 {
		result.AddError(&core.ValidationError{
			Code:     "CRITICAL_SECURITY_ISSUES",
			Message:  fmt.Sprintf("Found %d critical security issues", scan.Summary.Critical),
			Type:     core.ErrTypeSecurity,
			Severity: core.SeverityCritical,
			Field:    "security_scan.summary.critical",
		})
	}

	if scan.Summary.High > 10 {
		result.AddWarning(&core.ValidationWarning{
			ValidationError: &core.ValidationError{
				Code:     "HIGH_SECURITY_ISSUES",
				Message:  fmt.Sprintf("Found %d high severity security issues", scan.Summary.High),
				Type:     core.ErrTypeSecurity,
				Severity: core.SeverityHigh,
				Field:    "security_scan.summary.high",
			},
		})
	}

	// Validate scanner information
	if scan.Scanner == "" {
		result.AddWarning(&core.ValidationWarning{
			ValidationError: &core.ValidationError{
				Code:     "MISSING_SCANNER_INFO",
				Message:  "Scanner information is missing",
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityLow,
				Field:    "security_scan.scanner",
			},
		})
	}

	// Validate image reference
	if scan.ImageRef == "" {
		result.AddWarning(&core.ValidationWarning{
			ValidationError: &core.ValidationError{
				Code:     "MISSING_SCAN_IMAGE_REF",
				Message:  "Scanned image reference is missing",
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityLow,
				Field:    "security_scan.image_ref",
			},
		})
	}
}

func (s *SessionValidator) validateKubernetesManifests(manifests map[string]types.K8sManifest, result *core.ValidationResult) {
	for name, manifest := range manifests {
		if manifest.Kind == "" {
			result.AddError(&core.ValidationError{
				Code:     "MISSING_MANIFEST_KIND",
				Message:  fmt.Sprintf("Kubernetes manifest '%s' is missing kind", name),
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityHigh,
				Field:    fmt.Sprintf("k8s_manifests.%s.kind", name),
			})
		}

		// Note: K8sManifest doesn't have APIVersion field, it's embedded in Content
		// We could parse the content to validate APIVersion but that's expensive

		if manifest.Content == "" {
			result.AddWarning(&core.ValidationWarning{
				ValidationError: &core.ValidationError{
					Code:     "EMPTY_MANIFEST_CONTENT",
					Message:  fmt.Sprintf("Kubernetes manifest '%s' has empty content", name),
					Type:     core.ErrTypeValidation,
					Severity: core.SeverityMedium,
					Field:    fmt.Sprintf("k8s_manifests.%s.content", name),
				},
			})
		}
	}
}

func (s *SessionValidator) validateDiskUsage(state *SessionState, result *core.ValidationResult) {
	if state.DiskUsage < 0 {
		result.AddError(&core.ValidationError{
			Code:     "NEGATIVE_DISK_USAGE",
			Message:  "Disk usage cannot be negative",
			Type:     core.ErrTypeValidation,
			Severity: core.SeverityHigh,
			Field:    "disk_usage",
		})
	}

	if state.MaxDiskUsage > 0 && state.DiskUsage > state.MaxDiskUsage {
		result.AddError(&core.ValidationError{
			Code:     "DISK_USAGE_EXCEEDED",
			Message:  fmt.Sprintf("Disk usage (%d bytes) exceeds maximum (%d bytes)", state.DiskUsage, state.MaxDiskUsage),
			Type:     core.ErrTypeSystem,
			Severity: core.SeverityCritical,
			Field:    "disk_usage",
		})
	}

	if s.maxDiskPerSession > 0 && state.DiskUsage > s.maxDiskPerSession {
		result.AddError(&core.ValidationError{
			Code:     "SESSION_DISK_LIMIT_EXCEEDED",
			Message:  fmt.Sprintf("Session disk usage (%d bytes) exceeds per-session limit (%d bytes)", state.DiskUsage, s.maxDiskPerSession),
			Type:     core.ErrTypeSystem,
			Severity: core.SeverityHigh,
			Field:    "disk_usage",
		})
	}

	// Warn if approaching limits
	if s.maxDiskPerSession > 0 && float64(state.DiskUsage) > float64(s.maxDiskPerSession)*0.8 {
		result.AddWarning(&core.ValidationWarning{
			ValidationError: &core.ValidationError{
				Code:     "APPROACHING_DISK_LIMIT",
				Message:  fmt.Sprintf("Session is using %d%% of allowed disk space", int(float64(state.DiskUsage)/float64(s.maxDiskPerSession)*100)),
				Type:     core.ErrTypeSystem,
				Severity: core.SeverityMedium,
				Field:    "disk_usage",
			},
		})
	}
}

func (s *SessionValidator) validateActiveJobs(jobs map[string]JobInfo, result *core.ValidationResult) {
	for jobID, job := range jobs {
		if job.Status == "" {
			result.AddWarning(&core.ValidationWarning{
				ValidationError: &core.ValidationError{
					Code:     "MISSING_JOB_STATUS",
					Message:  fmt.Sprintf("Job '%s' is missing status", jobID),
					Type:     core.ErrTypeValidation,
					Severity: core.SeverityLow,
					Field:    fmt.Sprintf("active_jobs.%s.status", jobID),
				},
			})
		}

		// Check for long-running jobs
		if !job.StartTime.IsZero() && time.Since(job.StartTime) > 1*time.Hour {
			result.AddWarning(&core.ValidationWarning{
				ValidationError: &core.ValidationError{
					Code:     "LONG_RUNNING_JOB",
					Message:  fmt.Sprintf("Job '%s' has been running for %v", jobID, time.Since(job.StartTime)),
					Type:     core.ErrTypeSystem,
					Severity: core.SeverityMedium,
					Field:    fmt.Sprintf("active_jobs.%s.start_time", jobID),
				},
			})
		}
	}
}

func (s *SessionValidator) validateMetadata(metadata map[string]interface{}, result *core.ValidationResult) {
	// Check for reasonable metadata size
	if len(metadata) > 100 {
		result.AddWarning(&core.ValidationWarning{
			ValidationError: &core.ValidationError{
				Code:     "LARGE_METADATA",
				Message:  fmt.Sprintf("Session metadata contains %d entries, consider cleanup", len(metadata)),
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityLow,
				Field:    "metadata",
			},
		})
	}

	// Check for sensitive data patterns in metadata keys/values
	sensitivePatterns := []string{"password", "secret", "key", "token", "credential"}
	for key, value := range metadata {
		keyLower := strings.ToLower(key)
		for _, pattern := range sensitivePatterns {
			if strings.Contains(keyLower, pattern) {
				result.AddWarning(&core.ValidationWarning{
					ValidationError: &core.ValidationError{
						Code:     "SENSITIVE_METADATA_KEY",
						Message:  fmt.Sprintf("Metadata key '%s' may contain sensitive information", key),
						Type:     core.ErrTypeSecurity,
						Severity: core.SeverityMedium,
						Field:    fmt.Sprintf("metadata.%s", key),
					},
				})
			}
		}

		// Check string values for sensitive patterns
		if str, ok := value.(string); ok {
			strLower := strings.ToLower(str)
			for _, pattern := range sensitivePatterns {
				if strings.Contains(strLower, pattern) {
					result.AddWarning(&core.ValidationWarning{
						ValidationError: &core.ValidationError{
							Code:     "SENSITIVE_METADATA_VALUE",
							Message:  fmt.Sprintf("Metadata value for key '%s' may contain sensitive information", key),
							Type:     core.ErrTypeSecurity,
							Severity: core.SeverityMedium,
							Field:    fmt.Sprintf("metadata.%s", key),
						},
					})
				}
			}
		}
	}
}

func (s *SessionValidator) validateCreationArgsMap(args map[string]interface{}, result *core.ValidationResult, options *core.ValidationOptions) {
	// Validate workspace directory
	if workspaceDir, exists := args["workspace_dir"]; exists {
		if dir, ok := workspaceDir.(string); ok {
			s.validateWorkspaceDirectory(dir, result)
		} else {
			result.AddError(&core.ValidationError{
				Code:     "INVALID_WORKSPACE_DIR_TYPE",
				Message:  "Workspace directory must be a string",
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityHigh,
				Field:    "workspace_dir",
			})
		}
	}

	// Validate session limits
	if maxSessions, exists := args["max_sessions"]; exists {
		if max, ok := maxSessions.(int); ok {
			if max <= 0 {
				result.AddError(&core.ValidationError{
					Code:     "INVALID_MAX_SESSIONS",
					Message:  "Maximum sessions must be positive",
					Type:     core.ErrTypeValidation,
					Severity: core.SeverityHigh,
					Field:    "max_sessions",
				})
			}
		}
	}

	// Validate TTL
	if ttl, exists := args["session_ttl"]; exists {
		if duration, ok := ttl.(time.Duration); ok {
			if duration <= 0 {
				result.AddError(&core.ValidationError{
					Code:     "INVALID_SESSION_TTL",
					Message:  "Session TTL must be positive",
					Type:     core.ErrTypeValidation,
					Severity: core.SeverityHigh,
					Field:    "session_ttl",
				})
			}
		}
	}
}

func (s *SessionValidator) validateCreationConfig(config SessionManagerConfig, result *core.ValidationResult, options *core.ValidationOptions) {
	s.validateWorkspaceDirectory(config.WorkspaceDir, result)

	if config.MaxSessions <= 0 {
		result.AddError(&core.ValidationError{
			Code:     "INVALID_MAX_SESSIONS_CONFIG",
			Message:  "Maximum sessions must be positive",
			Type:     core.ErrTypeValidation,
			Severity: core.SeverityHigh,
			Field:    "max_sessions",
		})
	}

	if config.SessionTTL <= 0 {
		result.AddError(&core.ValidationError{
			Code:     "INVALID_SESSION_TTL_CONFIG",
			Message:  "Session TTL must be positive",
			Type:     core.ErrTypeValidation,
			Severity: core.SeverityHigh,
			Field:    "session_ttl",
		})
	}
}

func (s *SessionValidator) validateWorkspaceAccess(workspaceDir string, result *core.ValidationResult) {
	if workspaceDir == "" {
		return
	}

	// Check if directory exists
	if _, err := os.Stat(workspaceDir); os.IsNotExist(err) {
		result.AddError(&core.ValidationError{
			Code:     "WORKSPACE_NOT_EXISTS",
			Message:  fmt.Sprintf("Workspace directory does not exist: %s", workspaceDir),
			Type:     core.ErrTypeSystem,
			Severity: core.SeverityCritical,
			Field:    "workspace_dir",
		})
		return
	}

	// Check if it's a directory
	if info, err := os.Stat(workspaceDir); err == nil && !info.IsDir() {
		result.AddError(&core.ValidationError{
			Code:     "WORKSPACE_NOT_DIRECTORY",
			Message:  fmt.Sprintf("Workspace path is not a directory: %s", workspaceDir),
			Type:     core.ErrTypeSystem,
			Severity: core.SeverityCritical,
			Field:    "workspace_dir",
		})
	}

	// Check write permissions
	testFile := filepath.Join(workspaceDir, ".write_test")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		result.AddError(&core.ValidationError{
			Code:     "WORKSPACE_NOT_WRITABLE",
			Message:  fmt.Sprintf("Workspace directory is not writable: %s", workspaceDir),
			Type:     core.ErrTypeSystem,
			Severity: core.SeverityCritical,
			Field:    "workspace_dir",
		})
	} else {
		os.Remove(testFile) // Clean up test file
	}
}

func (s *SessionValidator) validateSessionLimits(currentSessions, maxSessions int, result *core.ValidationResult) {
	if currentSessions > maxSessions {
		result.AddError(&core.ValidationError{
			Code:     "SESSION_LIMIT_EXCEEDED",
			Message:  fmt.Sprintf("Current sessions (%d) exceed maximum allowed (%d)", currentSessions, maxSessions),
			Type:     core.ErrTypeSystem,
			Severity: core.SeverityCritical,
		})
	}

	// Warn if approaching limit
	if float64(currentSessions) > float64(maxSessions)*0.8 {
		result.AddWarning(&core.ValidationWarning{
			ValidationError: &core.ValidationError{
				Code:     "APPROACHING_SESSION_LIMIT",
				Message:  fmt.Sprintf("Using %d%% of session limit (%d/%d)", int(float64(currentSessions)/float64(maxSessions)*100), currentSessions, maxSessions),
				Type:     core.ErrTypeSystem,
				Severity: core.SeverityMedium,
			},
		})
	}
}

func (s *SessionValidator) validateTotalDiskUsage(manager *SessionManager, result *core.ValidationResult) {
	var totalUsage int64
	for _, usage := range manager.diskUsage {
		totalUsage += usage
	}

	if s.totalDiskLimit > 0 && totalUsage > s.totalDiskLimit {
		result.AddError(&core.ValidationError{
			Code:     "TOTAL_DISK_LIMIT_EXCEEDED",
			Message:  fmt.Sprintf("Total disk usage (%d bytes) exceeds limit (%d bytes)", totalUsage, s.totalDiskLimit),
			Type:     core.ErrTypeSystem,
			Severity: core.SeverityCritical,
		})
	}

	// Warn if approaching limit
	if s.totalDiskLimit > 0 && float64(totalUsage) > float64(s.totalDiskLimit)*0.8 {
		result.AddWarning(&core.ValidationWarning{
			ValidationError: &core.ValidationError{
				Code:     "APPROACHING_TOTAL_DISK_LIMIT",
				Message:  fmt.Sprintf("Using %d%% of total disk limit", int(float64(totalUsage)/float64(s.totalDiskLimit)*100)),
				Type:     core.ErrTypeSystem,
				Severity: core.SeverityMedium,
			},
		})
	}
}

func (s *SessionValidator) validateSessionTTL(ttl time.Duration, result *core.ValidationResult) {
	if ttl <= 0 {
		result.AddError(&core.ValidationError{
			Code:     "INVALID_SESSION_TTL",
			Message:  "Session TTL must be positive",
			Type:     core.ErrTypeValidation,
			Severity: core.SeverityHigh,
		})
		return
	}

	// Warn about very short TTLs
	if ttl < 5*time.Minute {
		result.AddWarning(&core.ValidationWarning{
			ValidationError: &core.ValidationError{
				Code:     "SHORT_SESSION_TTL",
				Message:  fmt.Sprintf("Session TTL (%v) is very short, may cause frequent cleanup", ttl),
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityLow,
			},
		})
	}

	// Warn about very long TTLs
	if ttl > 30*24*time.Hour { // 30 days
		result.AddWarning(&core.ValidationWarning{
			ValidationError: &core.ValidationError{
				Code:     "LONG_SESSION_TTL",
				Message:  fmt.Sprintf("Session TTL (%v) is very long, may cause resource buildup", ttl),
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityLow,
			},
		})
	}
}

func (s *SessionValidator) validateAllSessions(sessions map[string]*SessionState, result *core.ValidationResult) {
	options := core.NewValidationOptions().WithStrictMode(false)

	for sessionID, session := range sessions {
		sessionResult := s.ValidateSessionState(context.Background(), session, options)

		// Add session-specific errors to overall result
		for _, err := range sessionResult.Errors {
			sessionErr := &core.ValidationError{
				Code:     err.Code,
				Message:  fmt.Sprintf("Session %s: %s", sessionID, err.Message),
				Type:     err.Type,
				Severity: err.Severity,
				Field:    fmt.Sprintf("sessions.%s.%s", sessionID, err.Field),
				Context:  err.Context,
			}
			result.Errors = append(result.Errors, sessionErr)
		}

		// Add high-severity warnings
		for _, warning := range sessionResult.Warnings {
			if warning.Severity == core.SeverityHigh || warning.Severity == core.SeverityCritical {
				sessionWarning := &core.ValidationWarning{
					ValidationError: &core.ValidationError{
						Code:     warning.Code,
						Message:  fmt.Sprintf("Session %s: %s", sessionID, warning.Message),
						Type:     warning.Type,
						Severity: warning.Severity,
						Field:    fmt.Sprintf("sessions.%s.%s", sessionID, warning.Field),
						Context:  warning.Context,
					},
				}
				result.Warnings = append(result.Warnings, sessionWarning)
			}
		}
	}
}

func (s *SessionValidator) calculateValidationScore(result *core.ValidationResult) {
	score := 100.0

	// Deduct points for errors
	for _, err := range result.Errors {
		switch err.Severity {
		case core.SeverityCritical:
			score -= 25
		case core.SeverityHigh:
			score -= 15
		case core.SeverityMedium:
			score -= 10
		case core.SeverityLow:
			score -= 5
		}
	}

	// Deduct points for warnings
	for _, warning := range result.Warnings {
		switch warning.Severity {
		case core.SeverityHigh:
			score -= 5
		case core.SeverityMedium:
			score -= 3
		case core.SeverityLow:
			score -= 1
		}
	}

	if score < 0 {
		score = 0
	}

	result.Score = score

	// Set risk level based on score and critical issues
	hasCritical := false
	for _, err := range result.Errors {
		if err.Severity == core.SeverityCritical {
			hasCritical = true
			break
		}
	}

	if hasCritical || score < 30 {
		result.RiskLevel = "critical"
	} else if score < 60 {
		result.RiskLevel = "high"
	} else if score < 80 {
		result.RiskLevel = "medium"
	} else {
		result.RiskLevel = "low"
	}

	// Update validity
	result.Valid = len(result.Errors) == 0
}

// Implement core.Validator interface

func (s *SessionValidator) Validate(ctx context.Context, data interface{}, options *core.ValidationOptions) *core.ValidationResult {
	switch v := data.(type) {
	case *SessionState:
		return s.ValidateSessionState(ctx, v, options)
	case SessionState:
		return s.ValidateSessionState(ctx, &v, options)
	case *SessionManager:
		return s.ValidateSessionManager(ctx, v, options)
	case SessionManager:
		return s.ValidateSessionManager(ctx, &v, options)
	case map[string]interface{}:
		return s.ValidateSessionCreationArgs(ctx, v, options)
	case SessionManagerConfig:
		return s.ValidateSessionCreationArgs(ctx, v, options)
	default:
		return &core.ValidationResult{
			Valid: false,
			Errors: []*core.ValidationError{{
				Code:     "INVALID_SESSION_DATA_TYPE",
				Message:  fmt.Sprintf("Expected session data, got %T", data),
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityHigh,
			}},
			Warnings: make([]*core.ValidationWarning, 0),
			Metadata: core.ValidationMetadata{
				ValidatedAt:      time.Now(),
				ValidatorName:    s.GetName(),
				ValidatorVersion: s.GetVersion(),
				Context:          make(map[string]interface{}),
			},
			Suggestions: make([]string, 0),
		}
	}
}

// Public validation functions for external use

// ValidateSessionState validates a session state using unified validation
func ValidateSessionState(state *SessionState) *core.ValidationResult {
	validator := NewSessionValidator(100, 10*1024*1024*1024, 100*1024*1024*1024, 24*time.Hour)
	ctx := context.Background()
	options := core.NewValidationOptions()

	return validator.ValidateSessionState(ctx, state, options)
}

// ValidateSessionManager validates a session manager using unified validation
func ValidateSessionManager(manager *SessionManager) *core.ValidationResult {
	validator := NewSessionValidator(100, 10*1024*1024*1024, 100*1024*1024*1024, 24*time.Hour)
	ctx := context.Background()
	options := core.NewValidationOptions()

	return validator.ValidateSessionManager(ctx, manager, options)
}

// ValidateSessionCreation validates session creation arguments
func ValidateSessionCreation(args interface{}) *core.ValidationResult {
	validator := NewSessionValidator(100, 10*1024*1024*1024, 100*1024*1024*1024, 24*time.Hour)
	ctx := context.Background()
	options := core.NewValidationOptions()

	return validator.ValidateSessionCreationArgs(ctx, args, options)
}
