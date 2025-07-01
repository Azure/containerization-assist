package errors

import (
	"encoding/json"
	"fmt"
	"time"
)

// Type definitions consolidated from internal/types/errors*.go

// RichError provides comprehensive error information for Claude to reason about
type RichError struct {
	// Basic error information
	Code      string    `json:"code"`      // Error code (e.g., "BUILD_FAILED", "DEPLOY_ERROR")
	Message   string    `json:"message"`   // Human-readable error message
	Type      string    `json:"type"`      // Error type category
	Severity  string    `json:"severity"`  // "low", "medium", "high", "critical"
	Timestamp time.Time `json:"timestamp"` // When the error occurred

	// Context information
	Context     RichErrorContext     `json:"context"`     // Rich context about the error
	Diagnostics RichErrorDiagnostics `json:"diagnostics"` // Diagnostic information
	Resolution  RichErrorResolution  `json:"resolution"`  // Suggested resolutions

	// Session and retry information
	SessionState   *SessionStateSnapshot `json:"session_state,omitempty"`
	Tool           string                `json:"tool"`
	AttemptNumber  int                   `json:"attempt_number"`
	PreviousErrors []string              `json:"previous_errors,omitempty"`
	Environment    map[string]string     `json:"environment,omitempty"`
}

// RichErrorContext provides detailed context about where and why the error occurred
type RichErrorContext struct {
	// Operation context
	Operation string `json:"operation"` // What operation was being performed
	Stage     string `json:"stage"`     // What stage of the operation
	Component string `json:"component"` // Which component failed

	// Input/output context
	Input         map[string]interface{} `json:"input,omitempty"`          // Input that caused the error
	PartialOutput map[string]interface{} `json:"partial_output,omitempty"` // Any partial results

	// System context
	SystemState   RichSystemState   `json:"system_state"`   // System state at error time
	ResourceUsage RichResourceUsage `json:"resource_usage"` // Resource usage info

	// Additional context
	RelatedFiles []string           `json:"related_files,omitempty"` // Files involved in the error
	Logs         []RichLogEntry     `json:"logs,omitempty"`          // Relevant log entries
	Metadata     *RichErrorMetadata `json:"metadata,omitempty"`      // Structured metadata
}

// RichErrorDiagnostics provides diagnostic information for troubleshooting
type RichErrorDiagnostics struct {
	RootCause     string                `json:"root_cause"`               // Identified root cause
	ErrorPattern  string                `json:"error_pattern"`            // Common error pattern
	Symptoms      []string              `json:"symptoms"`                 // Observed symptoms
	Checks        []RichDiagnosticCheck `json:"checks"`                   // Diagnostic checks performed
	SimilarErrors []RichSimilarError    `json:"similar_errors,omitempty"` // Similar past errors
}

// RichErrorResolution provides actionable resolution suggestions
type RichErrorResolution struct {
	ImmediateSteps []RichResolutionStep `json:"immediate_steps"` // Steps to resolve now
	Alternatives   []RichAlternative    `json:"alternatives"`    // Alternative approaches
	Prevention     []string             `json:"prevention"`      // How to prevent in future
	RetryStrategy  *RichRetryStrategy   `json:"retry_strategy,omitempty"`
	ManualSteps    []string             `json:"manual_steps,omitempty"` // Manual intervention needed
}

// RichSystemState captures system state information
type RichSystemState struct {
	DockerAvailable bool              `json:"docker_available"`
	DockerVersion   string            `json:"docker_version,omitempty"`
	K8sConnected    bool              `json:"k8s_connected"`
	K8sVersion      string            `json:"k8s_version,omitempty"`
	DiskSpaceMB     int64             `json:"disk_space_mb"`
	MemoryMB        int64             `json:"memory_mb"`
	LoadAverage     float64           `json:"load_average"`
	Environment     map[string]string `json:"environment,omitempty"`
}

// RichResourceUsage captures resource usage at error time
type RichResourceUsage struct {
	CPUPercent     float64 `json:"cpu_percent"`
	MemoryMB       int64   `json:"memory_mb"`
	DiskUsageMB    int64   `json:"disk_usage_mb"`
	NetworkBytesTx int64   `json:"network_bytes_tx"`
	NetworkBytesRx int64   `json:"network_bytes_rx"`
}

