package errors

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"time"
)

// Common error codes
const (
	// General errors
	CodeUnknown              ErrorCode = "UNKNOWN"
	CodeInternalError        ErrorCode = "INTERNAL_ERROR"
	CodeValidationFailed     ErrorCode = "VALIDATION_FAILED"
	CodeInvalidParameter     ErrorCode = "INVALID_PARAMETER"
	CodeMissingParameter     ErrorCode = "MISSING_PARAMETER"
	CodeTypeConversionFailed ErrorCode = "TYPE_CONVERSION_FAILED"

	// Network/IO errors
	CodeNetworkTimeout        ErrorCode = "NETWORK_TIMEOUT"
	CodeIOError               ErrorCode = "IO_ERROR"
	CodeFileNotFound          ErrorCode = "FILE_NOT_FOUND"
	CodePermissionDenied      ErrorCode = "PERMISSION_DENIED"
	CodeResourceNotFound      ErrorCode = "RESOURCE_NOT_FOUND"
	CodeResourceAlreadyExists ErrorCode = "RESOURCE_ALREADY_EXISTS"
	CodeResourceExhausted     ErrorCode = "RESOURCE_EXHAUSTED"

	// Container/Docker errors
	CodeDockerfileSyntaxError ErrorCode = "DOCKERFILE_SYNTAX_ERROR"
	CodeImageBuildFailed      ErrorCode = "IMAGE_BUILD_FAILED"
	CodeImagePushFailed       ErrorCode = "IMAGE_PUSH_FAILED"
	CodeImagePullFailed       ErrorCode = "IMAGE_PULL_FAILED"
	CodeContainerStartFailed  ErrorCode = "CONTAINER_START_FAILED"

	// Kubernetes errors
	CodeKubernetesAPIError ErrorCode = "KUBERNETES_API_ERROR"
	CodeManifestInvalid    ErrorCode = "MANIFEST_INVALID"
	CodeDeploymentFailed   ErrorCode = "DEPLOYMENT_FAILED"
	CodeNamespaceNotFound  ErrorCode = "NAMESPACE_NOT_FOUND"

	// Tool/Registry errors
	CodeToolNotFound          ErrorCode = "TOOL_NOT_FOUND"
	CodeToolExecutionFailed   ErrorCode = "TOOL_EXECUTION_FAILED"
	CodeToolAlreadyRegistered ErrorCode = "TOOL_ALREADY_REGISTERED"
	CodeVersionMismatch       ErrorCode = "VERSION_MISMATCH"

	// Additional codes for tests
	CodeConfigurationInvalid ErrorCode = "CONFIGURATION_INVALID"
	CodeNetworkError         ErrorCode = "NETWORK_ERROR"
	CodeOperationFailed      ErrorCode = "OPERATION_FAILED"
	CodeTimeoutError         ErrorCode = "TIMEOUT_ERROR"
	CodeTypeMismatch         ErrorCode = "TYPE_MISMATCH"

	// Security error codes
	CodeSecurity           ErrorCode = "SECURITY_ERROR"
	CodeValidation         ErrorCode = "VALIDATION_ERROR"
	CodeSecurityViolation  ErrorCode = "SECURITY_VIOLATION"
	CodeVulnerabilityFound ErrorCode = "VULNERABILITY_FOUND"

	// Additional error codes
	CodeNotImplemented ErrorCode = "NOT_IMPLEMENTED"
	CodeAlreadyExists  ErrorCode = "ALREADY_EXISTS"
	CodeInvalidState   ErrorCode = "INVALID_STATE"
	CodeNotFound       ErrorCode = "NOT_FOUND"
	CodeDisabled       ErrorCode = "DISABLED"
	CodeInternal       ErrorCode = "INTERNAL"
	CodeInvalidType    ErrorCode = "INVALID_TYPE"
)

// ErrorType categorizes the error
type ErrorType string

const (
	ErrTypeInternal      ErrorType = "internal"
	ErrTypeValidation    ErrorType = "validation"
	ErrTypeNetwork       ErrorType = "network"
	ErrTypeIO            ErrorType = "io"
	ErrTypeTimeout       ErrorType = "timeout"
	ErrTypeNotFound      ErrorType = "not_found"
	ErrTypeConflict      ErrorType = "conflict"
	ErrTypeContainer     ErrorType = "container"
	ErrTypeKubernetes    ErrorType = "kubernetes"
	ErrTypeTool          ErrorType = "tool"
	ErrTypeSecurity      ErrorType = "security"
	ErrTypeSession       ErrorType = "session"
	ErrTypeResource      ErrorType = "resource"
	ErrTypeBusiness      ErrorType = "business"
	ErrTypeSystem        ErrorType = "system"
	ErrTypePermission    ErrorType = "permission"
	ErrTypeConfiguration ErrorType = "configuration"
	ErrTypeOperation     ErrorType = "operation"
	ErrTypeExternal      ErrorType = "external"
)

// ErrorSeverity indicates the error severity
type ErrorSeverity string

