package utils

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// TypedAssertionResult represents the result of a type assertion operation
type TypedAssertionResult[T any] struct {
	Value T
	Ok    bool
	Error error
}

// SafeTypedAssertion provides type-safe conversion with proper error handling
func SafeTypedAssertion[T any](value interface{}, fieldName string) TypedAssertionResult[T] {
	if value == nil {
		var zero T
		return TypedAssertionResult[T]{
			Value: zero,
			Ok:    true,
			Error: nil,
		}
	}

	if typed, ok := value.(T); ok {
		return TypedAssertionResult[T]{
			Value: typed,
			Ok:    true,
			Error: nil,
		}
	}

	var zero T
	return TypedAssertionResult[T]{
		Value: zero,
		Ok:    false,
		Error: func() *errors.RichError {
			err := errors.TypeConversionError(
				fmt.Sprintf("%T", value),
				fmt.Sprintf("%T", zero),
				value,
			)
			err.Context["field"] = fieldName
			return err
		}(),
	}
}

// SafeTypedAssertionWithDefault provides type-safe conversion with a default value
func SafeTypedAssertionWithDefault[T any](value interface{}, defaultValue T, fieldName string) T {
	result := SafeTypedAssertion[T](value, fieldName)
	if result.Ok {
		return result.Value
	}
	return defaultValue
}

// SafeTypedAssertionOrError provides type-safe conversion that returns an error on failure
func SafeTypedAssertionOrError[T any](value interface{}, fieldName string) (T, error) {
	result := SafeTypedAssertion[T](value, fieldName)
	return result.Value, result.Error
}

// TypedStringAssertion safely converts interface{} to string with proper error handling
func TypedStringAssertion(value interface{}, fieldName string) (string, error) {
	return SafeTypedAssertionOrError[string](value, fieldName)
}

