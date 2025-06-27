package utils

import (
	"fmt"
	"reflect"
	"strings"

	utils "github.com/Azure/container-kit/pkg/mcp/utils"
)

// BuildArgsMap converts a struct to a map[string]interface{} using reflection
// and JSON tags for key naming. This eliminates the need for repetitive
// manual argument mapping code.
func BuildArgsMap(args interface{}) (map[string]interface{}, error) {
	if args == nil {
		return nil, fmt.Errorf("args cannot be nil")
	}

	argsMap := make(map[string]interface{})

	// Get the value and type of the struct
	val := reflect.ValueOf(args)
	typ := reflect.TypeOf(args)

	// Dereference pointers
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil, fmt.Errorf("args cannot be nil pointer")
		}
		val = val.Elem()
		typ = typ.Elem()
	}

	// Ensure we have a struct
	if val.Kind() != reflect.Struct {
		return nil, fmt.Errorf("args must be a struct or pointer to struct, got %s", val.Kind())
	}

	// Iterate through all fields
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		// Skip unexported fields
		if !field.CanInterface() {
			continue
		}

		// Get the key name from JSON tag or field name
		keyName := getKeyName(fieldType)

		// Handle embedded structs (like BaseToolArgs)
		if field.Kind() == reflect.Struct && fieldType.Anonymous {
			// Recursively process embedded struct
			embeddedMap, err := BuildArgsMap(field.Interface())
			if err != nil {
				return nil, fmt.Errorf("failed to process embedded struct %s: %w", fieldType.Name, err)
			}
			// Merge embedded fields into main map
			for k, v := range embeddedMap {
				argsMap[k] = v
			}
			continue
		}

		// Add the field to the map
		argsMap[keyName] = field.Interface()
	}

	return argsMap, nil
}

// getKeyName extracts the key name from JSON tag or converts field name to snake_case
func getKeyName(field reflect.StructField) string {
	// Check for JSON tag first
	if tag := field.Tag.Get("json"); tag != "" {
		// Handle json:",omitempty" and similar cases
		if idx := strings.Index(tag, ","); idx != -1 {
			tag = tag[:idx]
		}
		if tag != "" && tag != "-" {
			return tag
		}
	}

	// Check for explicit mapkey tag for backward compatibility
	if tag := field.Tag.Get("mapkey"); tag != "" {
		return tag
	}

	// Convert field name to snake_case
	return utils.ToSnakeCase(field.Name)
}

// toSnakeCase function has been moved to utils.ToSnakeCase

// ConvertSliceToInterfaceSlice converts []T to []interface{} for generic use
func ConvertSliceToInterfaceSlice[T any](slice []T) []interface{} {
	if slice == nil {
		return nil
	}

	result := make([]interface{}, len(slice))
	for i, v := range slice {
		result[i] = v
	}
	return result
}
