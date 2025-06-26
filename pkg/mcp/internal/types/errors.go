package types

import (
	"encoding/json"
	"fmt"
	"time"
)

// RichError provides comprehensive error information for Claude to reason about
type RichError struct {
	// Basic error information
	Code      string    `json:"code"`      // Error code (e.g., "BUILD_FAILED", "DEPLOY_ERROR")
	Message   string    `json:"message"`   // Human-readable error message
	Type      string    `json:"type"`      // Error type category
	Severity  string    `json:"severity"`  // "low", "medium", "high", "critical"
	Timestamp time.Time `json:"timestamp"` // When the error occurred

	// Context information
	Context     ErrorContext     `json:"context"`     // Rich context about the error
	Diagnostics ErrorDiagnostics `json:"diagnostics"` // Diagnostic information
	Resolution  ErrorResolution  `json:"resolution"`  // Suggested resolutions

	// Session and retry information
	SessionState   *SessionStateSnapshot `json:"session_state,omitempty"`
	Tool           string                `json:"tool"`
	AttemptNumber  int                   `json:"attempt_number"`
	PreviousErrors []string              `json:"previous_errors,omitempty"`
	Environment    map[string]string     `json:"environment,omitempty"`
}

// ErrorContext provides detailed context about where and why the error occurred
type ErrorContext struct {
	// Operation context
	Operation string `json:"operation"` // What operation was being performed
	Stage     string `json:"stage"`     // What stage of the operation
	Component string `json:"component"` // Which component failed

	// Input/output context
	Input         map[string]interface{} `json:"input,omitempty"`          // Input that caused the error
	PartialOutput map[string]interface{} `json:"partial_output,omitempty"` // Any partial results

	// System context
	SystemState   SystemState   `json:"system_state"`   // System state at error time
	ResourceUsage ResourceUsage `json:"resource_usage"` // Resource usage info

	// Additional context
	RelatedFiles []string       `json:"related_files,omitempty"` // Files involved in the error
	Logs         []LogEntry     `json:"logs,omitempty"`          // Relevant log entries
	Metadata     *ErrorMetadata `json:"metadata,omitempty"`      // Structured metadata
}

// ErrorDiagnostics provides diagnostic information for troubleshooting
type ErrorDiagnostics struct {
	// Error analysis
	RootCause    string `json:"root_cause"`    // Identified root cause
	ErrorPattern string `json:"error_pattern"` // Common error pattern identified
	Category     string `json:"category"`      // Error category

	// Diagnostic checks
	Checks   []DiagnosticCheck `json:"checks"`   // Diagnostic checks performed
	Symptoms []string          `json:"symptoms"` // Observed symptoms

	// Related information
	SimilarErrors []SimilarError `json:"similar_errors,omitempty"` // Similar past errors
	Documentation []string       `json:"documentation,omitempty"`  // Relevant docs
}

// ErrorResolution provides actionable resolution suggestions
type ErrorResolution struct {
	// Immediate actions
	ImmediateSteps []ResolutionStep `json:"immediate_steps"` // Steps to resolve now

	// Alternative approaches
	Alternatives []Alternative `json:"alternatives"` // Alternative approaches

	// Prevention
	Prevention []string `json:"prevention"` // How to prevent in future

	// Retry guidance
	RetryStrategy RetryStrategy `json:"retry_strategy"` // How/when to retry

	// Manual intervention
	ManualSteps []string `json:"manual_steps,omitempty"` // Manual steps if needed
}

// Supporting types

// SessionStateSnapshot captures session state at error time
type SessionStateSnapshot struct {
	ID              string                 `json:"id"`
	CurrentStage    string                 `json:"current_stage"`
	CompletedStages []string               `json:"completed_stages"`
	Metadata        map[string]interface{} `json:"metadata"`
}

// SystemState captures system state information
type SystemState struct {
	DockerAvailable bool   `json:"docker_available"`
	K8sConnected    bool   `json:"k8s_connected"`
	DiskSpaceMB     int64  `json:"disk_space_mb"`
	WorkspaceQuota  int64  `json:"workspace_quota_mb"`
	NetworkStatus   string `json:"network_status"`
}

// ResourceUsage captures resource usage at error time
type ResourceUsage struct {
	CPUPercent       float64 `json:"cpu_percent"`
	MemoryMB         int64   `json:"memory_mb"`
	DiskUsageMB      int64   `json:"disk_usage_mb"`
	NetworkBandwidth string  `json:"network_bandwidth"`
}

// LogEntry represents a relevant log entry
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Component string    `json:"component"`
	Message   string    `json:"message"`
}

// DiagnosticCheck represents a diagnostic check performed
type DiagnosticCheck struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// SimilarError represents a similar past error
type SimilarError struct {
	Code       string    `json:"code"`
	Occurred   time.Time `json:"occurred"`
	Resolution string    `json:"resolution"`
	Successful bool      `json:"successful"`
}

// ResolutionStep represents a step to resolve the error
type ResolutionStep struct {
	Order       int    `json:"order"`
	Action      string `json:"action"`
	Description string `json:"description"`
	Command     string `json:"command,omitempty"`
	ToolCall    string `json:"tool_call,omitempty"`
	Expected    string `json:"expected"`
}

