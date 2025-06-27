package errors

import (
	"encoding/json"
	"fmt"
	"time"
)

// CoreError is the unified error type for the MCP system
// It consolidates functionality from MCPError, RichError, and other error types
type CoreError struct {
	// Basic error information (from MCPError)
	Category  ErrorCategory `json:"category"`       // Error category for classification
	Module    string        `json:"module"`         // Module where error occurred
	Operation string        `json:"operation"`      // Operation being performed
	Message   string        `json:"message"`        // Human-readable error message
	Code      string        `json:"code,omitempty"` // Specific error code (e.g., "BUILD_FAILED")
	Severity  Severity      `json:"severity"`       // Error severity level
	Timestamp time.Time     `json:"timestamp"`      // When the error occurred

	// Error relationships
	Cause    error    `json:"-"`                 // Underlying error (not serialized)
	CauseStr string   `json:"cause,omitempty"`   // String representation of cause
	Wrapped  []string `json:"wrapped,omitempty"` // Chain of wrapped errors

	// Error characteristics
	Retryable   bool `json:"retryable"`   // Whether operation can be retried
	Recoverable bool `json:"recoverable"` // Whether error is recoverable
	Fatal       bool `json:"fatal"`       // Whether error is fatal to session

	// Context and metadata (enhanced from both systems)
	Context map[string]interface{} `json:"context,omitempty"` // Basic context information
	Input   map[string]interface{} `json:"input,omitempty"`   // Input that caused the error
	Output  map[string]interface{} `json:"output,omitempty"`  // Any partial output

	// Session information
	SessionID string `json:"session_id,omitempty"` // Associated session
	Tool      string `json:"tool,omitempty"`       // Tool that generated the error
	Stage     string `json:"stage,omitempty"`      // Stage of operation
	Component string `json:"component,omitempty"`  // Specific component

	// Diagnostics and resolution (from RichError)
	Diagnostics *ErrorDiagnostics `json:"diagnostics,omitempty"` // Diagnostic information
	Resolution  *ErrorResolution  `json:"resolution,omitempty"`  // Resolution suggestions

	// Retry and recovery information
	AttemptNumber  int      `json:"attempt_number,omitempty"`  // Current attempt number
	PreviousErrors []string `json:"previous_errors,omitempty"` // Previous error messages

	// System context
	SystemState   *SystemState   `json:"system_state,omitempty"`   // System state snapshot
	ResourceUsage *ResourceUsage `json:"resource_usage,omitempty"` // Resource usage info

	// Files and logs
	RelatedFiles []string   `json:"related_files,omitempty"` // Files involved in error
	LogEntries   []LogEntry `json:"log_entries,omitempty"`   // Relevant log entries
}

// ErrorCategory represents different types of errors in the MCP system
type ErrorCategory string

const (
	// Core system categories
	CategoryValidation ErrorCategory = "validation" // Invalid input or configuration
	CategoryNetwork    ErrorCategory = "network"    // Connection, timeout, DNS issues
	CategoryInternal   ErrorCategory = "internal"   // Unexpected system failures
	CategoryAuth       ErrorCategory = "auth"       // Permission denied, authentication failures
	CategoryResource   ErrorCategory = "resource"   // Not found, already exists, quota exceeded
	CategoryTimeout    ErrorCategory = "timeout"    // Operation timeout
	CategoryConfig     ErrorCategory = "config"     // Invalid or missing configuration

	// Domain-specific categories
	CategoryBuild    ErrorCategory = "build"    // Build and compilation errors
	CategoryDeploy   ErrorCategory = "deploy"   // Deployment errors
	CategorySecurity ErrorCategory = "security" // Security scan and policy errors
	CategoryAnalysis ErrorCategory = "analysis" // Repository analysis errors
	CategorySession  ErrorCategory = "session"  // Session management errors
	CategoryWorkflow ErrorCategory = "workflow" // Workflow orchestration errors
)

// Severity represents the severity level of an error
type Severity string

const (
	SeverityLow      Severity = "low"      // Minor issues, warnings
	SeverityMedium   Severity = "medium"   // Moderate issues, some impact
	SeverityHigh     Severity = "high"     // Significant issues, major impact
	SeverityCritical Severity = "critical" // Critical failures, system-level impact
)

// Supporting types for diagnostics and resolution

// ErrorDiagnostics provides diagnostic information for troubleshooting
type ErrorDiagnostics struct {
	RootCause     string            `json:"root_cause,omitempty"`     // Identified root cause
	ErrorPattern  string            `json:"error_pattern,omitempty"`  // Common error pattern
	Symptoms      []string          `json:"symptoms,omitempty"`       // Observed symptoms
	Checks        []DiagnosticCheck `json:"checks,omitempty"`         // Diagnostic checks performed
	SimilarErrors []SimilarError    `json:"similar_errors,omitempty"` // Similar past errors
}

