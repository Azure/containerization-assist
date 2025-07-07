package config

import (
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// MigrateValidationError converts legacy ValidationError to RichError
func MigrateValidationError(legacyErr *ValidationError) *errors.RichError {
	return errors.ToolConfigValidationError(
		legacyErr.Field,
		legacyErr.Message,
		nil,
	)
}

// NewRichConfigValidationError creates a new rich validation error for configuration
func NewRichConfigValidationError(field, message string) *errors.RichError {
	return errors.ToolConfigValidationError(field, message, nil)
}

// NewRichValidationErrorWithValue creates a new rich validation error with value
func NewRichValidationErrorWithValue(field, message string, value interface{}) *errors.RichError {
	return errors.ToolConfigValidationError(field, message, value)
}

// ValidateConfigField validates a configuration field with type checking
func ValidateConfigField(field string, value interface{}, expectedType string, required bool) *errors.RichError {
	// Check if required field is missing
	if required && value == nil {
		return errors.MissingParameterError(field)
	}

	// If value is nil and not required, that's okay
	if value == nil {
		return nil
	}

	// Validate type
	actualType := getValueType(value)
	if actualType != expectedType {
		return errors.TypeConversionError(actualType, expectedType, value)
	}

	return nil
}

// ValidateConfigString validates string configuration with constraints
func ValidateConfigString(field string, value string, required bool, minLength, maxLength int) *errors.RichError {
	if required && value == "" {
		return errors.MissingParameterError(field)
	}

	if value == "" && !required {
		return nil
	}

	length := len(value)
	if length < minLength {
		return errors.ToolConfigValidationError(
			field,
			"string value is too short",
			value,
		)
	}

	if maxLength > 0 && length > maxLength {
		return errors.ToolConfigValidationError(
			field,
			"string value is too long",
			value,
		)
	}

	return nil
}

// ValidateConfigNumber validates numeric configuration with range
func ValidateConfigNumber(field string, value interface{}, required bool, min, max float64) *errors.RichError {
	if required && value == nil {
		return errors.MissingParameterError(field)
	}

	if value == nil && !required {
		return nil
	}

	numValue, err := convertToFloat64(value)
	if err != nil {
		return errors.TypeConversionError(getValueType(value), "number", value)
	}

	if numValue < min {
		return errors.ToolConfigValidationError(
			field,
			"numeric value is below minimum",
			value,
		)
	}

	if numValue > max {
		return errors.ToolConfigValidationError(
			field,
			"numeric value is above maximum",
			value,
		)
	}

	return nil
}

// ValidateConfigEnum validates that a config value is in allowed set
func ValidateConfigEnum(field string, value interface{}, required bool, allowedValues []interface{}) *errors.RichError {
	if required && value == nil {
		return errors.MissingParameterError(field)
	}

	if value == nil && !required {
		return nil
	}

	for _, allowed := range allowedValues {
		if value == allowed {
			return nil
		}
	}

	return errors.ToolConfigValidationError(
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
		return 0, errors.TypeConversionError(getValueType(value), "number", value)
	}
}
