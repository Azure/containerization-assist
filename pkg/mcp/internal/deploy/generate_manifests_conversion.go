package deploy

import (
	"github.com/Azure/container-copilot/pkg/genericutils"
)

// getStringValue safely extracts a string value from a map
func getStringValue(m map[string]interface{}, key string) string {
	return genericutils.MapGetWithDefault[string](m, key, "")
}

// getIntValue safely extracts an int value from a map
func getIntValue(m map[string]interface{}, key string) int {
	// Try direct int first
	if val, ok := genericutils.MapGet[int](m, key); ok {
		return val
	}
	// Try float64 (common in JSON)
	if val, ok := genericutils.MapGet[float64](m, key); ok {
		return int(val)
	}
	return 0
}

// getBoolValue safely extracts a bool value from a map
func getBoolValue(m map[string]interface{}, key string) bool {
	return genericutils.MapGetWithDefault[bool](m, key, false)
}
