package config

import (
	"github.com/Azure/container-kit/pkg/mcp/errors/rich"
)

// MigrateValidationError converts legacy ValidationError to RichError
func MigrateValidationError(legacyErr *ValidationError) *rich.RichError {
	return rich.ToolConfigValidationError(
		legacyErr.Field,
		legacyErr.Message,
		nil,
	)
}

// NewRichConfigValidationError creates a new rich validation error for configuration
func NewRichConfigValidationError(field, message string) *rich.RichError {
	return rich.ToolConfigValidationError(field, message, nil)
}

// NewRichValidationErrorWithValue creates a new rich validation error with value
func NewRichValidationErrorWithValue(field, message string, value interface{}) *rich.RichError {
	return rich.ToolConfigValidationError(field, message, value)
}

// ValidateConfigField validates a configuration field with type checking
func ValidateConfigField(field string, value interface{}, expectedType string, required bool) *rich.RichError {
	// Check if required field is missing
	if required && value == nil {
		return rich.MissingParameterError(field)
	}

	// If value is nil and not required, that's okay
	if value == nil {
		return nil
	}

	// Validate type
	actualType := getValueType(value)
	if actualType != expectedType {
		return rich.TypeConversionError(actualType, expectedType, value)
	}

	return nil
}

// ValidateConfigString validates string configuration with constraints
func ValidateConfigString(field string, value string, required bool, minLength, maxLength int) *rich.RichError {
	if required && value == "" {
		return rich.MissingParameterError(field)
	}

	if value == "" && !required {
		return nil
	}

	length := len(value)
	if length < minLength {
		return rich.ToolConfigValidationError(
			field,
			"string value is too short",
			value,
		)
	}

	if maxLength > 0 && length > maxLength {
		return rich.ToolConfigValidationError(
			field,
			"string value is too long",
			value,
		)
	}

	return nil
}

// ValidateConfigNumber validates numeric configuration with range
func ValidateConfigNumber(field string, value interface{}, required bool, min, max float64) *rich.RichError {
	if required && value == nil {
		return rich.MissingParameterError(field)
	}

	if value == nil && !required {
		return nil
	}

	numValue, err := convertToFloat64(value)
	if err != nil {
		return rich.TypeConversionError(getValueType(value), "number", value)
	}

	if numValue < min {
		return rich.ToolConfigValidationError(
			field,
			"numeric value is below minimum",
			value,
		)
	}

	if numValue > max {
		return rich.ToolConfigValidationError(
			field,
			"numeric value is above maximum",
			value,
		)
	}

	return nil
}

// ValidateConfigEnum validates that a config value is in allowed set
func ValidateConfigEnum(field string, value interface{}, required bool, allowedValues []interface{}) *rich.RichError {
	if required && value == nil {
		return rich.MissingParameterError(field)
	}

	if value == nil && !required {
		return nil
	}

	for _, allowed := range allowedValues {
		if value == allowed {
			return nil
		}
	}

	return rich.ToolConfigValidationError(
		field,
		"value not in allowed configuration options",
		value,
	)
}

// Helper functions

func getValueType(value interface{}) string {
	switch value.(type) {
	case string:
		return "string"
	case int, int8, int16, int32, int64:
		return "integer"
	case uint, uint8, uint16, uint32, uint64:
		return "integer"
	case float32, float64:
		return "number"
	case bool:
		return "boolean"
	case []interface{}:
		return "array"
	case map[string]interface{}:
		return "object"
	default:
		return "unknown"
	}
}

func convertToFloat64(value interface{}) (float64, error) {
	switch v := value.(type) {
	case int:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case float32:
		return float64(v), nil
	case float64:
		return v, nil
	default:
		return 0, rich.TypeConversionError(getValueType(value), "number", value)
	}
}
