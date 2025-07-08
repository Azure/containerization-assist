// Package validation provides validation utilities for the MCP system.
//
// DEPRECATED: This entire file contains reflection-based validation that is being phased out.
// DO NOT USE THIS FILE FOR NEW CODE.
// Please use generic_validator.go for new code, which provides type-safe validation
// without reflection overhead.
//
// Migration path:
// 1. Replace ValidateOptionalFields with field-specific validators from generic_validator.go
// 2. Use FieldValidator[T] for type-safe field validation
// 3. See example_migration.go for migration examples
package validation

import (
	"fmt"
	"reflect"
	"strings"

	errors "github.com/Azure/container-kit/pkg/mcp/errors"
)

// ValidateOptionalFields validates that fields marked with omitempty are truly optional
// and provides consistent validation error messages
//
// Deprecated: Use ValidateOptionalFieldsGeneric from generic_validator.go instead.
// This function uses reflection which has performance overhead.
func ValidateOptionalFields(args interface{}) error {
	return validateFieldsRecursive(reflect.ValueOf(args), "")
}

// validateFieldsRecursive recursively validates struct fields and their tags
func validateFieldsRecursive(v reflect.Value, prefix string) error {
	// Handle pointers and interfaces
	if v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return nil
	}

	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)

		// Skip unexported fields
		if !fieldValue.CanInterface() {
			continue
		}

		// Get JSON tag
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}

		fieldName := field.Name
		if prefix != "" {
			fieldName = prefix + "." + fieldName
		}

		// Parse JSON tag
		parts := strings.Split(jsonTag, ",")
		jsonFieldName := parts[0]
		hasOmitEmpty := len(parts) > 1 && containsString(parts[1:], "omitempty")

		// Check for validation inconsistencies
		if hasOmitEmpty {
			// Field should be truly optional - check if it has required validation
			if err := checkOptionalFieldConsistency(field, fieldName, jsonFieldName); err != nil {
				return err
			}
		}

		// Recursively validate nested structs
		if fieldValue.Kind() == reflect.Struct ||
			(fieldValue.Kind() == reflect.Ptr && fieldValue.Type().Elem().Kind() == reflect.Struct) {
			if err := validateFieldsRecursive(fieldValue, fieldName); err != nil {
				return err
			}
		}

		// Handle slices of structs
		if fieldValue.Kind() == reflect.Slice && fieldValue.Type().Elem().Kind() == reflect.Struct {
			for j := 0; j < fieldValue.Len(); j++ {
				if err := validateFieldsRecursive(fieldValue.Index(j), fmt.Sprintf("%s[%d]", fieldName, j)); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// checkOptionalFieldConsistency checks if a field marked as omitempty has conflicting validation
func checkOptionalFieldConsistency(field reflect.StructField, fieldName, jsonFieldName string) error {
	// Check for jsonschema required tag (conflicts with omitempty)
	if jsonSchemaTag := field.Tag.Get("jsonschema"); jsonSchemaTag != "" {
		if strings.Contains(jsonSchemaTag, "required") {
			return fmt.Errorf("field %s has conflicting tags: omitempty in json but required in jsonschema", fieldName)
		}
	}

	// Check for validate required tag (conflicts with omitempty)
	if validateTag := field.Tag.Get("validate"); validateTag != "" {
		if strings.Contains(validateTag, "required") {
			return fmt.Errorf("field %s has conflicting tags: omitempty in json but required in validate", fieldName)
		}
	}

	return nil
}

// ValidateRequiredFields validates that required fields are present and non-empty
//
// Deprecated: Use ValidateRequiredFieldsGeneric from generic_validator.go instead.
// This function uses reflection which has performance overhead.
func ValidateRequiredFields(args interface{}) error {
	return validateRequiredFieldsRecursive(reflect.ValueOf(args), "")
}

// validateRequiredFieldsRecursive validates required fields recursively
func validateRequiredFieldsRecursive(v reflect.Value, prefix string) error {
	// Handle pointers and interfaces
	if v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return nil
	}

	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)

		// Skip unexported fields
		if !fieldValue.CanInterface() {
			continue
		}

		// Get JSON tag
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}

		fieldName := field.Name
		if prefix != "" {
			fieldName = prefix + "." + fieldName
		}

		// Parse JSON tag
		parts := strings.Split(jsonTag, ",")
		jsonFieldName := parts[0]
		hasOmitEmpty := len(parts) > 1 && containsString(parts[1:], "omitempty")

		// If field doesn't have omitempty, it's required
		if !hasOmitEmpty {
			if err := checkRequiredField(fieldValue, fieldName, jsonFieldName, field.Type); err != nil {
				return err
			}
		}

		// Recursively validate nested structs
		if fieldValue.Kind() == reflect.Struct ||
			(fieldValue.Kind() == reflect.Ptr && fieldValue.Type().Elem().Kind() == reflect.Struct) {
			if err := validateRequiredFieldsRecursive(fieldValue, fieldName); err != nil {
				return err
			}
		}
	}

	return nil
}

// checkRequiredField checks if a required field has a valid value
func checkRequiredField(fieldValue reflect.Value, fieldName, jsonFieldName string, fieldType reflect.Type) error {
	// Check if field is zero value
	if fieldValue.IsZero() {
		return fmt.Errorf("required field %s (%s) is missing or empty", fieldName, jsonFieldName)
	}

	// Additional checks for specific types
	switch fieldValue.Kind() {
	case reflect.String:
		if strings.TrimSpace(fieldValue.String()) == "" {
			return fmt.Errorf("required field %s (%s) cannot be empty string", fieldName, jsonFieldName)
		}
	case reflect.Slice, reflect.Map:
		if fieldValue.Len() == 0 {
			return fmt.Errorf("required field %s (%s) cannot be empty", fieldName, jsonFieldName)
		}
	}

	return nil
}

// containsString checks if a string slice contains a specific string
func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// NewValidationError creates a standardized validation error
// Deprecated: Use core.NewFieldError or errors.RichError instead for better error context
func NewValidationError(field, message string) error {
	return errors.NewError().Messagef("validation error for field %s: %s", field, message).Build()
}
