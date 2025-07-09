package commands

import (
	"fmt"
	"os"
	"strconv"
)

// ValidationError represents a validation error with context
type ValidationError struct {
	Field   string
	Message string
	Code    string `json:"code,omitempty"`
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error: %s - %s", e.Field, e.Message)
}

// Common parameter extraction functions

func getStringParam(data map[string]interface{}, key string, defaultValue string) string {
	if value, ok := data[key].(string); ok {
		return value
	}
	return defaultValue
}

func getIntParam(data map[string]interface{}, key string, defaultValue int) int {
	if value, ok := data[key].(float64); ok {
		return int(value)
	}
	if value, ok := data[key].(int); ok {
		return value
	}
	if value, ok := data[key].(string); ok {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getBoolParam(data map[string]interface{}, key string, defaultValue bool) bool {
	if value, ok := data[key].(bool); ok {
		return value
	}
	if value, ok := data[key].(string); ok {
		return value == "true" || value == "1" || value == "yes"
	}
	return defaultValue
}

func getStringSliceParam(data map[string]interface{}, key string) []string {
	if value, ok := data[key].([]string); ok {
		return value
	}
	if values, ok := data[key].([]interface{}); ok {
		result := make([]string, 0, len(values))
		for _, v := range values {
			if str, ok := v.(string); ok {
				result = append(result, str)
			}
		}
		return result
	}
	return nil
}

func getStringMapParam(data map[string]interface{}, key string) map[string]string {
	if value, ok := data[key].(map[string]interface{}); ok {
		result := make(map[string]string)
		for k, v := range value {
			if str, ok := v.(string); ok {
				result[k] = str
			}
		}
		return result
	}
	return make(map[string]string)
}

// Common utility functions

// Note: Use slices.Contains from the standard library instead of a custom contains function
// Example: slices.Contains(mySlice, item)

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// isValidImageName validates Docker image name format
func isValidImageName(name string) bool {
	// Basic validation - can be enhanced with full Docker naming rules
	if name == "" || len(name) > 255 {
		return false
	}

	// Check for invalid characters
	for _, char := range name {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || char == '.' || char == '-' ||
			char == '_' || char == '/' || char == ':') {
			return false
		}
	}

	return true
}