// ErrorResolution provides actionable resolution suggestions
type ErrorResolution struct {
	ImmediateSteps []ResolutionStep `json:"immediate_steps,omitempty"` // Steps to resolve now
	Alternatives   []Alternative    `json:"alternatives,omitempty"`    // Alternative approaches
	Prevention     []string         `json:"prevention,omitempty"`      // How to prevent in future
	RetryStrategy  *RetryStrategy   `json:"retry_strategy,omitempty"`  // How/when to retry
	ManualSteps    []string         `json:"manual_steps,omitempty"`    // Manual intervention needed
}

// DiagnosticCheck represents a diagnostic check that was performed
type DiagnosticCheck struct {
	Name    string `json:"name"`              // Name of the check
	Status  string `json:"status"`            // pass, fail, warning, unknown
	Details string `json:"details,omitempty"` // Additional details
}

// SimilarError represents a similar error from the past
type SimilarError struct {
	ErrorCode  string    `json:"error_code"`           // Error code of similar error
	Frequency  int       `json:"frequency"`            // How often this error occurs
	LastSeen   time.Time `json:"last_seen"`            // When this error was last seen
	Resolution string    `json:"resolution,omitempty"` // How it was resolved
}

// ResolutionStep represents a specific step to resolve the error
type ResolutionStep struct {
	Step        int    `json:"step"`               // Step number
	Action      string `json:"action"`             // Action to take
	Description string `json:"description"`        // Detailed description
	Command     string `json:"command,omitempty"`  // Command to run, if any
	Expected    string `json:"expected,omitempty"` // Expected outcome
}

// Alternative represents an alternative approach to resolve the error
type Alternative struct {
	Approach    string `json:"approach"`         // Alternative approach name
	Description string `json:"description"`      // Description of approach
	Effort      string `json:"effort,omitempty"` // Required effort (low, medium, high)
	Risk        string `json:"risk,omitempty"`   // Risk level (low, medium, high)
}

// RetryStrategy defines how and when to retry after this error
type RetryStrategy struct {
	Retryable     bool     `json:"retryable"`                // Whether retry is recommended
	MaxAttempts   int      `json:"max_attempts,omitempty"`   // Maximum retry attempts
	BackoffMs     int      `json:"backoff_ms,omitempty"`     // Backoff time in milliseconds
	ExponentialMs int      `json:"exponential_ms,omitempty"` // Exponential backoff factor
	Conditions    []string `json:"conditions,omitempty"`     // Conditions that must be met
}

// SystemState captures system state information
type SystemState struct {
	DockerAvailable bool    `json:"docker_available"`
	K8sConnected    bool    `json:"k8s_connected"`
	DiskSpaceMB     int64   `json:"disk_space_mb"`
	MemoryMB        int64   `json:"memory_mb"`
	LoadAverage     float64 `json:"load_average"`
}

// ResourceUsage captures resource usage at error time
type ResourceUsage struct {
	CPUPercent     float64 `json:"cpu_percent"`
	MemoryMB       int64   `json:"memory_mb"`
	DiskUsageMB    int64   `json:"disk_usage_mb"`
	NetworkBytesTx int64   `json:"network_bytes_tx"`
	NetworkBytesRx int64   `json:"network_bytes_rx"`
}

// LogEntry represents a relevant log entry
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Source    string    `json:"source,omitempty"`
}

// Error implements the error interface
func (e *CoreError) Error() string {
	if e.Module != "" {
		return fmt.Sprintf("mcp/%s: %s", e.Module, e.Message)
	}
	return fmt.Sprintf("mcp: %s", e.Message)
}

// Unwrap returns the underlying error for error unwrapping
func (e *CoreError) Unwrap() error {
	return e.Cause
}

// Is checks if the error matches a target error
func (e *CoreError) Is(target error) bool {
	if coreErr, ok := target.(*CoreError); ok {
		return e.Category == coreErr.Category && e.Module == coreErr.Module && e.Code == coreErr.Code
	}
	if e.Cause != nil {
		return fmt.Sprintf("%v", e.Cause) == fmt.Sprintf("%v", target)
	}
	return false
}

// WithContext adds context information to the error
func (e *CoreError) WithContext(key string, value interface{}) *CoreError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// WithInput adds input information that caused the error
func (e *CoreError) WithInput(input map[string]interface{}) *CoreError {
	e.Input = input
	return e
}

// WithOutput adds partial output information
func (e *CoreError) WithOutput(output map[string]interface{}) *CoreError {
	e.Output = output
	return e
}

// WithSession adds session information
func (e *CoreError) WithSession(sessionID, tool, stage, component string) *CoreError {
	e.SessionID = sessionID
	e.Tool = tool
	e.Stage = stage
	e.Component = component
	return e
}

// WithDiagnostics adds diagnostic information
func (e *CoreError) WithDiagnostics(diagnostics *ErrorDiagnostics) *CoreError {
	e.Diagnostics = diagnostics
	return e
}

// WithResolution adds resolution information
func (e *CoreError) WithResolution(resolution *ErrorResolution) *CoreError {
	e.Resolution = resolution
	return e
}

// WithSystemState adds system state information
func (e *CoreError) WithSystemState(state *SystemState) *CoreError {
	e.SystemState = state
	return e
}

