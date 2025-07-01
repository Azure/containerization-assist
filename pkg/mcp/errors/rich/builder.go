package rich

import (
	"fmt"
	"time"
)

// ErrorBuilder provides a fluent API for constructing RichError instances
type ErrorBuilder struct {
	err *RichError
}

// NewError creates a new error builder
func NewError() *ErrorBuilder {
	return &ErrorBuilder{
		err: &RichError{
			Timestamp: time.Now(),
			Type:      ErrTypeInternal, // Default type
			Severity:  SeverityMedium,  // Default severity
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
func (b *ErrorBuilder) Messagef(format string, args ...interface{}) *ErrorBuilder {
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

// Contexts adds multiple context values
func (b *ErrorBuilder) Contexts(contexts ErrorContext) *ErrorBuilder {
	if b.err.Context == nil {
		b.err.Context = make(ErrorContext)
	}
	for k, v := range contexts {
		b.err.Context[k] = v
	}
	return b
}

// Cause sets the underlying cause
func (b *ErrorBuilder) Cause(cause error) *ErrorBuilder {
	b.err.Cause = cause
	if cause != nil {
		b.err.CauseText = cause.Error()
	}
	return b
}

// Suggestion adds a suggestion for resolving the error
func (b *ErrorBuilder) Suggestion(suggestion string) *ErrorBuilder {
	b.err.Suggestion = suggestion
	return b
}

// HelpURL adds a help URL
func (b *ErrorBuilder) HelpURL(url string) *ErrorBuilder {
	b.err.HelpURL = url
	return b
}

// SessionID sets the session ID
func (b *ErrorBuilder) SessionID(id string) *ErrorBuilder {
	b.err.SessionID = id
	return b
}

// RequestID sets the request ID
func (b *ErrorBuilder) RequestID(id string) *ErrorBuilder {
	b.err.RequestID = id
	return b
}

// UserID sets the user ID
func (b *ErrorBuilder) UserID(id string) *ErrorBuilder {
	b.err.UserID = id
	return b
}

// WithLocation captures the current location with default skip
func (b *ErrorBuilder) WithLocation() *ErrorBuilder {
	b.err.CaptureLocation(1)
	return b
}

// WithLocationSkip captures location with custom skip
func (b *ErrorBuilder) WithLocationSkip(skip int) *ErrorBuilder {
	b.err.CaptureLocation(skip + 1)
	return b
}

// WithStack captures the current stack trace with default skip
func (b *ErrorBuilder) WithStack() *ErrorBuilder {
	b.err.CaptureStack(1)
	return b
}

// WithStackSkip captures stack trace with custom skip
func (b *ErrorBuilder) WithStackSkip(skip int) *ErrorBuilder {
	b.err.CaptureStack(skip + 1)
	return b
}

// Build finalizes and returns the RichError
func (b *ErrorBuilder) Build() *RichError {
	// Set default message if not provided
	if b.err.Message == "" && b.err.Code != "" {
		b.err.Message = string(b.err.Code)
	}

	// Ensure code is set
	if b.err.Code == "" {
		b.err.Code = CodeUnknownError
	}

	return b.err
}

// Quick constructors for common patterns

// ValidationError creates a validation error
func ValidationError(message string) *ErrorBuilder {
	return NewError().
		Type(ErrTypeValidation).
		Code(CodeValidationFailed).
		Message(message).
		Severity(SeverityMedium)
}

// NetworkError creates a network error
func NetworkError(message string) *ErrorBuilder {
	return NewError().
		Type(ErrTypeNetwork).
		Code(CodeConnectionFailed).
		Message(message).
		Severity(SeverityHigh)
}

// SecurityError creates a security error
func SecurityError(message string) *ErrorBuilder {
	return NewError().
		Type(ErrTypeSecurity).
		Code(CodeAuthenticationFailed).
		Message(message).
		Severity(SeverityCritical)
}

// ResourceError creates a resource error
func ResourceError(message string) *ErrorBuilder {
	return NewError().
		Type(ErrTypeResource).
		Code(CodeResourceNotFound).
		Message(message).
		Severity(SeverityMedium)
}

// ConfigurationError creates a configuration error
func ConfigurationError(message string) *ErrorBuilder {
	return NewError().
		Type(ErrTypeConfiguration).
		Code(CodeInvalidParameter).
		Message(message).
		Severity(SeverityHigh)
}

// InternalError creates an internal error
func InternalError(message string) *ErrorBuilder {
	return NewError().
		Type(ErrTypeInternal).
		Code(CodeInternalError).
		Message(message).
		Severity(SeverityCritical)
}

// NotImplementedError creates a not implemented error
func NotImplementedError(feature string) *ErrorBuilder {
	return NewError().
		Type(ErrTypeInternal).
		Code(CodeNotImplemented).
		Messagef("Feature not implemented: %s", feature).
		Severity(SeverityLow)
}

// Chain helpers for wrapping errors

// Wrap wraps an existing error with additional context
func Wrap(err error, message string) *ErrorBuilder {
	if err == nil {
		return nil
	}

	// If already a RichError, preserve its properties
	if re, ok := err.(*RichError); ok {
		return NewError().
			Code(re.Code).
			Type(re.Type).
			Severity(re.Severity).
			Message(message).
			Cause(err).
			Contexts(re.Context)
	}

	// Otherwise create a new RichError
	return NewError().
		Code(CodeUnknownError).
		Message(message).
		Cause(err)
}

// Wrapf wraps an error with a formatted message
func Wrapf(err error, format string, args ...interface{}) *ErrorBuilder {
	if err == nil {
		return nil
	}
	return Wrap(err, fmt.Sprintf(format, args...))
}
