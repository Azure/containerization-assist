package rich

import (
	"fmt"
	"runtime"
	"time"
)

// ErrorCode represents a unique error identifier
type ErrorCode string

// Common error codes
const (
	// Validation errors
	CodeValidationFailed     ErrorCode = "VALIDATION_FAILED"
	CodeInvalidParameter     ErrorCode = "INVALID_PARAMETER"
	CodeMissingParameter     ErrorCode = "MISSING_PARAMETER"
	CodeTypeConversionFailed ErrorCode = "TYPE_CONVERSION_FAILED"

	// Network errors
	CodeNetworkTimeout      ErrorCode = "NETWORK_TIMEOUT"
	CodeConnectionFailed    ErrorCode = "CONNECTION_FAILED"
	CodeDNSResolutionFailed ErrorCode = "DNS_RESOLUTION_FAILED"

	// Security errors
	CodeAuthenticationFailed ErrorCode = "AUTHENTICATION_FAILED"
	CodeAuthorizationFailed  ErrorCode = "AUTHORIZATION_FAILED"
	CodeSecretNotFound       ErrorCode = "SECRET_NOT_FOUND"
	CodeCertificateInvalid   ErrorCode = "CERTIFICATE_INVALID"

	// Resource errors
	CodeResourceNotFound      ErrorCode = "RESOURCE_NOT_FOUND"
	CodeResourceAlreadyExists ErrorCode = "RESOURCE_ALREADY_EXISTS"
	CodeResourceLocked        ErrorCode = "RESOURCE_LOCKED"
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

	// Generic errors
	CodeUnknownError   ErrorCode = "UNKNOWN_ERROR"
	CodeInternalError  ErrorCode = "INTERNAL_ERROR"
	CodeNotImplemented ErrorCode = "NOT_IMPLEMENTED"
)

// ErrorType categorizes errors by their nature
type ErrorType string

const (
	ErrTypeValidation    ErrorType = "VALIDATION"
	ErrTypeNetwork       ErrorType = "NETWORK"
	ErrTypeSecurity      ErrorType = "SECURITY"
	ErrTypeResource      ErrorType = "RESOURCE"
	ErrTypeBusiness      ErrorType = "BUSINESS"
	ErrTypeConfiguration ErrorType = "CONFIGURATION"
	ErrTypeInternal      ErrorType = "INTERNAL"
	ErrTypeNotFound      ErrorType = "NOT_FOUND"
	ErrTypeSystem        ErrorType = "SYSTEM"
	ErrTypeTimeout       ErrorType = "TIMEOUT"
	ErrTypePermission    ErrorType = "PERMISSION"
)

// ErrorSeverity indicates the impact level of an error
type ErrorSeverity string

const (
	SeverityCritical ErrorSeverity = "CRITICAL"
	SeverityHigh     ErrorSeverity = "HIGH"
	SeverityMedium   ErrorSeverity = "MEDIUM"
	SeverityLow      ErrorSeverity = "LOW"
	SeverityInfo     ErrorSeverity = "INFO"
)

// ErrorContext holds contextual information about an error
type ErrorContext map[string]interface{}

// ErrorLocation captures where an error occurred
type ErrorLocation struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Function string `json:"function"`
}

// StackFrame represents a single frame in a stack trace
type StackFrame struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Function string `json:"function"`
}

// RichError provides comprehensive error information with context
type RichError struct {
	// Core fields
	Code     ErrorCode     `json:"code"`
	Message  string        `json:"message"`
	Type     ErrorType     `json:"type"`
	Severity ErrorSeverity `json:"severity"`

	// Context and metadata
	Context   ErrorContext   `json:"context,omitempty"`
	Location  *ErrorLocation `json:"location,omitempty"`
	Timestamp time.Time      `json:"timestamp"`

	// Stack trace
	Stack []StackFrame `json:"stack,omitempty"`

	// Cause chain
	Cause     error  `json:"-"`
	CauseText string `json:"cause,omitempty"`

	// Help information
	Suggestion string `json:"suggestion,omitempty"`
	HelpURL    string `json:"help_url,omitempty"`

	// Additional metadata
	SessionID string `json:"session_id,omitempty"`
	RequestID string `json:"request_id,omitempty"`
	UserID    string `json:"user_id,omitempty"`
}

// Error implements the error interface
func (e *RichError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %s (caused by: %v)", e.Code, e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s: %s", e.Code, e.Type, e.Message)
}

// Unwrap returns the underlying cause
func (e *RichError) Unwrap() error {
	return e.Cause
}

// WithContext adds or updates context values
func (e *RichError) WithContext(key string, value interface{}) *RichError {
	if e.Context == nil {
		e.Context = make(ErrorContext)
	}
	e.Context[key] = value
	return e
}

// AddStackFrame adds a stack frame
func (e *RichError) AddStackFrame(frame StackFrame) *RichError {
	e.Stack = append(e.Stack, frame)
	return e
}

// CaptureStack captures the current stack trace
func (e *RichError) CaptureStack(skip int) *RichError {
	const maxDepth = 32
	pcs := make([]uintptr, maxDepth)
	n := runtime.Callers(skip+2, pcs)

	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		if frame.Function == "" {
			break
		}

		e.Stack = append(e.Stack, StackFrame{
			File:     frame.File,
			Line:     frame.Line,
			Function: frame.Function,
		})

		if !more {
			break
		}
	}

	return e
}

// CaptureLocation captures the current code location
func (e *RichError) CaptureLocation(skip int) *RichError {
	pc, file, line, ok := runtime.Caller(skip + 1)
	if ok {
		fn := runtime.FuncForPC(pc)
		e.Location = &ErrorLocation{
			File:     file,
			Line:     line,
			Function: fn.Name(),
		}
	}
	return e
}

// Is checks if the error matches a target error
func (e *RichError) Is(target error) bool {
	if target == nil {
		return false
	}

	// Check if target is also a RichError
	if re, ok := target.(*RichError); ok {
		return e.Code == re.Code && e.Type == re.Type
	}

	// Check the cause chain
	return e.Cause != nil && e.Cause == target
}

// GetContext retrieves a context value
func (e *RichError) GetContext(key string) (interface{}, bool) {
	if e.Context == nil {
		return nil, false
	}
	val, ok := e.Context[key]
	return val, ok
}

// HasCode checks if the error has a specific code
func (e *RichError) HasCode(code ErrorCode) bool {
	return e.Code == code
}

// HasType checks if the error has a specific type
func (e *RichError) HasType(errType ErrorType) bool {
	return e.Type == errType
}

// IsSeverity checks if the error has a specific severity or higher
func (e *RichError) IsSeverity(severity ErrorSeverity) bool {
	severityOrder := map[ErrorSeverity]int{
		SeverityInfo:     1,
		SeverityLow:      2,
		SeverityMedium:   3,
		SeverityHigh:     4,
		SeverityCritical: 5,
	}

	return severityOrder[e.Severity] >= severityOrder[severity]
}
