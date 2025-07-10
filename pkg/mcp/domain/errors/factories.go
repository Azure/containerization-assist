package errors

import (
	"fmt"
	"strings"
	"time"
)

// Domain-specific error factory functions

// BuildError creates a build domain error with appropriate defaults
func BuildError(code ErrorCode, message string, cause error) *RichError {
	return &RichError{
		Code:      code,
		Message:   message,
		Type:      ErrTypeContainer,
		Severity:  SeverityMedium,
		Context:   make(ErrorContext),
		Timestamp: time.Now(),
		Cause:     cause,
		Suggestions: []string{
			"Check build configuration and dependencies",
			"Verify Dockerfile syntax and base image availability",
			"Review build logs for detailed error information",
		},
	}
}

// BuildErrorWithContext creates a build error with additional context
func BuildErrorWithContext(code ErrorCode, message string, cause error, context ErrorContext) *RichError {
	err := BuildError(code, message, cause)
	for k, v := range context {
		err.Context[k] = v
	}
	err.Context["domain"] = "build"
	return err
}

// DeployError creates a deployment domain error with appropriate defaults
func DeployError(code ErrorCode, message string, cause error) *RichError {
	return &RichError{
		Code:      code,
		Message:   message,
		Type:      ErrTypeKubernetes,
		Severity:  SeverityMedium,
		Context:   make(ErrorContext),
		Timestamp: time.Now(),
		Cause:     cause,
		Suggestions: []string{
			"Verify Kubernetes configuration and connectivity",
			"Check cluster resources and quotas",
			"Validate deployment manifests",
		},
	}
}

// DeployErrorWithContext creates a deployment error with additional context
func DeployErrorWithContext(code ErrorCode, message string, cause error, context ErrorContext) *RichError {
	err := DeployError(code, message, cause)
	for k, v := range context {
		err.Context[k] = v
	}
	err.Context["domain"] = "deployment"
	return err
}

// SecurityError creates a security domain error with appropriate defaults
func SecurityError(code ErrorCode, message string, cause error) *RichError {
	severity := SeverityMedium
	suggestions := []string{
		"Review security policies and scan configuration",
		"Check for policy violations and compliance requirements",
		"Verify scanner availability and database updates",
	}

	// Adjust severity and suggestions based on error code
	switch code {
	case "SECURITY_VULNERABILITY_CRITICAL":
		severity = SeverityCritical
		suggestions = []string{
			"CRITICAL: Address vulnerability immediately",
			"Block deployment until vulnerability is resolved",
			"Review security baseline and policies",
		}
	case "SECURITY_VULNERABILITY_HIGH":
		severity = SeverityHigh
		suggestions = []string{
			"HIGH: Address vulnerability in next maintenance window",
			"Consider temporary mitigations",
			"Review exposure and attack vectors",
		}
	case "SECURITY_POLICY_VIOLATION", "SECURITY_COMPLIANCE_VIOLATION":
		severity = SeverityHigh
		suggestions = []string{
			"Review and update security policies",
			"Ensure compliance with organizational standards",
			"Check for policy exemptions or exceptions",
		}
	}

	return &RichError{
		Code:        code,
		Message:     message,
		Type:        ErrTypeSecurity,
		Severity:    severity,
		Context:     make(ErrorContext),
		Timestamp:   time.Now(),
		Cause:       cause,
		Suggestions: suggestions,
	}
}

// SecurityErrorWithContext creates a security error with additional context
func SecurityErrorWithContext(code ErrorCode, message string, cause error, context ErrorContext) *RichError {
	err := SecurityError(code, message, cause)
	for k, v := range context {
		err.Context[k] = v
	}
	err.Context["domain"] = "security"
	return err
}

// ValidationError creates a validation error with appropriate defaults
func ValidationError(code ErrorCode, message string, cause error) *RichError {
	return &RichError{
		Code:      code,
		Message:   message,
		Type:      ErrTypeValidation,
		Severity:  SeverityMedium,
		Context:   make(ErrorContext),
		Timestamp: time.Now(),
		Cause:     cause,
		Suggestions: []string{
			"Check input parameters and format",
			"Verify required fields are provided",
			"Review validation schema and constraints",
		},
	}
}

