package tools

import (
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// MigrateValidationError converts legacy ValidationError to RichError
func MigrateValidationError(toolName string, legacyErr ValidationError) *errors.RichError {
	return errors.ToolValidationError(
		toolName,
		legacyErr.Field,
		legacyErr.Message,
		legacyErr.Code,
		legacyErr.Value,
	)
}

// CreateRichValidationError creates a new rich validation error for tools
func CreateRichValidationError(toolName, field, message string) *errors.RichError {
	return errors.ToolValidationError(toolName, field, message, "", nil)
}

// CreateRichValidationErrorWithCode creates a new rich validation error with code
func CreateRichValidationErrorWithCode(toolName, field, message, code string) *errors.RichError {
	return errors.ToolValidationError(toolName, field, message, code, nil)
}

// NewRichValidationErrorWithValue creates a new rich validation error with value
func NewRichValidationErrorWithValue(toolName, field, message string, value interface{}) *errors.RichError {
	return errors.ToolValidationError(toolName, field, message, "", value)
}

// NewRichValidationErrorComplete creates a new rich validation error with all parameters
func NewRichValidationErrorComplete(toolName, field, message, code string, value interface{}) *errors.RichError {
	return errors.ToolValidationError(toolName, field, message, code, value)
}

// ValidateRequiredField validates that a field is not empty
func ValidateRequiredField(toolName, field string, value interface{}) *errors.RichError {
	if value == nil {
		return errors.MissingParameterError(field)
	}

	// Check for empty strings
	if str, ok := value.(string); ok && str == "" {
		return errors.MissingParameterError(field)
	}

	return nil
}

// ValidateFieldType validates that a field is of the expected type
func ValidateFieldType(toolName, field, expectedType string, value interface{}) *errors.RichError {
	actualType := ""
	switch value.(type) {
	case string:
		actualType = "string"
	case int, int8, int16, int32, int64:
		actualType = "integer"
	case uint, uint8, uint16, uint32, uint64:
		actualType = "integer"
	case float32, float64:
		actualType = "number"
	case bool:
		actualType = "boolean"
	case []interface{}:
		actualType = "array"
	case map[string]interface{}:
		actualType = "object"
	default:
		actualType = "unknown"
	}

	if actualType != expectedType {
		return errors.TypeConversionError(actualType, expectedType, value)
	}

	return nil
}

// ValidateStringLength validates string length constraints
func ValidateStringLength(toolName, field string, value string, minLength, maxLength int) *errors.RichError {
	length := len(value)

	if length < minLength {
		return errors.ToolConstraintViolationError(
			field,
			"minimum_length",
			"string is too short",
			value,
		)
	}

	if maxLength > 0 && length > maxLength {
		return errors.ToolConstraintViolationError(
			field,
			"maximum_length",
			"string is too long",
			value,
		)
	}

	return nil
}

// ValidateNumericRange validates numeric range constraints
func ValidateNumericRange(toolName, field string, value interface{}, min, max float64) *errors.RichError {
	var numValue float64

	switch v := value.(type) {
	case int:
		numValue = float64(v)
	case int32:
		numValue = float64(v)
	case int64:
		numValue = float64(v)
	case float32:
		numValue = float64(v)
	case float64:
		numValue = v
	default:
		return errors.TypeConversionError("unknown", "number", value)
	}

	if numValue < min {
		return errors.ToolConstraintViolationError(
			field,
			"minimum_value",
			"value is below minimum",
			value,
		)
	}

	if numValue > max {
		return errors.ToolConstraintViolationError(
			field,
			"maximum_value",
			"value is above maximum",
			value,
		)
	}

	return nil
}

// ValidateEnum validates that a value is in an allowed set
func ValidateEnum(toolName, field string, value interface{}, allowedValues []interface{}) *errors.RichError {
	for _, allowed := range allowedValues {
		if value == allowed {
			return nil
		}
	}

	return errors.ToolConstraintViolationError(
		field,
		"enum_constraint",
		"value not in allowed set",
		value,
	)
}
