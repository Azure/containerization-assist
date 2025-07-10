package errors

import (
	"fmt"
)

// MissingParameterError creates an error for missing required parameters
func MissingParameterError(paramName string) *RichError {
	return NewError().
		Code(CodeMissingParameter).
		Type(ErrTypeValidation).
		Severity(SeverityMedium).
		Messagef("Required parameter '%s' is missing", paramName).
		Context("parameter", paramName).
		Suggestion(fmt.Sprintf("Provide a value for the required parameter '%s'", paramName)).
		WithLocation().
		Build()
}

// TypeConversionError creates an error for type conversion failures
func TypeConversionError(fromType, toType string, value interface{}) *RichError {
	return NewError().
		Code(CodeTypeConversionFailed).
		Type(ErrTypeValidation).
		Severity(SeverityMedium).
		Messagef("Cannot convert %s to %s", fromType, toType).
		Context("from_type", fromType).
		Context("to_type", toType).
		Context("value", value).
		Suggestion(fmt.Sprintf("Ensure the value can be converted from %s to %s", fromType, toType)).
		WithLocation().
		Build()
}

// DockerBuildGenericError creates a generic Docker build error
func DockerBuildGenericError(message string, details map[string]interface{}) *RichError {
	builder := NewError().
		Code(CodeImageBuildFailed).
		Type(ErrTypeContainer).
		Severity(SeverityHigh).
		Message(message)

	for k, v := range details {
		builder = builder.Context(k, v)
	}

	return builder.
		Suggestion("Check Docker daemon status and build configuration").
		WithLocation().
		Build()
}

// ImagePullError creates an error for image pull failures
func ImagePullError(imageRef string, cause error) *RichError {
	return NewError().
		Code(CodeImagePullFailed).
		Type(ErrTypeContainer).
		Severity(SeverityHigh).
		Messagef("Failed to pull image: %s", imageRef).
		Context("image", imageRef).
		Cause(cause).
		Suggestion("Check image name, registry access, and network connectivity").
		WithLocation().
		Build()
}

// ImagePushError creates an error for image push failures
func ImagePushError(imageRef, registry string, cause error) *RichError {
	return NewError().
		Code(CodeImagePushFailed).
		Type(ErrTypeContainer).
		Severity(SeverityHigh).
		Messagef("Failed to push image %s to registry %s", imageRef, registry).
		Context("image", imageRef).
		Context("registry", registry).
		Cause(cause).
		Suggestion("Check registry credentials and network connectivity").
		WithLocation().
		Build()
}

// ToolValidationError creates a validation error for tool parameters
func ToolValidationError(toolName, field, message, code string, value interface{}) *RichError {
	builder := NewError().
		Code(CodeValidationFailed).
		Type(ErrTypeValidation).
		Severity(SeverityMedium).
		Messagef("Tool '%s' validation failed for field '%s': %s", toolName, field, message).
		Context("tool", toolName).
		Context("field", field)

	if code != "" {
		builder = builder.Context("validation_code", code)
	}

	if value != nil {
		builder = builder.Context("value", value)
	}

	return builder.
		Suggestion(fmt.Sprintf("Check the value of field '%s' in tool '%s'", field, toolName)).
		WithLocation().
		Build()
}

// ToolConfigValidationError creates a validation error for tool configuration
func ToolConfigValidationError(field, message string, value interface{}) *RichError {
	builder := NewError().
		Code(CodeValidationFailed).
		Type(ErrTypeValidation).
		Severity(SeverityMedium).
		Messagef("Configuration validation failed for field '%s': %s", field, message).
		Context("field", field)

	if value != nil {
		builder = builder.Context("value", value)
	}

	return builder.
		Suggestion(fmt.Sprintf("Check the configuration value for field '%s'", field)).
		WithLocation().
		Build()
}

// ToolConstraintViolationError creates an error for constraint violations
func ToolConstraintViolationError(field, constraint, message string, value interface{}) *RichError {
	return NewError().
		Code(CodeValidationFailed).
		Type(ErrTypeValidation).
		Severity(SeverityMedium).
		Messagef("Constraint '%s' violated for field '%s': %s", constraint, field, message).
		Context("field", field).
		Context("constraint", constraint).
		Context("value", value).
		Suggestion(fmt.Sprintf("Ensure field '%s' meets the '%s' constraint", field, constraint)).
		WithLocation().
		Build()
}

// CoreError is a simplified error for basic cases
type CoreError struct {
	Code    ErrorCode
	Message string
}