const (
	SeverityUnknown  ErrorSeverity = "unknown"
	SeverityLow      ErrorSeverity = "low"
	SeverityMedium   ErrorSeverity = "medium"
	SeverityHigh     ErrorSeverity = "high"
	SeverityCritical ErrorSeverity = "critical"
)

// ErrorContext provides additional error context
type ErrorContext map[string]interface{}

// RichError provides comprehensive error information
type RichError struct {
	// Core fields
	Code     ErrorCode     `json:"code"`
	Message  string        `json:"message"`
	Type     ErrorType     `json:"type"`
	Severity ErrorSeverity `json:"severity"`

	// Context and metadata
	Context   ErrorContext    `json:"context,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
	Location  *SourceLocation `json:"location,omitempty"`

	// Error chain
	Cause error `json:"-"`

	// Suggestions for resolution
	Suggestions []string `json:"suggestions,omitempty"`
}

// SourceLocation captures where the error occurred
type SourceLocation struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Function string `json:"function"`
}

// Error implements the error interface
func (e *RichError) Error() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[%s] %s", e.Code, e.Message))

	if e.Location != nil {
		sb.WriteString(fmt.Sprintf(" (at %s:%d)", e.Location.File, e.Location.Line))
	}

	if e.Cause != nil {
		sb.WriteString(fmt.Sprintf(" - caused by: %v", e.Cause))
	}

	return sb.String()
}

// Unwrap returns the cause of the error
func (e *RichError) Unwrap() error {
	return e.Cause
}

// MarshalJSON customizes JSON serialization
func (e *RichError) MarshalJSON() ([]byte, error) {
	type Alias RichError
	data, err := json.Marshal(&struct {
		*Alias
		CauseMessage string `json:"cause,omitempty"`
	}{
		Alias: (*Alias)(e),
		CauseMessage: func() string {
			if e.Cause != nil {
				return e.Cause.Error()
			}
			return ""
		}(),
	})
	if err != nil {
		// Log error but don't fail catastrophically
		return []byte(fmt.Sprintf(`{"code":"%s","message":"%s","error":"marshal_failed"}`, e.Code, e.Message)), nil
	}
	return data, nil
}

// ErrorBuilder provides a fluent API for constructing RichError instances
type ErrorBuilder struct {
	err *RichError
}

// NewError creates a new error builder
func NewError() *ErrorBuilder {
	return &ErrorBuilder{
		err: &RichError{
			Timestamp: time.Now(),
			Type:      ErrTypeInternal,
			Severity:  SeverityMedium,
		},
	}
}

// Code sets the error code
func (b *ErrorBuilder) Code(code ErrorCode) *ErrorBuilder {
	b.err.Code = code
	return b
}

// Message sets the error message
func (b *ErrorBuilder) Message(message string) *ErrorBuilder {
	b.err.Message = message
	return b
}

// Messagef sets a formatted error message
// Supports %w error wrapping verb like fmt.Errorf
func (b *ErrorBuilder) Messagef(format string, args ...interface{}) *ErrorBuilder {
	// Check if format contains %w and extract the error
	if strings.Contains(format, "%w") {
		// Find the error argument that corresponds to %w
		for _, arg := range args {
			if err, ok := arg.(error); ok {
				b.err.Cause = err
				// Replace %w with %v for the message formatting
				format = strings.ReplaceAll(format, "%w", "%v")
				break
			}
		}
	}
	b.err.Message = fmt.Sprintf(format, args...)
	return b
}

// Type sets the error type
func (b *ErrorBuilder) Type(errType ErrorType) *ErrorBuilder {
	b.err.Type = errType
	return b
}

// Severity sets the error severity
func (b *ErrorBuilder) Severity(severity ErrorSeverity) *ErrorBuilder {
	b.err.Severity = severity
	return b
}

// Context adds a context key-value pair
func (b *ErrorBuilder) Context(key string, value interface{}) *ErrorBuilder {
	if b.err.Context == nil {
		b.err.Context = make(ErrorContext)
	}
	b.err.Context[key] = value
	return b
}

// WithField adds context information to the error (alias for Context)
func (b *ErrorBuilder) WithField(key string, value interface{}) *ErrorBuilder {
	return b.Context(key, value)
}

// Cause sets the error cause
func (b *ErrorBuilder) Cause(err error) *ErrorBuilder {
	b.err.Cause = err
	return b
}

// Suggestion adds a suggestion for resolution
func (b *ErrorBuilder) Suggestion(suggestion string) *ErrorBuilder {
	b.err.Suggestions = append(b.err.Suggestions, suggestion)
	return b
}

// WithLocation captures the source location
func (b *ErrorBuilder) WithLocation() *ErrorBuilder {
	if pc, file, line, ok := runtime.Caller(2); ok {
		b.err.Location = &SourceLocation{
			File:     file,
			Line:     line,
			Function: runtime.FuncForPC(pc).Name(),
		}
	}
	return b
}

// Build returns the constructed RichError
func (b *ErrorBuilder) Build() *RichError {
	return b.err
}
