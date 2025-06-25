package orchestration

import (
	"strings"

	"github.com/Azure/container-copilot/pkg/mcp/internal/workflow"
	"github.com/rs/zerolog"
)

// ErrorClassifier handles error classification and severity determination
type ErrorClassifier struct {
	logger zerolog.Logger
}

// NewErrorClassifier creates a new error classifier
func NewErrorClassifier(logger zerolog.Logger) *ErrorClassifier {
	return &ErrorClassifier{
		logger: logger.With().Str("component", "error_classifier").Logger(),
	}
}

// IsFatalError determines if an error should be considered fatal and cause immediate workflow failure
func (ec *ErrorClassifier) IsFatalError(workflowError *workflow.WorkflowError) bool {
	// Critical severity errors are always fatal
	if workflowError.Severity == "critical" {
		return true
	}

	// Define fatal error patterns
	fatalErrorTypes := []string{
		"authentication_failure",
		"authorization_denied",
		"invalid_credentials",
		"permission_denied",
		"configuration_invalid",
		"dependency_missing",
		"resource_exhausted",
		"quota_exceeded",
		"system_error",
		"security_violation",
		"data_corruption",
		"incompatible_version",
		"license_expired",
		"malformed_request",
		"invalid_input_format",
	}

	for _, fatalType := range fatalErrorTypes {
		if strings.Contains(strings.ToLower(workflowError.ErrorType), fatalType) {
			ec.logger.Debug().
				Str("error_type", workflowError.ErrorType).
				Str("fatal_pattern", fatalType).
				Msg("Error classified as fatal")
			return true
		}
	}

	// Check for fatal patterns in error message
	fatalMessagePatterns := []string{
		"cannot be retried",
		"permanent failure",
		"unrecoverable error",
		"fatal:",
		"critical:",
		"access denied",
		"forbidden",
		"unauthorized",
		"not found",
		"does not exist",
		"invalid format",
		"syntax error",
		"parse error",
		"validation failed",
	}

	lowerMessage := strings.ToLower(workflowError.Message)
	for _, pattern := range fatalMessagePatterns {
		if strings.Contains(lowerMessage, pattern) {
			ec.logger.Debug().
				Str("error_message", workflowError.Message).
				Str("fatal_pattern", pattern).
				Msg("Error message contains fatal pattern")
			return true
		}
	}

	return false
}

// CanRecover determines if an error can be recovered from
func (ec *ErrorClassifier) CanRecover(workflowError *workflow.WorkflowError, recoveryStrategies map[string]RecoveryStrategy) bool {
	// Fatal errors cannot be recovered
	if ec.IsFatalError(workflowError) {
		ec.logger.Debug().
			Str("error_id", workflowError.ID).
			Str("error_type", workflowError.ErrorType).
			Msg("Error is fatal, cannot recover")
		return false
	}

	if !workflowError.Retryable {
		return false
	}

	// Check if we have a recovery strategy for this error type
	for _, strategy := range recoveryStrategies {
		for _, errorType := range strategy.ApplicableErrors {
			if ec.matchesErrorType(workflowError.ErrorType, errorType) {
				return true
			}
		}
	}

	// Default recoverability based on error type
	recoverableTypes := []string{
		"network_error",
		"timeout_error",
		"resource_unavailable",
		"temporary_failure",
		"rate_limit_exceeded",
		"connection_error",
		"service_unavailable",
	}

	for _, recoverableType := range recoverableTypes {
		if strings.Contains(workflowError.ErrorType, recoverableType) {
			return true
		}
	}

	return false
}

// ClassifySeverity determines the severity of an error if not already set
func (ec *ErrorClassifier) ClassifySeverity(workflowError *workflow.WorkflowError) string {
	if workflowError.Severity != "" {
		return workflowError.Severity
	}

	// Classify based on error type
	if ec.IsFatalError(workflowError) {
		return "critical"
	}

	// Authentication/authorization errors are high severity
	lowerErrorType := strings.ToLower(workflowError.ErrorType)
	if strings.Contains(lowerErrorType, "auth") || strings.Contains(lowerErrorType, "permission") {
		return "high"
	}

	// Network and timeout errors are medium severity
	if strings.Contains(lowerErrorType, "network") || strings.Contains(lowerErrorType, "timeout") {
		return "medium"
	}

	// Default to low severity
	return "low"
}

// matchesErrorType checks if an error type matches a pattern
func (ec *ErrorClassifier) matchesErrorType(errorType, pattern string) bool {
	if pattern == "*" {
		return true
	}
	return strings.Contains(strings.ToLower(errorType), strings.ToLower(pattern))
}
