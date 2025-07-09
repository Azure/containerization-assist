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