// RichLogEntry represents a relevant log entry
type RichLogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Source    string    `json:"source,omitempty"`
}

// RichDiagnosticCheck represents a diagnostic check that was performed
type RichDiagnosticCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // pass, fail, warning, unknown
	Details string `json:"details,omitempty"`
}

// RichSimilarError represents a similar error from the past
type RichSimilarError struct {
	ErrorCode  string    `json:"error_code"`
	Frequency  int       `json:"frequency"`
	LastSeen   time.Time `json:"last_seen"`
	Resolution string    `json:"resolution,omitempty"`
}

// RichResolutionStep represents a specific step to resolve the error
type RichResolutionStep struct {
	Step        int    `json:"step"`
	Action      string `json:"action"`
	Description string `json:"description"`
	Command     string `json:"command,omitempty"`
	Expected    string `json:"expected,omitempty"`
}

// RichAlternative represents an alternative approach to resolve the error
type RichAlternative struct {
	Approach    string `json:"approach"`
	Description string `json:"description"`
	Effort      string `json:"effort,omitempty"` // low, medium, high
	Risk        string `json:"risk,omitempty"`   // low, medium, high
}

// RichRetryStrategy defines how and when to retry after this error
type RichRetryStrategy struct {
	Retryable     bool     `json:"retryable"`
	MaxAttempts   int      `json:"max_attempts,omitempty"`
	BackoffMs     int      `json:"backoff_ms,omitempty"`
	ExponentialMs int      `json:"exponential_ms,omitempty"`
	Conditions    []string `json:"conditions,omitempty"` // Conditions that must be met
}

// RichErrorMetadata provides structured metadata for errors
type RichErrorMetadata struct {
	ErrorID       string            `json:"error_id"`
	CorrelationID string            `json:"correlation_id,omitempty"`
	UserID        string            `json:"user_id,omitempty"`
	Tags          []string          `json:"tags,omitempty"`
	Labels        map[string]string `json:"labels,omitempty"`
}