// Alternative represents an alternative approach
type Alternative struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Steps       []string `json:"steps"`
	TradeOffs   []string `json:"trade_offs"`
	Confidence  float64  `json:"confidence"`
}

// RetryStrategy defines how to retry after the error
type RetryStrategy struct {
	Recommended     bool          `json:"recommended"`
	WaitTime        time.Duration `json:"wait_time"`
	MaxAttempts     int           `json:"max_attempts"`
	BackoffStrategy string        `json:"backoff_strategy"`
	Conditions      []string      `json:"conditions"`
}

// Error implements the error interface
func (e *RichError) Error() string {
	return fmt.Sprintf("[%s] %s: %s", e.Code, e.Type, e.Message)
}

// ToJSON converts the error to JSON
func (e *RichError) ToJSON() ([]byte, error) {
	return json.MarshalIndent(e, "", "  ")
}

// NewRichError creates a new rich error with basic information
func NewRichError(code, message, errorType string) *RichError {
	return &RichError{
		Code:      code,
		Message:   message,
		Type:      errorType,
		Severity:  "medium",
		Timestamp: time.Now(),
		Context: ErrorContext{
			SystemState:   SystemState{},
			ResourceUsage: ResourceUsage{},
		},
		Diagnostics: ErrorDiagnostics{
			Checks:   make([]DiagnosticCheck, 0),
			Symptoms: make([]string, 0),
		},
		Resolution: ErrorResolution{
			ImmediateSteps: make([]ResolutionStep, 0),
			Alternatives:   make([]Alternative, 0),
			Prevention:     make([]string, 0),
		},
	}
}

// Common error codes - now using MCP standard error codes
// These constants map our application-specific errors to MCP error codes
const (
	// Build errors - mapped to appropriate MCP error codes
	ErrCodeBuildFailed       = "internal_server_error" // Build failed -> internal server error
	ErrCodeDockerfileInvalid = "invalid_arguments"     // Dockerfile invalid -> invalid arguments
	ErrCodeBuildTimeout      = "internal_server_error" // Build timeout -> internal server error
	ErrCodeImagePushFailed   = "internal_server_error" // Image push failed -> internal server error

	// Deployment errors
	ErrCodeDeployFailed          = "internal_server_error" // Deploy failed -> internal server error
	ErrCodeManifestInvalid       = "invalid_arguments"     // Manifest invalid -> invalid arguments
	ErrCodeClusterUnreachable    = "internal_server_error" // Cluster unreachable -> internal server error
	ErrCodeResourceQuotaExceeded = "internal_server_error" // Resource quota exceeded -> internal server error

	// Analysis errors
	ErrCodeRepoUnreachable = "invalid_request"       // Repo unreachable -> invalid request
	ErrCodeAnalysisFailed  = "internal_server_error" // Analysis failed -> internal server error
	ErrCodeLanguageUnknown = "invalid_arguments"     // Language unknown -> invalid arguments
	ErrCodeCloneFailed     = "internal_server_error" // Clone failed -> internal server error

	// System errors
	ErrCodeDiskFull         = "internal_server_error" // Disk full -> internal server error
	ErrCodeNetworkError     = "internal_server_error" // Network error -> internal server error
	ErrCodePermissionDenied = "invalid_request"       // Permission denied -> invalid request
	ErrCodeTimeout          = "internal_server_error" // Timeout -> internal server error

	// Session errors
	ErrCodeSessionNotFound        = "invalid_request"       // Session not found -> invalid request
	ErrCodeSessionExpired         = "invalid_request"       // Session expired -> invalid request
	ErrCodeWorkspaceQuotaExceeded = "internal_server_error" // Workspace quota exceeded -> internal server error

	// Security errors
	ErrCodeSecurityVulnerabilities = "internal_server_error" // Security vulnerabilities -> internal server error
)

// Error type categories
const (
	ErrTypeBuild      = "build_error"
	ErrTypeDeployment = "deployment_error"
	ErrTypeAnalysis   = "analysis_error"
	ErrTypeSystem     = "system_error"
	ErrTypeSession    = "session_error"
	ErrTypeValidation = "validation_error"
	ErrTypeSecurity   = "security_error"
)

// Error severity levels are defined in constants.go

// Helper methods for ErrorContext to ease migration

// SetMetadata sets metadata from components (migration helper)
func (ec *ErrorContext) SetMetadata(sessionID, toolName, operation string) {
	ec.Metadata = NewErrorMetadata(sessionID, toolName, operation)
}

// SetMetadataContext sets the metadata context directly
func (ec *ErrorContext) SetMetadataContext(metadata *ErrorMetadata) {
	ec.Metadata = metadata
}

// AddCustomMetadata adds a custom metadata field (for backward compatibility)
func (ec *ErrorContext) AddCustomMetadata(key string, value interface{}) {
	if ec.Metadata == nil {
		ec.Metadata = NewErrorMetadata("", "", "")
	}
	ec.Metadata.AddCustom(key, value)
}

// Migration helpers for legacy map[string]interface{} usage

// Legacy metadata migration functions have been removed as part of
// Workstream 2: Adapter Deprecation cleanup.
//
// All error metadata now uses structured types directly - no migration needed.
