package errors

import (
	"fmt"
	"time"

	mcperrors "github.com/Azure/container-kit/pkg/mcp/errors"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
)

// MigrateMCPError converts an MCPError to CoreError
func MigrateMCPError(mcpErr *mcperrors.MCPError) *CoreError {
	if mcpErr == nil {
		return nil
	}

	coreErr := &CoreError{
		Category:    mapMCPCategory(mcpErr.Category),
		Module:      mcpErr.Module,
		Operation:   mcpErr.Operation,
		Message:     mcpErr.Message,
		Severity:    SeverityMedium, // Default severity
		Timestamp:   time.Now(),
		Cause:       mcpErr.Cause,
		Context:     mcpErr.Context,
		Retryable:   mcpErr.Retryable,
		Recoverable: mcpErr.Recoverable,
	}

	// Convert cause to string for serialization
	if mcpErr.Cause != nil {
		coreErr.CauseStr = mcpErr.Cause.Error()
	}

	return coreErr
}

// MigrateRichError converts a RichError to CoreError
func MigrateRichError(richErr *types.RichError) *CoreError {
	if richErr == nil {
		return nil
	}

	coreErr := &CoreError{
		Category:    mapRichErrorCategory(richErr.Type),
		Module:      richErr.Tool,
		Message:     richErr.Message,
		Code:        richErr.Code,
		Severity:    mapRichErrorSeverity(richErr.Severity),
		Timestamp:   richErr.Timestamp,
		Context:     richErr.Context.Input,
		Tool:        richErr.Tool,
		Retryable:   richErr.Resolution.RetryStrategy.Recommended,
		Recoverable: true, // Default to recoverable
	}

	// Map diagnostics
	if richErr.Diagnostics.RootCause != "" {
		coreErr.Diagnostics = &ErrorDiagnostics{
			RootCause:     richErr.Diagnostics.RootCause,
			ErrorPattern:  richErr.Diagnostics.ErrorPattern,
			Symptoms:      richErr.Diagnostics.Symptoms,
			Checks:        mapDiagnosticChecks(richErr.Diagnostics.Checks),
			SimilarErrors: mapSimilarErrors(richErr.Diagnostics.SimilarErrors),
		}
	}

	// Map resolution
	if len(richErr.Resolution.ImmediateSteps) > 0 {
		coreErr.Resolution = &ErrorResolution{
			ImmediateSteps: mapResolutionSteps(richErr.Resolution.ImmediateSteps),
			Alternatives:   mapAlternatives(richErr.Resolution.Alternatives),
			Prevention:     richErr.Resolution.Prevention,
			RetryStrategy:  mapRetryStrategy(richErr.Resolution.RetryStrategy),
			ManualSteps:    richErr.Resolution.ManualSteps,
		}
	}

	// Map system state
	if richErr.Context.SystemState.DockerAvailable || richErr.Context.SystemState.K8sConnected {
		coreErr.SystemState = &SystemState{
			DockerAvailable: richErr.Context.SystemState.DockerAvailable,
			K8sConnected:    richErr.Context.SystemState.K8sConnected,
			DiskSpaceMB:     richErr.Context.SystemState.DiskSpaceMB,
			MemoryMB:        richErr.Context.SystemState.WorkspaceQuota,
			LoadAverage:     0, // Not available in RichError
		}
	}

	// Map resource usage
	if richErr.Context.ResourceUsage.CPUPercent > 0 {
		coreErr.ResourceUsage = &ResourceUsage{
			CPUPercent:  richErr.Context.ResourceUsage.CPUPercent,
			MemoryMB:    richErr.Context.ResourceUsage.MemoryMB,
			DiskUsageMB: richErr.Context.ResourceUsage.DiskUsageMB,
		}
	}

	// Map log entries
	if len(richErr.Context.Logs) > 0 {
		coreErr.LogEntries = mapLogEntries(richErr.Context.Logs)
	}

	// Map related files
	if len(richErr.Context.RelatedFiles) > 0 {
		coreErr.RelatedFiles = richErr.Context.RelatedFiles
	}

	// Map session information
	if richErr.SessionState != nil {
		coreErr.SessionID = richErr.SessionState.ID
		coreErr.Stage = richErr.SessionState.CurrentStage
	}

	// Map attempt information
	coreErr.AttemptNumber = richErr.AttemptNumber
	coreErr.PreviousErrors = richErr.PreviousErrors

	return coreErr
}

// Helper mapping functions

func mapMCPCategory(category mcperrors.ErrorCategory) ErrorCategory {
	switch category {
	case mcperrors.CategoryValidation:
		return CategoryValidation
	case mcperrors.CategoryNetwork:
		return CategoryNetwork
	case mcperrors.CategoryInternal:
		return CategoryInternal
	case mcperrors.CategoryAuth:
		return CategoryAuth
	case mcperrors.CategoryResource:
		return CategoryResource
	case mcperrors.CategoryTimeout:
		return CategoryTimeout
	case mcperrors.CategoryConfig:
		return CategoryConfig
	default:
		return CategoryInternal
	}
}

func mapRichErrorCategory(errorType string) ErrorCategory {
	switch errorType {
	case types.ErrTypeBuild:
		return CategoryBuild
	case types.ErrTypeDeployment:
		return CategoryDeploy
	case types.ErrTypeAnalysis:
		return CategoryAnalysis
	case types.ErrTypeSession:
		return CategorySession
	case types.ErrTypeValidation:
		return CategoryValidation
	case types.ErrTypeSecurity:
		return CategorySecurity
	case types.ErrTypeSystem:
		return CategoryInternal
	default:
		return CategoryInternal
	}
}