// WithResourceUsage adds resource usage information
func (e *CoreError) WithResourceUsage(usage *ResourceUsage) *CoreError {
	e.ResourceUsage = usage
	return e
}

// WithFiles adds related file information
func (e *CoreError) WithFiles(files []string) *CoreError {
	e.RelatedFiles = files
	return e
}

// WithLogs adds related log entries
func (e *CoreError) WithLogs(logs []LogEntry) *CoreError {
	e.LogEntries = logs
	return e
}

// SetRetryable marks the error as retryable or not
func (e *CoreError) SetRetryable(retryable bool) *CoreError {
	e.Retryable = retryable
	return e
}

// SetRecoverable marks the error as recoverable or not
func (e *CoreError) SetRecoverable(recoverable bool) *CoreError {
	e.Recoverable = recoverable
	return e
}

// SetFatal marks the error as fatal or not
func (e *CoreError) SetFatal(fatal bool) *CoreError {
	e.Fatal = fatal
	return e
}

// ToJSON serializes the error to JSON for logging or API responses
func (e *CoreError) ToJSON() ([]byte, error) {
	// Convert cause to string for serialization
	if e.Cause != nil && e.CauseStr == "" {
		e.CauseStr = e.Cause.Error()
	}
	return json.Marshal(e)
}

// Constructor functions

// New creates a new CoreError with basic information
func New(module, message string, category ErrorCategory) *CoreError {
	return &CoreError{
		Module:      module,
		Message:     message,
		Category:    category,
		Severity:    SeverityMedium, // Default severity
		Timestamp:   time.Now(),
		Context:     make(map[string]interface{}),
		Retryable:   false, // Default to not retryable
		Recoverable: true,  // Default to recoverable
		Fatal:       false, // Default to not fatal
	}
}

// Newf creates a new CoreError with formatted message
func Newf(module string, category ErrorCategory, format string, args ...interface{}) *CoreError {
	return &CoreError{
		Module:      module,
		Message:     fmt.Sprintf(format, args...),
		Category:    category,
		Severity:    SeverityMedium,
		Timestamp:   time.Now(),
		Context:     make(map[string]interface{}),
		Retryable:   false,
		Recoverable: true,
		Fatal:       false,
	}
}

// Wrap wraps an existing error with additional context
func Wrap(err error, module, message string) *CoreError {
	if err == nil {
		return nil
	}

	coreErr := &CoreError{
		Module:      module,
		Message:     message,
		Category:    CategoryInternal, // Default category for wrapped errors
		Severity:    SeverityMedium,
		Timestamp:   time.Now(),
		Cause:       err,
		CauseStr:    err.Error(),
		Context:     make(map[string]interface{}),
		Retryable:   false,
		Recoverable: true,
		Fatal:       false,
	}

	// If wrapping another CoreError, preserve some information
	if existingCore, ok := err.(*CoreError); ok {
		coreErr.Category = existingCore.Category
		coreErr.Severity = existingCore.Severity
		coreErr.Retryable = existingCore.Retryable
		coreErr.Recoverable = existingCore.Recoverable
		coreErr.Fatal = existingCore.Fatal

		// Build wrapped error chain
		coreErr.Wrapped = append([]string{existingCore.Message}, existingCore.Wrapped...)
	}

	return coreErr
}

// Validation creates a validation error
func Validation(module, message string) *CoreError {
	return New(module, message, CategoryValidation).SetRetryable(false)
}

// Network creates a network error
func Network(module, message string) *CoreError {
	return New(module, message, CategoryNetwork).SetRetryable(true)
}

// Internal creates an internal error
func Internal(module, message string) *CoreError {
	return New(module, message, CategoryInternal).SetRetryable(false).SetRecoverable(false)
}

// Auth creates an authentication/authorization error
func Auth(module, message string) *CoreError {
	return New(module, message, CategoryAuth).SetRetryable(false)
}

// Resource creates a resource error
func Resource(module, message string) *CoreError {
	return New(module, message, CategoryResource).SetRetryable(true)
}

// Timeout creates a timeout error
func Timeout(module, message string) *CoreError {
	return New(module, message, CategoryTimeout).SetRetryable(true)
}

// Config creates a configuration error
func Config(module, message string) *CoreError {
	return New(module, message, CategoryConfig).SetRetryable(false)
}

// Build creates a build error
func Build(module, message string) *CoreError {
	return New(module, message, CategoryBuild).SetRetryable(true)
}

// Deploy creates a deployment error
func Deploy(module, message string) *CoreError {
	return New(module, message, CategoryDeploy).SetRetryable(true)
}

// Security creates a security error
func Security(module, message string) *CoreError {
	return New(module, message, CategorySecurity).SetRetryable(false).SetFatal(true)
}

// Analysis creates an analysis error
func Analysis(module, message string) *CoreError {
	return New(module, message, CategoryAnalysis).SetRetryable(true)
}

// Session creates a session error
func Session(module, message string) *CoreError {
	return New(module, message, CategorySession).SetRetryable(false).SetRecoverable(false)
}

// Workflow creates a workflow error
func Workflow(module, message string) *CoreError {
	return New(module, message, CategoryWorkflow).SetRetryable(true)
}
