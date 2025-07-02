package errors

import (
	"fmt"

	"github.com/Azure/container-kit/pkg/mcp/errors/rich"
)

// MigrateToRichError converts a legacy MCPError to a RichError
func MigrateToRichError(legacyErr *MCPError) *rich.RichError {
	if legacyErr == nil {
		return nil
	}

	// Map legacy categories to rich error types and codes
	errorType, errorCode := mapCategoryToTypeAndCode(legacyErr.Category)

	// Determine severity based on category
	severity := mapCategoryToSeverity(legacyErr.Category)

	// Create the rich error
	richErr := rich.NewError().
		Code(errorCode).
		Type(errorType).
		Severity(severity).
		Message(legacyErr.Message)

	// Add module context
	if legacyErr.Module != "" {
		richErr = richErr.Context("module", legacyErr.Module)
	}

	// Add operation context
	if legacyErr.Operation != "" {
		richErr = richErr.Context("operation", legacyErr.Operation)
	}

	// Add legacy context
	for key, value := range legacyErr.Context {
		richErr = richErr.Context(key, value)
	}

	// Add retryability context
	richErr = richErr.Context("retryable", legacyErr.Retryable)
	richErr = richErr.Context("recoverable", legacyErr.Recoverable)

	// Add cause if present
	if legacyErr.Cause != nil {
		richErr = richErr.Cause(legacyErr.Cause)
	}

	// Add location and build
	return richErr.WithLocation().Build()
}

// mapCategoryToTypeAndCode maps legacy error categories to rich error types and codes
func mapCategoryToTypeAndCode(category ErrorCategory) (rich.ErrorType, rich.ErrorCode) {
	switch category {
	case CategoryValidation:
		return rich.ErrTypeValidation, rich.CodeInvalidParameter
	case CategoryNetwork:
		return rich.ErrTypeNetwork, rich.CodeConnectionFailed
	case CategoryInternal:
		return rich.ErrTypeInternal, rich.CodeInternalError
	case CategoryAuth:
		return rich.ErrTypeSecurity, rich.CodeAuthenticationFailed
	case CategoryResource:
		return rich.ErrTypeResource, rich.CodeResourceNotFound
	case CategoryTimeout:
		return rich.ErrTypeTimeout, rich.CodeNetworkTimeout
	case CategoryConfig:
		return rich.ErrTypeConfiguration, rich.CodeInvalidParameter
	default:
		return rich.ErrTypeInternal, rich.CodeUnknownError
	}
}

// mapCategoryToSeverity maps legacy error categories to rich error severities
func mapCategoryToSeverity(category ErrorCategory) rich.ErrorSeverity {
	switch category {
	case CategoryValidation:
		return rich.SeverityMedium
	case CategoryNetwork:
		return rich.SeverityHigh
	case CategoryInternal:
		return rich.SeverityCritical
	case CategoryAuth:
		return rich.SeverityCritical
	case CategoryResource:
		return rich.SeverityMedium
	case CategoryTimeout:
		return rich.SeverityHigh
	case CategoryConfig:
		return rich.SeverityHigh
	default:
		return rich.SeverityMedium
	}
}

// Helper functions to create rich errors using legacy-style constructors
// These provide a migration path for code currently using the legacy system

// NewRichValidation creates a validation rich error (replaces errors.Validation)
func NewRichValidation(module, message string) *rich.RichError {
	return rich.NewError().
		Code(rich.CodeInvalidParameter).
		Type(rich.ErrTypeValidation).
		Severity(rich.SeverityMedium).
		Message(message).
		Context("module", module).
		WithLocation().
		Build()
}

// NewRichValidationf creates a validation rich error with formatting (replaces errors.Validationf)
func NewRichValidationf(module, format string, args ...interface{}) *rich.RichError {
	return rich.NewError().
		Code(rich.CodeInvalidParameter).
		Type(rich.ErrTypeValidation).
		Severity(rich.SeverityMedium).
		Messagef(format, args...).
		Context("module", module).
		WithLocation().
		Build()
}

// NewRichNetwork creates a network rich error (replaces errors.Network)
func NewRichNetwork(module, message string) *rich.RichError {
	return rich.NewError().
		Code(rich.CodeConnectionFailed).
		Type(rich.ErrTypeNetwork).
		Severity(rich.SeverityHigh).
		Message(message).
		Context("module", module).
		Context("retryable", true).
		WithLocation().
		Build()
}

// NewRichNetworkf creates a network rich error with formatting (replaces errors.Networkf)
func NewRichNetworkf(module, format string, args ...interface{}) *rich.RichError {
	return rich.NewError().
		Code(rich.CodeConnectionFailed).
		Type(rich.ErrTypeNetwork).
		Severity(rich.SeverityHigh).
		Messagef(format, args...).
		Context("module", module).
		Context("retryable", true).
		WithLocation().
		Build()
}