// NetworkError creates a network error with appropriate defaults
func NetworkError(code ErrorCode, message string, cause error) *RichError {
	return &RichError{
		Code:      code,
		Message:   message,
		Type:      ErrTypeNetwork,
		Severity:  SeverityMedium,
		Context:   make(ErrorContext),
		Timestamp: time.Now(),
		Cause:     cause,
		Suggestions: []string{
			"Check network connectivity and firewall rules",
			"Verify DNS resolution and routing",
			"Review proxy and authentication settings",
		},
	}
}

// SystemError creates a system error with appropriate defaults
func SystemError(code ErrorCode, message string, cause error) *RichError {
	return &RichError{
		Code:      code,
		Message:   message,
		Type:      ErrTypeSystem,
		Severity:  SeverityHigh,
		Context:   make(ErrorContext),
		Timestamp: time.Now(),
		Cause:     cause,
		Suggestions: []string{
			"Check system resources and availability",
			"Review system configuration and dependencies",
			"Contact system administrator if issue persists",
		},
	}
}

// Convenience functions for common error patterns

// BuildConfigError creates a build configuration error
func BuildConfigError(message string, cause error) *RichError {
	return BuildError(CodeInvalidParameter, message, cause).
		WithContext("error_type", "configuration")
}

// BuildExecutionError creates a build execution error
func BuildExecutionError(message string, cause error) *RichError {
	return BuildError(CodeToolExecutionFailed, message, cause).
		WithContext("error_type", "execution")
}

// DeployManifestError creates a deployment manifest error
func DeployManifestError(message string, cause error) *RichError {
	return DeployError(CodeManifestInvalid, message, cause).
		WithContext("error_type", "manifest")
}

// DeployClusterError creates a deployment cluster error
func DeployClusterError(message string, cause error) *RichError {
	return DeployError(CodeKubernetesAPIError, message, cause).
		WithContext("error_type", "cluster")
}

// SecurityVulnerabilityError creates a security vulnerability error
func SecurityVulnerabilityError(severity string, message string, cause error) *RichError {
	code := ErrorCode(fmt.Sprintf("SECURITY_VULNERABILITY_%s", severity))
	return SecurityError(code, message, cause).
		WithContext("vulnerability_severity", severity)
}

// SecurityPolicyError creates a security policy error
func SecurityPolicyError(message string, cause error) *RichError {
	return SecurityError("SECURITY_POLICY_VIOLATION", message, cause).
		WithContext("error_type", "policy")
}

// Helper methods for error factories

// WithContext adds context to a RichError
func (e *RichError) WithContext(key string, value interface{}) *RichError {
	if e.Context == nil {
		e.Context = make(ErrorContext)
	}
	e.Context[key] = value
	return e
}

// WithSuggestion adds a suggestion to a RichError
func (e *RichError) WithSuggestion(suggestion string) *RichError {
	e.Suggestions = append(e.Suggestions, suggestion)
	return e
}

// WithSeverity sets the severity of a RichError
func (e *RichError) WithSeverity(severity ErrorSeverity) *RichError {
	e.Severity = severity
	return e
}

// WithType sets the type of a RichError
func (e *RichError) WithType(errType ErrorType) *RichError {
	e.Type = errType
	return e
}

// Tool-specific error constructors

// BuildFailedError creates a build failure error
func BuildFailedError(stage, reason string) *RichError {
	return BuildError(CodeImageBuildFailed, fmt.Sprintf("Build failed at %s: %s", stage, reason), nil).
		WithContext("stage", stage).
		WithContext("reason", reason)
}

// DockerfileGenerationError creates a Dockerfile generation error
func DockerfileGenerationError(reason string) *RichError {
	return BuildError(CodeDockerfileSyntaxError, fmt.Sprintf("Failed to generate Dockerfile: %s", reason), nil).
		WithContext("error_code", "DOCKERFILE_GEN_FAILED")
}

// Note: ImagePushError and ImagePullError already exist in constructors.go

// DeploymentError creates a deployment error
func DeploymentError(resource, reason string) *RichError {
	return DeployError(CodeDeploymentFailed, fmt.Sprintf("Failed to deploy %s: %s", resource, reason), nil).
		WithContext("resource", resource)
}

// ManifestGenerationError creates a manifest generation error
func ManifestGenerationError(kind, reason string) *RichError {
	return DeployError(CodeManifestInvalid, fmt.Sprintf("Failed to generate %s manifest: %s", kind, reason), nil).
		WithContext("kind", kind).
		WithContext("error_code", "MANIFEST_GEN_FAILED")
}

