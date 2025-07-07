package errors

import (
	"errors"
	"fmt"
)

// Sentinel errors for common cases.
var (
	// ErrToolNotFound is returned when a requested tool is not found.
	ErrToolNotFound = fmt.Errorf("tool not found")

	// ErrInvalidParams is returned when tool parameters are invalid.
	ErrInvalidParams = fmt.Errorf("invalid parameters")

	// ErrExecutionFailed is returned when tool execution fails.
	ErrExecutionFailed = fmt.Errorf("execution failed")

	// ErrTimeout is returned when an operation times out.
	ErrTimeout = fmt.Errorf("operation timeout")

	// ErrToolAlreadyExists is returned when trying to register a tool that already exists.
	ErrToolAlreadyExists = fmt.Errorf("tool already exists")

	// ErrRegistrationFailed is returned when tool registration fails.
	ErrRegistrationFailed = fmt.Errorf("registration failed")

	// ErrInvalidTool is returned when a tool doesn't implement the required interface.
	ErrInvalidTool = fmt.Errorf("invalid tool interface")
)

// Docker-specific errors.
var (
	// ErrInvalidImage is returned when an image reference is invalid.
	ErrInvalidImage = fmt.Errorf("invalid image reference")

	// ErrImageNotFound is returned when an image is not found.
	ErrImageNotFound = fmt.Errorf("image not found")

	// ErrBuildFailed is returned when image build fails.
	ErrBuildFailed = fmt.Errorf("image build failed")

	// ErrPushFailed is returned when image push fails.
	ErrPushFailed = fmt.Errorf("image push failed")

	// ErrPullFailed is returned when image pull fails.
	ErrPullFailed = fmt.Errorf("image pull failed")
)

// Kubernetes-specific errors.
var (
	// ErrClusterNotFound is returned when a cluster is not found.
	ErrClusterNotFound = fmt.Errorf("cluster not found")

	// ErrDeploymentFailed is returned when deployment fails.
	ErrDeploymentFailed = fmt.Errorf("deployment failed")

	// ErrResourceNotFound is returned when a Kubernetes resource is not found.
	ErrResourceNotFound = fmt.Errorf("resource not found")

	// ErrManifestInvalid is returned when a Kubernetes manifest is invalid.
	ErrManifestInvalid = fmt.Errorf("manifest invalid")

	// ErrNamespaceNotFound is returned when a namespace is not found.
	ErrNamespaceNotFound = fmt.Errorf("namespace not found")
)

// Security-specific errors.
var (
	// ErrVulnerabilityFound is returned when security vulnerabilities are found.
	ErrVulnerabilityFound = fmt.Errorf("vulnerability found")

	// ErrSecretDetected is returned when secrets are detected in code.
	ErrSecretDetected = fmt.Errorf("secret detected")

	// ErrScanFailed is returned when security scanning fails.
	ErrScanFailed = fmt.Errorf("security scan failed")

	// ErrPolicyViolation is returned when security policies are violated.
	ErrPolicyViolation = fmt.Errorf("policy violation")
)

// Analysis-specific errors.
var (
	// ErrRepositoryNotFound is returned when a repository is not found.
	ErrRepositoryNotFound = fmt.Errorf("repository not found")

	// ErrAnalysisFailed is returned when repository analysis fails.
	ErrAnalysisFailed = fmt.Errorf("analysis failed")

	// ErrUnsupportedLanguage is returned when a language is not supported.
	ErrUnsupportedLanguage = fmt.Errorf("unsupported language")

	// ErrInvalidRepository is returned when a repository is invalid.
	ErrInvalidRepository = fmt.Errorf("invalid repository")
)

// Is reports whether any error in err's chain matches target.
func Is(err, target error) bool {
	return errors.Is(err, target)
}

// As finds the first error in err's chain that matches target.
func As(err error, target interface{}) bool {
	return errors.As(err, target)
}

// Unwrap returns the result of calling the Unwrap method on err.
func Unwrap(err error) error {
	return errors.Unwrap(err)
}

// Join returns an error that wraps the given errors.
func Join(errs ...error) error {
	return errors.Join(errs...)
}

// ClassifiedError provides additional context about an error.
type ClassifiedError struct {
	// Type is the error type.
	Type ErrorType

	// Message is the error message.
	Message string

	// Cause is the underlying error.
	Cause error

	// Context provides additional context about the error.
	Context map[string]interface{}
}

// Error implements the error interface.
func (e *ClassifiedError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

// Unwrap returns the underlying error.
func (e *ClassifiedError) Unwrap() error {
	return e.Cause
}

// NewClassifiedError creates a new classified error.
func NewClassifiedError(errorType ErrorType, message string, cause error) *ClassifiedError {
	return &ClassifiedError{
		Type:    errorType,
		Message: message,
		Cause:   cause,
		Context: make(map[string]interface{}),
	}
}

// WithContext adds context to the error.
func (e *ClassifiedError) WithContext(key string, value interface{}) *ClassifiedError {
	e.Context[key] = value
	return e
}
