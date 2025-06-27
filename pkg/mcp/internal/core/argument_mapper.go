package core

import (
	"context"
	"fmt"
	"reflect"
	"strings"
)

// BuildArgsMap converts a struct to a map[string]interface{} using reflection
// It prioritizes JSON tags and converts snake_case to camelCase for consistency
func BuildArgsMap(ctx context.Context, args interface{}) (map[string]interface{}, error) {
	if args == nil {
		return nil, fmt.Errorf("args cannot be nil")
	}

	v := reflect.ValueOf(args)
	t := reflect.TypeOf(args)

	// Handle pointer to struct
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil, fmt.Errorf("args pointer cannot be nil")
		}
		v = v.Elem()
		t = t.Elem()
	}

	// Ensure we have a struct
	if v.Kind() != reflect.Struct {
		return nil, fmt.Errorf("args must be a struct, got %T", args)
	}

	result := make(map[string]interface{})

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		// Skip unexported fields
		if !field.CanInterface() {
			continue
		}

		// Get field name from JSON tag or field name
		fieldName := getFieldName(fieldType)

		// Convert field value, handling special cases
		fieldValue := convertFieldValue(field)

		result[fieldName] = fieldValue
	}

	return result, nil
}

// getFieldName extracts the field name from JSON tag or converts field name to camelCase
func getFieldName(fieldType reflect.StructField) string {
	// Check for JSON tag first
	if tag := fieldType.Tag.Get("json"); tag != "" && tag != "-" {
		// Handle json:",omitempty" case
		if idx := strings.Index(tag, ","); idx != -1 {
			tag = tag[:idx]
		}
		if tag != "" {
			return tag
		}
	}

	// Convert field name from PascalCase to camelCase for consistency
	fieldName := fieldType.Name
	if len(fieldName) > 0 {
		return strings.ToLower(fieldName[:1]) + fieldName[1:]
	}

	return fieldName
}

// convertFieldValue converts reflect.Value to interface{} handling special cases
func convertFieldValue(field reflect.Value) interface{} {
	if !field.IsValid() {
		return nil
	}

	switch field.Kind() {
	case reflect.Slice:
		if field.IsNil() {
			return field.Interface()
		}
		return convertSliceToInterfaceSlice(field)
	case reflect.Map:
		return field.Interface()
	case reflect.Ptr:
		if field.IsNil() {
			// Return the actual nil pointer with type information
			return field.Interface()
		}
		return convertFieldValue(field.Elem())
	default:
		return field.Interface()
	}
}

// convertSliceToInterfaceSlice converts []T to []interface{} for generic handling
func convertSliceToInterfaceSlice(slice reflect.Value) []interface{} {
	if !slice.IsValid() || slice.IsNil() {
		return nil
	}

	result := make([]interface{}, slice.Len())
	for i := 0; i < slice.Len(); i++ {
		result[i] = convertFieldValue(slice.Index(i))
	}
	return result
}