func (e *CoreError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// NewCoreError creates a simple core error
func NewCoreError(code ErrorCode, message string) *CoreError {
	return &CoreError{
		Code:    code,
		Message: message,
	}
}

// NewMissingParam creates a validation error for missing required parameters
// This is an alias for MissingParameterError to match the naming in WORKSTREAM_GAMMA_PROMPT.md
func NewMissingParam(field string) error {
	return MissingParameterError(field)
}

// NewValidationFailed creates a validation error with context
func NewValidationFailed(field, reason string) error {
	return NewError().
		Code(CodeValidationFailed).
		Type(ErrTypeValidation).
		Severity(SeverityMedium).
		Messagef("validation failed for %s: %s", field, reason).
		Context("field", field).
		Context("reason", reason).
		Suggestion("Check the field value and format").
		WithLocation().
		Build()
}

// NewInternalError creates an internal error wrapping a cause
func NewInternalError(operation string, cause error) error {
	return NewError().
		Code(CodeInternalError).
		Type(ErrTypeInternal).
		Severity(SeverityHigh).
		Messagef("internal error during %s", operation).
		Context("operation", operation).
		Cause(cause).
		WithLocation().
		Build()
}

// NewConfigurationError creates a configuration error
func NewConfigurationError(component, issue string) error {
	return NewError().
		Code(CodeConfigurationInvalid).
		Type(ErrTypeConfiguration).
		Severity(SeverityHigh).
		Messagef("configuration error in %s: %s", component, issue).
		Context("component", component).
		Context("issue", issue).
		Suggestion("Check configuration file and environment variables").
		WithLocation().
		Build()
}

// NewNotFoundError creates a not found error
func NewNotFoundError(resource, identifier string) error {
	return NewError().
		Code(CodeNotFound).
		Type(ErrTypeNotFound).
		Severity(SeverityMedium).
		Messagef("%s not found: %s", resource, identifier).
		Context("resource", resource).
		Context("identifier", identifier).
		WithLocation().
		Build()
}

// NewPermissionDeniedError creates a permission denied error
func NewPermissionDeniedError(resource, action string) error {
	return NewError().
		Code(CodePermissionDenied).
		Type(ErrTypePermission).
		Severity(SeverityHigh).
		Messagef("permission denied for %s on %s", action, resource).
		Context("resource", resource).
		Context("action", action).
		Suggestion("Check access permissions and authentication").
		WithLocation().
		Build()
}

// NewTimeoutError creates a timeout error
func NewTimeoutError(operation string, duration string) error {
	return NewError().
		Code(CodeTimeoutError).
		Type(ErrTypeTimeout).
		Severity(SeverityHigh).
		Messagef("operation %s timed out after %s", operation, duration).
		Context("operation", operation).
		Context("duration", duration).
		Suggestion("Increase timeout or check operation performance").
		WithLocation().
		Build()
}

// NewNetworkError creates a network error
func NewNetworkError(operation string, cause error) error {
	return NewError().
		Code(CodeNetworkError).
		Type(ErrTypeNetwork).
		Severity(SeverityHigh).
		Messagef("network error during %s", operation).
		Context("operation", operation).
		Cause(cause).
		Suggestion("Check network connectivity and firewall rules").
		WithLocation().
		Build()
}

// NewAlreadyExistsError creates an already exists error
func NewAlreadyExistsError(resource, identifier string) error {
	return NewError().
		Code(CodeAlreadyExists).
		Type(ErrTypeConflict).
		Severity(SeverityMedium).
		Messagef("%s already exists: %s", resource, identifier).
		Context("resource", resource).
		Context("identifier", identifier).
		Suggestion("Use a different identifier or update the existing resource").
		WithLocation().
		Build()
}

// NewOperationFailedError creates a generic operation failed error
func NewOperationFailedError(operation, reason string, cause error) error {
	builder := NewError().
		Code(CodeOperationFailed).
		Type(ErrTypeOperation).
		Severity(SeverityHigh).
		Messagef("operation %s failed: %s", operation, reason).
		Context("operation", operation).
		Context("reason", reason).
		WithLocation()
	
	if cause != nil {
		builder = builder.Cause(cause)
	}
	
	return builder.Build()
}

// NewSecurityError creates a security error
func NewSecurityError(violation string, details map[string]interface{}) error {
	builder := NewError().
		Code(CodeSecurityViolation).
		Type(ErrTypeSecurity).
		Severity(SeverityCritical).
		Messagef("security violation: %s", violation).
		Context("violation", violation).
		Suggestion("Review security policies and access controls").
		WithLocation()
	
	for k, v := range details {
		builder = builder.Context(k, v)
	}
	
	return builder.Build()
}

// NewMultiError creates an error that aggregates multiple errors
func NewMultiError(operation string, errors []error) error {
	if len(errors) == 0 {
		return nil
	}
	
	if len(errors) == 1 {
		return errors[0]
	}
	
	errorMessages := make([]string, len(errors))
	for i, err := range errors {
		errorMessages[i] = err.Error()
	}
	
	return NewError().
		Code(CodeOperationFailed).
		Type(ErrTypeOperation).
		Severity(SeverityHigh).
		Messagef("multiple errors during %s: %d errors occurred", operation, len(errors)).
		Context("operation", operation).
		Context("error_count", len(errors)).
		Context("errors", errorMessages).
		Suggestion("Review individual errors for specific issues").
		WithLocation().
		Build()
}