// SessionStateSnapshot represents a snapshot of session state
type SessionStateSnapshot struct {
	ID          string                 `json:"id"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	Status      string                 `json:"status"`
	CurrentTool string                 `json:"current_tool,omitempty"`
	Progress    int                    `json:"progress"`
	Metrics     map[string]interface{} `json:"metrics,omitempty"`
}

// Error implements the error interface for RichError
func (e *RichError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// ToJSON serializes the error to JSON
func (e *RichError) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// ToCoreError converts a RichError to a CoreError
func (e *RichError) ToCoreError() *CoreError {
	coreErr := &CoreError{
		Code:           e.Code,
		Message:        e.Message,
		Timestamp:      e.Timestamp,
		Tool:           e.Tool,
		Stage:          e.Context.Stage,
		Component:      e.Context.Component,
		Operation:      e.Context.Operation,
		AttemptNumber:  e.AttemptNumber,
		PreviousErrors: e.PreviousErrors,
	}

	// Convert severity
	switch e.Severity {
	case "critical":
		coreErr.Severity = SeverityCritical
	case "high":
		coreErr.Severity = SeverityHigh
	case "medium":
		coreErr.Severity = SeverityMedium
	case "low":
		coreErr.Severity = SeverityLow
	default:
		coreErr.Severity = SeverityMedium
	}

	// Convert category based on type
	switch e.Type {
	case "validation":
		coreErr.Category = CategoryValidation
	case "network":
		coreErr.Category = CategoryNetwork
	case "build":
		coreErr.Category = CategoryBuild
	case "deploy":
		coreErr.Category = CategoryDeploy
	case "security":
		coreErr.Category = CategorySecurity
	case "analysis":
		coreErr.Category = CategoryAnalysis
	case "session":
		coreErr.Category = CategorySession
	case "workflow":
		coreErr.Category = CategoryWorkflow
	default:
		coreErr.Category = CategoryInternal
	}

	// Convert context
	coreErr.Input = e.Context.Input
	coreErr.Output = e.Context.PartialOutput

	// Convert diagnostics
	if e.Diagnostics.RootCause != "" || len(e.Diagnostics.Symptoms) > 0 {
		diagnostics := &ErrorDiagnostics{
			RootCause:    e.Diagnostics.RootCause,
			ErrorPattern: e.Diagnostics.ErrorPattern,
			Symptoms:     e.Diagnostics.Symptoms,
		}

		// Convert diagnostic checks
		for _, check := range e.Diagnostics.Checks {
			diagnostics.Checks = append(diagnostics.Checks, DiagnosticCheck(check))
		}

		// Convert similar errors
		for _, similar := range e.Diagnostics.SimilarErrors {
			diagnostics.SimilarErrors = append(diagnostics.SimilarErrors, SimilarError(similar))
		}

		coreErr.WithDiagnostics(diagnostics)
	}

	// Convert resolution
	if len(e.Resolution.ImmediateSteps) > 0 || len(e.Resolution.Alternatives) > 0 {
		resolution := &ErrorResolution{
			Prevention:  e.Resolution.Prevention,
			ManualSteps: e.Resolution.ManualSteps,
		}

		// Convert immediate steps
		for _, step := range e.Resolution.ImmediateSteps {
			resolution.ImmediateSteps = append(resolution.ImmediateSteps, ResolutionStep(step))
		}

		// Convert alternatives
		for _, alt := range e.Resolution.Alternatives {
			resolution.Alternatives = append(resolution.Alternatives, Alternative(alt))
		}

		// Convert retry strategy
		if e.Resolution.RetryStrategy != nil {
			resolution.RetryStrategy = &RetryStrategy{
				Retryable:     e.Resolution.RetryStrategy.Retryable,
				MaxAttempts:   e.Resolution.RetryStrategy.MaxAttempts,
				BackoffMs:     e.Resolution.RetryStrategy.BackoffMs,
				ExponentialMs: e.Resolution.RetryStrategy.ExponentialMs,
				Conditions:    e.Resolution.RetryStrategy.Conditions,
			}
		}

		coreErr.WithResolution(resolution)
	}

	// Convert system state
	if e.Context.SystemState.DockerAvailable || e.Context.SystemState.K8sConnected {
		coreErr.WithSystemState(&SystemState{
			DockerAvailable: e.Context.SystemState.DockerAvailable,
			K8sConnected:    e.Context.SystemState.K8sConnected,
			DiskSpaceMB:     e.Context.SystemState.DiskSpaceMB,
			MemoryMB:        e.Context.SystemState.MemoryMB,
			LoadAverage:     e.Context.SystemState.LoadAverage,
		})
	}

	// Convert resource usage
	if e.Context.ResourceUsage.CPUPercent > 0 || e.Context.ResourceUsage.MemoryMB > 0 {
		coreErr.WithResourceUsage(&ResourceUsage{
			CPUPercent:     e.Context.ResourceUsage.CPUPercent,
			MemoryMB:       e.Context.ResourceUsage.MemoryMB,
			DiskUsageMB:    e.Context.ResourceUsage.DiskUsageMB,
			NetworkBytesTx: e.Context.ResourceUsage.NetworkBytesTx,
			NetworkBytesRx: e.Context.ResourceUsage.NetworkBytesRx,
		})
	}

	// Convert files and logs
	coreErr.WithFiles(e.Context.RelatedFiles)

	if len(e.Context.Logs) > 0 {
		logs := make([]LogEntry, 0, len(e.Context.Logs))
		for _, log := range e.Context.Logs {
			logs = append(logs, LogEntry(log))
		}
		coreErr.WithLogs(logs)
	}

	// Set retry strategy from resolution
	if e.Resolution.RetryStrategy != nil {
		coreErr.SetRetryable(e.Resolution.RetryStrategy.Retryable)
	}

	return coreErr
}

// NewRichError creates a new RichError from a CoreError
func NewRichError(coreErr *CoreError) *RichError {
	richErr := &RichError{
		Code:           coreErr.Code,
		Message:        coreErr.Message,
		Timestamp:      coreErr.Timestamp,
		Tool:           coreErr.Tool,
		AttemptNumber:  coreErr.AttemptNumber,
		PreviousErrors: coreErr.PreviousErrors,
	}

	// Convert severity
	switch coreErr.Severity {
	case SeverityCritical:
		richErr.Severity = "critical"
	case SeverityHigh:
		richErr.Severity = "high"
	case SeverityMedium:
		richErr.Severity = "medium"
	case SeverityLow:
		richErr.Severity = "low"
	}

	// Convert category to type
	switch coreErr.Category {
	case CategoryValidation:
		richErr.Type = "validation"
	case CategoryNetwork:
		richErr.Type = "network"
	case CategoryBuild:
		richErr.Type = "build"
	case CategoryDeploy:
		richErr.Type = "deploy"
	case CategorySecurity:
		richErr.Type = "security"
	case CategoryAnalysis:
		richErr.Type = "analysis"
	case CategorySession:
		richErr.Type = "session"
	case CategoryWorkflow:
		richErr.Type = "workflow"
	default:
		richErr.Type = "internal"
	}

	// Build context
	richErr.Context = RichErrorContext{
		Operation:     coreErr.Operation,
		Stage:         coreErr.Stage,
		Component:     coreErr.Component,
		Input:         coreErr.Input,
		PartialOutput: coreErr.Output,
		RelatedFiles:  coreErr.RelatedFiles,
	}

	// Convert system state
	if coreErr.SystemState != nil {
		richErr.Context.SystemState = RichSystemState{
			DockerAvailable: coreErr.SystemState.DockerAvailable,
			K8sConnected:    coreErr.SystemState.K8sConnected,
			DiskSpaceMB:     coreErr.SystemState.DiskSpaceMB,
			MemoryMB:        coreErr.SystemState.MemoryMB,
			LoadAverage:     coreErr.SystemState.LoadAverage,
		}
	}

	// Convert resource usage
	if coreErr.ResourceUsage != nil {
		richErr.Context.ResourceUsage = RichResourceUsage{
			CPUPercent:     coreErr.ResourceUsage.CPUPercent,
			MemoryMB:       coreErr.ResourceUsage.MemoryMB,
			DiskUsageMB:    coreErr.ResourceUsage.DiskUsageMB,
			NetworkBytesTx: coreErr.ResourceUsage.NetworkBytesTx,
			NetworkBytesRx: coreErr.ResourceUsage.NetworkBytesRx,
		}
	}

	// Convert logs
	for _, log := range coreErr.LogEntries {
		richErr.Context.Logs = append(richErr.Context.Logs, RichLogEntry(log))
	}

	// Convert diagnostics
	if coreErr.Diagnostics != nil {
		richErr.Diagnostics = RichErrorDiagnostics{
			RootCause:    coreErr.Diagnostics.RootCause,
			ErrorPattern: coreErr.Diagnostics.ErrorPattern,
			Symptoms:     coreErr.Diagnostics.Symptoms,
		}

		// Convert diagnostic checks
		for _, check := range coreErr.Diagnostics.Checks {
			richErr.Diagnostics.Checks = append(richErr.Diagnostics.Checks, RichDiagnosticCheck(check))
		}

		// Convert similar errors
		for _, similar := range coreErr.Diagnostics.SimilarErrors {
			richErr.Diagnostics.SimilarErrors = append(richErr.Diagnostics.SimilarErrors, RichSimilarError(similar))
		}
	}

	// Convert resolution
	if coreErr.Resolution != nil {
		richErr.Resolution = RichErrorResolution{
			Prevention:  coreErr.Resolution.Prevention,
			ManualSteps: coreErr.Resolution.ManualSteps,
		}

		// Convert immediate steps
		for _, step := range coreErr.Resolution.ImmediateSteps {
			richErr.Resolution.ImmediateSteps = append(richErr.Resolution.ImmediateSteps, RichResolutionStep(step))
		}

		// Convert alternatives
		for _, alt := range coreErr.Resolution.Alternatives {
			richErr.Resolution.Alternatives = append(richErr.Resolution.Alternatives, RichAlternative(alt))
		}

		// Convert retry strategy
		if coreErr.Resolution.RetryStrategy != nil {
			richErr.Resolution.RetryStrategy = &RichRetryStrategy{
				Retryable:     coreErr.Resolution.RetryStrategy.Retryable,
				MaxAttempts:   coreErr.Resolution.RetryStrategy.MaxAttempts,
				BackoffMs:     coreErr.Resolution.RetryStrategy.BackoffMs,
				ExponentialMs: coreErr.Resolution.RetryStrategy.ExponentialMs,
				Conditions:    coreErr.Resolution.RetryStrategy.Conditions,
			}
		}
	}

	return richErr
}