func mapRichErrorSeverity(severity string) Severity {
	switch severity {
	case "low":
		return SeverityLow
	case "medium":
		return SeverityMedium
	case "high":
		return SeverityHigh
	case "critical":
		return SeverityCritical
	default:
		return SeverityMedium
	}
}

func mapDiagnosticChecks(checks []types.DiagnosticCheck) []DiagnosticCheck {
	result := make([]DiagnosticCheck, len(checks))
	for i, check := range checks {
		status := "fail"
		if check.Passed {
			status = "pass"
		}
		result[i] = DiagnosticCheck{
			Name:    check.Name,
			Status:  status,
			Details: check.Details,
		}
	}
	return result
}

func mapSimilarErrors(errors []types.SimilarError) []SimilarError {
	result := make([]SimilarError, len(errors))
	for i, err := range errors {
		result[i] = SimilarError{
			ErrorCode:  err.Code,
			Frequency:  1, // Not available in RichError
			LastSeen:   err.Occurred,
			Resolution: err.Resolution,
		}
	}
	return result
}

func mapResolutionSteps(steps []types.ResolutionStep) []ResolutionStep {
	result := make([]ResolutionStep, len(steps))
	for i, step := range steps {
		result[i] = ResolutionStep{
			Step:        step.Order,
			Action:      step.Action,
			Description: step.Description,
			Command:     step.Command,
			Expected:    step.Expected,
		}
	}
	return result
}

func mapAlternatives(alternatives []types.Alternative) []Alternative {
	result := make([]Alternative, len(alternatives))
	for i, alt := range alternatives {
		effort := "medium"
		risk := "medium"
		if alt.Confidence > 0.8 {
			effort = "low"
			risk = "low"
		} else if alt.Confidence < 0.4 {
			effort = "high"
			risk = "high"
		}

		result[i] = Alternative{
			Approach:    alt.Name,
			Description: alt.Description,
			Effort:      effort,
			Risk:        risk,
		}
	}
	return result
}

func mapRetryStrategy(strategy types.RetryStrategy) *RetryStrategy {
	if !strategy.Recommended {
		return nil
	}

	return &RetryStrategy{
		Retryable:     strategy.Recommended,
		MaxAttempts:   strategy.MaxAttempts,
		BackoffMs:     int(strategy.WaitTime.Milliseconds()),
		ExponentialMs: 0, // Not available in RichError
		Conditions:    strategy.Conditions,
	}
}

func mapLogEntries(logs []types.LogEntry) []LogEntry {
	result := make([]LogEntry, len(logs))
	for i, log := range logs {
		result[i] = LogEntry{
			Timestamp: log.Timestamp,
			Level:     log.Level,
			Message:   log.Message,
			Source:    log.Component,
		}
	}
	return result
}

// Wrapper functions for backwards compatibility

// WrapError wraps any error as a CoreError
func WrapError(err error, module, operation string) *CoreError {
	if err == nil {
		return nil
	}

	// Check if it's already a CoreError
	if coreErr, ok := err.(*CoreError); ok {
		return coreErr
	}

	// Check if it's an MCPError
	if mcpErr, ok := err.(*mcperrors.MCPError); ok {
		return MigrateMCPError(mcpErr)
	}

	// Check if it's a RichError
	if richErr, ok := err.(*types.RichError); ok {
		return MigrateRichError(richErr)
	}

	// Create new CoreError for generic errors
	return Wrap(err, module, operation)
}

// IsRetryable checks if any error type is retryable
func IsRetryable(err error) bool {
	if coreErr, ok := err.(*CoreError); ok {
		return coreErr.Retryable
	}
	if mcpErr, ok := err.(*mcperrors.MCPError); ok {
		return mcpErr.Retryable
	}
	if richErr, ok := err.(*types.RichError); ok {
		return richErr.Resolution.RetryStrategy.Recommended
	}
	return false
}

// IsRecoverable checks if any error type is recoverable
func IsRecoverable(err error) bool {
	if coreErr, ok := err.(*CoreError); ok {
		return coreErr.Recoverable
	}
	if mcpErr, ok := err.(*mcperrors.MCPError); ok {
		return mcpErr.Recoverable
	}
	// RichError doesn't have recoverable flag, default to true
	return true
}

// GetErrorCategory returns the category of any error type
func GetErrorCategory(err error) ErrorCategory {
	if coreErr, ok := err.(*CoreError); ok {
		return coreErr.Category
	}
	if mcpErr, ok := err.(*mcperrors.MCPError); ok {
		return mapMCPCategory(mcpErr.Category)
	}
	if richErr, ok := err.(*types.RichError); ok {
		return mapRichErrorCategory(richErr.Type)
	}
	return CategoryInternal
}

// GetErrorSeverity returns the severity of any error type
func GetErrorSeverity(err error) Severity {
	if coreErr, ok := err.(*CoreError); ok {
		return coreErr.Severity
	}
	if richErr, ok := err.(*types.RichError); ok {
		return mapRichErrorSeverity(richErr.Severity)
	}
	return SeverityMedium
}

// FormatError formats any error type consistently
func FormatError(err error) string {
	if coreErr, ok := err.(*CoreError); ok {
		return coreErr.Error()
	}
	if mcpErr, ok := err.(*mcperrors.MCPError); ok {
		return mcpErr.Error()
	}
	if richErr, ok := err.(*types.RichError); ok {
		return richErr.Error()
	}
	return fmt.Sprintf("mcp: %v", err)
}
