package errors

import (
	"errors"
)

// Sentinel errors for common cases.
var (
	// ErrToolNotFound is returned when a requested tool is not found.
	ErrToolNotFound = NewError().Code(CodeToolNotFound).Message("tool not found").Build()

	// ErrInvalidParams is returned when tool parameters are invalid.
	ErrInvalidParams = NewError().Code(CodeInvalidParameter).Message("invalid parameters").Build()

	// ErrExecutionFailed is returned when tool execution fails.
	ErrExecutionFailed = NewError().Code(CodeToolExecutionFailed).Message("execution failed").Build()

	// ErrTimeout is returned when an operation times out.
	ErrTimeout = NewError().Code(CodeNetworkTimeout).Message("operation timeout").Build()

	// ErrToolAlreadyExists is returned when trying to register a tool that already exists.
	ErrToolAlreadyExists = NewError().Code(CodeToolAlreadyRegistered).Message("tool already exists").Build()

	// ErrRegistrationFailed is returned when tool registration fails.
	ErrRegistrationFailed = NewError().Code(CodeToolExecutionFailed).Message("registration failed").Build()

	// ErrInvalidTool is returned when a tool doesn't implement the required interface.
	ErrInvalidTool = NewError().Code(CodeValidationFailed).Message("invalid tool interface").Build()
)

// Docker-specific errors.
var (
	// ErrInvalidImage is returned when an image reference is invalid.
	ErrInvalidImage = NewError().Code(CodeValidationFailed).Message("invalid image reference").Build()

	// ErrImageNotFound is returned when an image is not found.
	ErrImageNotFound = NewError().Code(CodeResourceNotFound).Message("image not found").Build()

	// ErrBuildFailed is returned when image build fails.
	ErrBuildFailed = NewError().Code(CodeImageBuildFailed).Message("image build failed").Build()

	// ErrPushFailed is returned when image push fails.
	ErrPushFailed = NewError().Code(CodeImagePushFailed).Message("image push failed").Build()

	// ErrPullFailed is returned when image pull fails.
	ErrPullFailed = NewError().Code(CodeImagePullFailed).Message("image pull failed").Build()
)

// Kubernetes-specific errors.
var (
	// ErrClusterNotFound is returned when a cluster is not found.
	ErrClusterNotFound = NewError().Code(CodeResourceNotFound).Message("cluster not found").Build()

	// ErrDeploymentFailed is returned when deployment fails.
	ErrDeploymentFailed = NewError().Code(CodeDeploymentFailed).Message("deployment failed").Build()

	// ErrResourceNotFound is returned when a Kubernetes resource is not found.
	ErrResourceNotFound = NewError().Code(CodeResourceNotFound).Message("resource not found").Build()

	// ErrManifestInvalid is returned when a Kubernetes manifest is invalid.
	ErrManifestInvalid = NewError().Code(CodeManifestInvalid).Message("manifest invalid").Build()

	// ErrNamespaceNotFound is returned when a namespace is not found.
	ErrNamespaceNotFound = NewError().Code(CodeNamespaceNotFound).Message("namespace not found").Build()
)

// Security-specific errors.
var (
	// ErrVulnerabilityFound is returned when security vulnerabilities are found.
	ErrVulnerabilityFound = NewError().Code(CodeValidationFailed).Message("vulnerability found").Build()

	// ErrSecretDetected is returned when secrets are detected in code.
	ErrSecretDetected = NewError().Code(CodeValidationFailed).Message("secret detected").Build()

	// ErrScanFailed is returned when security scanning fails.
	ErrScanFailed = NewError().Code(CodeToolExecutionFailed).Message("security scan failed").Build()

	// ErrPolicyViolation is returned when security policies are violated.
	ErrPolicyViolation = NewError().Code(CodeValidationFailed).Message("policy violation").Build()
)

// Analysis-specific errors.
var (
	// ErrRepositoryNotFound is returned when a repository is not found.
	ErrRepositoryNotFound = NewError().Code(CodeResourceNotFound).Message("repository not found").Build()

	// ErrAnalysisFailed is returned when repository analysis fails.
	ErrAnalysisFailed = NewError().Code(CodeToolExecutionFailed).Message("analysis failed").Build()

	// ErrUnsupportedLanguage is returned when a language is not supported.
	ErrUnsupportedLanguage = NewError().Code(CodeValidationFailed).Message("unsupported language").Build()

	// ErrInvalidRepository is returned when a repository is invalid.
	ErrInvalidRepository = NewError().Code(CodeValidationFailed).Message("invalid repository").Build()
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
		return e.Message + ": " + e.Cause.Error()
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
