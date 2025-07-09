package errors

import (
	"fmt"
)

// NewRichValidation creates a rich validation error
func NewRichValidation(module, message string) *RichError {
	return NewError().
		Code(CodeValidationFailed).
		Type(ErrTypeValidation).
		Severity(SeverityMedium).
		Message(message).
		Context("module", module).
		WithLocation().
		Build()
}

// NewRichNetwork creates a rich network error
func NewRichNetwork(module, message string) *RichError {
	return NewError().
		Code(CodeNetworkTimeout).
		Type(ErrTypeNetwork).
		Severity(SeverityHigh).
		Message(message).
		Context("module", module).
		WithLocation().
		Build()
}

// WrapRich wraps an error with rich error context
func WrapRich(err error, module, message string) *RichError {
	if err == nil {
		return nil
	}

	// If it's already a rich error, create a new one that wraps it
	if richErr, ok := err.(*RichError); ok {
		return NewError().
			Code(richErr.Code).
			Type(richErr.Type).
			Severity(richErr.Severity).
			Message(message).
			Context("module", module).
			Context("wrapped_error", richErr.Message).
			Cause(err).
			WithLocation().
			Build()
	}

	// Create new rich error
	return NewError().
		Code(CodeInternalError).
		Type(ErrTypeInternal).
		Severity(SeverityMedium).
		Message(message).
		Context("module", module).
		Cause(err).
		WithLocation().
		Build()
}

// WrapRichf wraps an error with formatted message
func WrapRichf(err error, module, format string, args ...interface{}) *RichError {
	return WrapRich(err, module, fmt.Sprintf(format, args...))
}