// K8sConnectionError creates a Kubernetes connection error
func K8sConnectionError(cluster, reason string) *RichError {
	return DeployError(CodeKubernetesAPIError, fmt.Sprintf("Failed to connect to Kubernetes cluster %s: %s", cluster, reason), nil).
		WithContext("cluster", cluster).
		WithContext("retryable", true).
		WithSeverity(SeverityHigh)
}

// ScanError creates a security scan error
func ScanError(scanner, target, reason string) *RichError {
	return SecurityError("SECURITY_SCAN_FAILED", fmt.Sprintf("Security scan failed for %s using %s: %s", target, scanner, reason), nil).
		WithContext("scanner", scanner).
		WithContext("target", target)
}

// VulnerabilityError creates a vulnerability found error
func VulnerabilityError(severity string, count int, details string) *RichError {
	code := ErrorCode(fmt.Sprintf("SECURITY_VULNERABILITY_%s", strings.ToUpper(severity)))
	err := SecurityError(code, fmt.Sprintf("Found %d %s vulnerabilities: %s", count, severity, details), nil).
		WithContext("severity", severity).
		WithContext("count", count)

	if severity == "critical" {
		err = err.WithSeverity(SeverityCritical)
	}

	return err
}

// AnalysisError creates a repository analysis error
func AnalysisError(path, reason string) *RichError {
	return NewError().
		Code("ANALYSIS_FAILED").
		Message(fmt.Sprintf("Failed to analyze repository at %s: %s", path, reason)).
		Type(ErrTypeBusiness).
		Context("path", path).
		Build()
}

// NoSupportedFilesError creates a no supported files error
func NoSupportedFilesError(path string, languages []string) *RichError {
	return NewError().
		Code("NO_SUPPORTED_FILES").
		Message(fmt.Sprintf("No supported files found in %s. Looked for: %v", path, languages)).
		Type(ErrTypeBusiness).
		Context("path", path).
		Context("languages", languages).
		Build()
}

// LanguageNotSupportedError creates a language not supported error
func LanguageNotSupportedError(language string) *RichError {
	return NewError().
		Code("LANGUAGE_NOT_SUPPORTED").
		Message(fmt.Sprintf("Language %s is not supported", language)).
		Type(ErrTypeBusiness).
		Context("language", language).
		Build()
}

// SessionNotFoundError creates a session not found error
func SessionNotFoundError(sessionID string) *RichError {
	return NewError().
		Code(CodeResourceNotFound).
		Message(fmt.Sprintf("Session %s not found", sessionID)).
		Type(ErrTypeSession).
		Context("session_id", sessionID).
		Build()
}

// SessionExpiredError creates a session expired error
func SessionExpiredError(sessionID string) *RichError {
	return NewError().
		Code("SESSION_EXPIRED").
		Message(fmt.Sprintf("Session %s has expired", sessionID)).
		Type(ErrTypeSession).
		Context("session_id", sessionID).
		Build()
}

// SessionStoreError creates a session storage error
func SessionStoreError(operation string, err error) *RichError {
	return NewError().
		Code("SESSION_STORE_FAILED").
		Message(fmt.Sprintf("Session store operation failed: %s", operation)).
		Type(ErrTypeSession).
		Context("operation", operation).
		Cause(err).
		Build()
}

// ToolExecutionError creates a tool execution error to replace the legacy ToolError type
func ToolExecutionError(toolName, errorType, message string) *RichError {
	code := CodeToolExecutionFailed
	severity := SeverityMedium

	// Map error types to appropriate codes and severities
	switch errorType {
	case "validation_error":
		code = CodeInvalidParameter
		severity = SeverityLow
	case "execution_error":
		code = CodeToolExecutionFailed
		severity = SeverityMedium
	case "timeout_error":
		code = "TIMEOUT"
		severity = SeverityMedium
	case "resource_error":
		code = CodeResourceNotFound
		severity = SeverityMedium
	case "permission_error":
		code = CodePermissionDenied
		severity = SeverityHigh
	}

	return NewError().
		Code(code).
		Message(message).
		Type(ErrTypeBusiness).
		Severity(severity).
		Context("tool_name", toolName).
		Context("error_type", errorType).
		Context("retryable", errorType != "validation_error").
		Build()
}

// ToolExecutionErrorWithContext creates a tool execution error with additional context
func ToolExecutionErrorWithContext(toolName, errorType, message string, context ErrorContext) *RichError {
	err := ToolExecutionError(toolName, errorType, message)
	for k, v := range context {
		err.Context[k] = v
	}
	return err
}