// NewRichInternal creates an internal rich error (replaces errors.Internal)
func NewRichInternal(module, message string) *rich.RichError {
	return rich.NewError().
		Code(rich.CodeInternalError).
		Type(rich.ErrTypeInternal).
		Severity(rich.SeverityCritical).
		Message(message).
		Context("module", module).
		WithLocation().
		Build()
}

// NewRichInternalf creates an internal rich error with formatting (replaces errors.Internalf)
func NewRichInternalf(module, format string, args ...interface{}) *rich.RichError {
	return rich.NewError().
		Code(rich.CodeInternalError).
		Type(rich.ErrTypeInternal).
		Severity(rich.SeverityCritical).
		Messagef(format, args...).
		Context("module", module).
		WithLocation().
		Build()
}

// NewRichResource creates a resource rich error (replaces errors.Resource)
func NewRichResource(module, message string) *rich.RichError {
	return rich.NewError().
		Code(rich.CodeResourceNotFound).
		Type(rich.ErrTypeResource).
		Severity(rich.SeverityMedium).
		Message(message).
		Context("module", module).
		WithLocation().
		Build()
}

// NewRichResourcef creates a resource rich error with formatting (replaces errors.Resourcef)
func NewRichResourcef(module, format string, args ...interface{}) *rich.RichError {
	return rich.NewError().
		Code(rich.CodeResourceNotFound).
		Type(rich.ErrTypeResource).
		Severity(rich.SeverityMedium).
		Messagef(format, args...).
		Context("module", module).
		WithLocation().
		Build()
}

// NewRichTimeout creates a timeout rich error (replaces errors.Timeout)
func NewRichTimeout(module, message string) *rich.RichError {
	return rich.NewError().
		Code(rich.CodeNetworkTimeout).
		Type(rich.ErrTypeTimeout).
		Severity(rich.SeverityHigh).
		Message(message).
		Context("module", module).
		Context("retryable", true).
		WithLocation().
		Build()
}

// NewRichTimeoutf creates a timeout rich error with formatting (replaces errors.Timeoutf)
func NewRichTimeoutf(module, format string, args ...interface{}) *rich.RichError {
	return rich.NewError().
		Code(rich.CodeNetworkTimeout).
		Type(rich.ErrTypeTimeout).
		Severity(rich.SeverityHigh).
		Messagef(format, args...).
		Context("module", module).
		Context("retryable", true).
		WithLocation().
		Build()
}

// NewRichConfig creates a configuration rich error (replaces errors.Config)
func NewRichConfig(module, message string) *rich.RichError {
	return rich.NewError().
		Code(rich.CodeInvalidParameter).
		Type(rich.ErrTypeConfiguration).
		Severity(rich.SeverityHigh).
		Message(message).
		Context("module", module).
		WithLocation().
		Build()
}

// NewRichConfigf creates a configuration rich error with formatting (replaces errors.Configf)
func NewRichConfigf(module, format string, args ...interface{}) *rich.RichError {
	return rich.NewError().
		Code(rich.CodeInvalidParameter).
		Type(rich.ErrTypeConfiguration).
		Severity(rich.SeverityHigh).
		Messagef(format, args...).
		Context("module", module).
		WithLocation().
		Build()
}

// NewRichAuth creates an authentication/authorization rich error (replaces errors.Auth)
func NewRichAuth(module, message string) *rich.RichError {
	return rich.NewError().
		Code(rich.CodeAuthenticationFailed).
		Type(rich.ErrTypeSecurity).
		Severity(rich.SeverityCritical).
		Message(message).
		Context("module", module).
		WithLocation().
		Build()
}

// NewRichAuthf creates an authentication/authorization rich error with formatting (replaces errors.Authf)
func NewRichAuthf(module, format string, args ...interface{}) *rich.RichError {
	return rich.NewError().
		Code(rich.CodeAuthenticationFailed).
		Type(rich.ErrTypeSecurity).
		Severity(rich.SeverityCritical).
		Messagef(format, args...).
		Context("module", module).
		WithLocation().
		Build()
}

// WrapRich wraps an existing error with rich error context (replaces errors.Wrap)
func WrapRich(err error, module, message string) *rich.RichError {
	if err == nil {
		return nil
	}

	// For any error type, create a new rich error that wraps it
	return rich.NewError().
		Code(rich.CodeInternalError).
		Type(rich.ErrTypeInternal).
		Severity(rich.SeverityMedium).
		Message(message).
		Context("module", module).
		Context("wrapped_error_type", fmt.Sprintf("%T", err)).
		Cause(err).
		WithLocation().
		Build()
}

// WrapRichf wraps an existing error with formatted rich error context (replaces errors.Wrapf)
func WrapRichf(err error, module, format string, args ...interface{}) *rich.RichError {
	return WrapRich(err, module, fmt.Sprintf(format, args...))
}
