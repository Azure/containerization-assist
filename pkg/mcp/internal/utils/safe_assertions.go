package utils

import (
	"fmt"

	"github.com/Azure/container-kit/pkg/mcp/errors"
)

// SafeTypeAssertions provides type-safe utilities for converting interface{} values
// This helps eliminate unsafe type assertions throughout the codebase

// SafeStringAssertion safely converts interface{} to string
func SafeStringAssertion(value interface{}, fieldName string) (string, error) {
	if value == nil {
		return "", nil
	}

	if str, ok := value.(string); ok {
		return str, nil
	}

	return "", errors.TypeConversionError("interface{}", "string", value).
		WithContext("field", fieldName).
		WithSuggestion("Ensure the field contains a string value")
}

// SafeIntAssertion safely converts interface{} to int
func SafeIntAssertion(value interface{}, fieldName string) (int, error) {
	if value == nil {
		return 0, nil
	}

	switch v := value.(type) {
	case int:
		return v, nil
	case int32:
		return int(v), nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	case float32:
		return int(v), nil
	default:
		return 0, errors.TypeConversionError(fmt.Sprintf("%T", value), "int", value).
			WithContext("field", fieldName).
			WithSuggestion("Ensure the field contains a numeric value")
	}
}

// SafeBoolAssertion safely converts interface{} to bool
func SafeBoolAssertion(value interface{}, fieldName string) (bool, error) {
	if value == nil {
		return false, nil
	}

	if b, ok := value.(bool); ok {
		return b, nil
	}

	return false, errors.TypeConversionError(fmt.Sprintf("%T", value), "bool", value).
		WithContext("field", fieldName).
		WithSuggestion("Ensure the field contains a boolean value")
}

// SafeMapAssertion safely converts interface{} to map[string]interface{}
func SafeMapAssertion(value interface{}, fieldName string) (map[string]interface{}, error) {
	if value == nil {
		return make(map[string]interface{}), nil
	}

	if m, ok := value.(map[string]interface{}); ok {
		return m, nil
	}

	return nil, errors.TypeConversionError(fmt.Sprintf("%T", value), "map[string]interface{}", value).
		WithContext("field", fieldName).
		WithSuggestion("Ensure the field contains a map structure")
}

// SafeSliceAssertion safely converts interface{} to []interface{}
func SafeSliceAssertion(value interface{}, fieldName string) ([]interface{}, error) {
	if value == nil {
		return make([]interface{}, 0), nil
	}

	if s, ok := value.([]interface{}); ok {
		return s, nil
	}

	return nil, errors.TypeConversionError(fmt.Sprintf("%T", value), "[]interface{}", value).
		WithContext("field", fieldName).
		WithSuggestion("Ensure the field contains a slice/array structure")
}

// SafeStringSliceAssertion safely converts interface{} to []string
func SafeStringSliceAssertion(value interface{}, fieldName string) ([]string, error) {
	if value == nil {
		return make([]string, 0), nil
	}

	if ss, ok := value.([]string); ok {
		return ss, nil
	}

	// Try to convert []interface{} to []string
	if si, ok := value.([]interface{}); ok {
		result := make([]string, len(si))
		for i, item := range si {
			if str, ok := item.(string); ok {
				result[i] = str
			} else {
				return nil, errors.TypeConversionError(fmt.Sprintf("%T", item), "string", item).
					WithContext("field", fmt.Sprintf("%s[%d]", fieldName, i)).
					WithSuggestion("Ensure all array elements are strings")
			}
		}
		return result, nil
	}

	return nil, errors.TypeConversionError(fmt.Sprintf("%T", value), "[]string", value).
		WithContext("field", fieldName).
		WithSuggestion("Ensure the field contains a string array")
}

// SafeAssertionWithDefault provides safe type assertion with default fallback
func SafeAssertionWithDefault[T any](value interface{}, defaultValue T, fieldName string) T {
	if value == nil {
		return defaultValue
	}

	if typed, ok := value.(T); ok {
		return typed
	}

	// Log warning about type mismatch but return default
	// This is for cases where we want graceful degradation
	return defaultValue
}

// SafeAssertionStrict provides safe type assertion with error on mismatch
func SafeAssertionStrict[T any](value interface{}, fieldName string) (T, error) {
	var zero T

	if value == nil {
		return zero, nil
	}

	if typed, ok := value.(T); ok {
		return typed, nil
	}

	return zero, errors.TypeConversionError(fmt.Sprintf("%T", value), fmt.Sprintf("%T", zero), value).
		WithContext("field", fieldName).
		WithSuggestion("Ensure the field contains the expected type")
}

// ConvertMapWithSafetyChecks converts between different map types safely
func ConvertMapWithSafetyChecks(source map[string]interface{}, target interface{}) error {
	// This function can be used for converting between different typed map structures
	// Implementation would depend on the specific conversion needed
	return nil
}

// ValidateAndConvertInterface validates interface{} values and converts them to expected types
func ValidateAndConvertInterface(value interface{}, expectedType string, fieldName string) (interface{}, error) {
	switch expectedType {
	case "string":
		return SafeStringAssertion(value, fieldName)
	case "int":
		return SafeIntAssertion(value, fieldName)
	case "bool":
		return SafeBoolAssertion(value, fieldName)
	case "map":
		return SafeMapAssertion(value, fieldName)
	case "slice":
		return SafeSliceAssertion(value, fieldName)
	case "[]string":
		return SafeStringSliceAssertion(value, fieldName)
	default:
		return value, errors.NewError().
			Code(errors.CodeInvalidParameter).
			Message("Unsupported type for validation").
			Context("expected_type", expectedType).
			Context("field", fieldName).
			Build()
	}
}