// TypedIntAssertion safely converts interface{} to int with numeric type coercion
func TypedIntAssertion(value interface{}, fieldName string) (int, error) {
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

// TypedBoolAssertion safely converts interface{} to bool
func TypedBoolAssertion(value interface{}, fieldName string) (bool, error) {
	return SafeTypedAssertionOrError[bool](value, fieldName)
}

// TypedSliceAssertion safely converts interface{} to a typed slice
func TypedSliceAssertion[T any](value interface{}, fieldName string) ([]T, error) {
	if value == nil {
		return make([]T, 0), nil
	}

	// Direct slice type conversion
	if s, ok := value.([]T); ok {
		return s, nil
	}

	// Try to convert []interface{} to []T
	if si, ok := value.([]interface{}); ok {
		result := make([]T, 0, len(si))
		for _, item := range si {
			if typed, ok := item.(T); ok {
				result = append(result, typed)
			} else {
				var zero T
				err := errors.TypeConversionError(
					fmt.Sprintf("%T", item),
					fmt.Sprintf("%T", zero),
					item,
				)
				err.Context["field"] = fmt.Sprintf("%s[%d]", fieldName, len(result))
				return nil, err
			}
		}
		return result, nil
	}

	var zero []T
	err := errors.TypeConversionError(
		fmt.Sprintf("%T", value),
		fmt.Sprintf("[]%T", *new(T)),
		value,
	)
	err.Context["field"] = fieldName
	return zero, err
}

// TypedMapAssertion safely converts interface{} to a typed map
func TypedMapAssertion[V any](value interface{}, fieldName string) (map[string]V, error) {
	if value == nil {
		return make(map[string]V), nil
	}

	// Direct map type conversion
	if m, ok := value.(map[string]V); ok {
		return m, nil
	}

	// Try to convert map[string]interface{} to map[string]V
	if mi, ok := value.(map[string]interface{}); ok {
		result := make(map[string]V)
		for k, v := range mi {
			if typed, ok := v.(V); ok {
				result[k] = typed
			} else {
				var zero V
				err := errors.TypeConversionError(
					fmt.Sprintf("%T", v),
					fmt.Sprintf("%T", zero),
					v,
				)
				err.Context["field"] = fmt.Sprintf("%s[%s]", fieldName, k)
				return nil, err
			}
		}
		return result, nil
	}

	var zero map[string]V
	err := errors.TypeConversionError(
		fmt.Sprintf("%T", value),
		fmt.Sprintf("map[string]%T", *new(V)),
		value,
	)
	err.Context["field"] = fieldName
	return zero, err
}

// TypedValidator provides validation for typed values
type TypedValidator[T any] struct {
	validator func(T) error
}

// NewTypedValidator creates a new typed validator
func NewTypedValidator[T any](validator func(T) error) *TypedValidator[T] {
	return &TypedValidator[T]{validator: validator}
}

// ValidateAndAssert validates and converts interface{} to typed value
func (tv *TypedValidator[T]) ValidateAndAssert(value interface{}, fieldName string) (T, error) {
	typedValue, err := SafeTypedAssertionOrError[T](value, fieldName)
	if err != nil {
		return typedValue, err
	}

	if tv.validator != nil {
		if err := tv.validator(typedValue); err != nil {
			return typedValue, errors.NewError().Message(fmt.Sprintf("validation failed for field %s", fieldName)).Cause(err).Build()
		}
	}

	return typedValue, nil
}

// CommonValidators provides common validation functions
type CommonValidators struct{}

// StringValidator creates a string validator with constraints
func (cv *CommonValidators) StringValidator(minLen, maxLen int, required bool) func(string) error {
	return func(s string) error {
		if required && s == "" {
			return errors.NewError().Messagef("string value is required").Build()
		}
		if len(s) < minLen {
			return errors.NewError().Messagef("string too short: %d < %d", len(s), minLen).Build()
		}
		if maxLen > 0 && len(s) > maxLen {
			return errors.NewError().Messagef("string too long: %d > %d", len(s), maxLen).Build()
		}
		return nil
	}
}

// IntValidator creates an integer validator with constraints
func (cv *CommonValidators) IntValidator(min, max int) func(int) error {
	return func(i int) error {
		if i < min {
			return errors.NewError().Messagef("value too small: %d < %d", i, min).Build()
		}
		if i > max {
			return errors.NewError().Messagef("value too large: %d > %d", i, max).Build()
		}
		return nil
	}
}

// SliceValidator creates a slice validator with constraints (generic function, not method)
func SliceValidator[T any](minLen, maxLen int, itemValidator func(T) error) func([]T) error {
	return func(s []T) error {
		if len(s) < minLen {
			return errors.NewError().Messagef("slice too short: %d < %d", len(s), minLen).Build()
		}
		if maxLen > 0 && len(s) > maxLen {
			return errors.NewError().Messagef("slice too long: %d > %d", len(s), maxLen).Build()
		}
		if itemValidator != nil {
			for i, item := range s {
				if err := itemValidator(item); err != nil {
					return errors.NewError().Message(fmt.Sprintf("item at index %d failed validation", i)).Cause(err).Build()
				}
			}
		}
		return nil
	}
}

// TypedConversionHelper provides additional type conversion utilities
type TypedConversionHelper struct{}

// ConvertStructToTyped converts a struct to typed value using reflection (generic function, not method)
func ConvertStructToTyped[T any](source interface{}) (T, error) {
	var result T

	if source == nil {
		return result, nil
	}

	sourceValue := reflect.ValueOf(source)
	resultValue := reflect.ValueOf(&result).Elem()

	if sourceValue.Type().AssignableTo(resultValue.Type()) {
		resultValue.Set(sourceValue)
		return result, nil
	}

	return result, errors.NewError().Messagef("cannot convert %T to %T", source, result).Build()
}

func ConvertMapToStruct[T any](source map[string]interface{}) (T, error) {
	var result T

	resultValue := reflect.ValueOf(&result).Elem()
	resultType := resultValue.Type()

	for i := 0; i < resultType.NumField(); i++ {
		field := resultType.Field(i)
		fieldValue := resultValue.Field(i)

		if !fieldValue.CanSet() {
			continue
		}

		// Get field name from json tag or field name
		fieldName := field.Name
		if jsonTag := field.Tag.Get("json"); jsonTag != "" && jsonTag != "-" {
			if commaIdx := strings.Index(jsonTag, ","); commaIdx != -1 {
				fieldName = jsonTag[:commaIdx]
			} else {
				fieldName = jsonTag
			}
		}

		if value, exists := source[fieldName]; exists && value != nil {
			valueReflect := reflect.ValueOf(value)
			if valueReflect.Type().AssignableTo(fieldValue.Type()) {
				fieldValue.Set(valueReflect)
			}
		}
	}

	return result, nil
}

// NewCommonValidators creates a new CommonValidators instance
func NewCommonValidators() *CommonValidators {
	return &CommonValidators{}
}

// NewTypedConversionHelper creates a new TypedConversionHelper instance
func NewTypedConversionHelper() *TypedConversionHelper {
	return &TypedConversionHelper{}
}
