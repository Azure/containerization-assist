package utils

import (
	"fmt"
	"strings"

	"github.com/Azure/container-copilot/pkg/genericutils"
)

// ExtractBaseImage extracts the base image from Dockerfile content
func ExtractBaseImage(dockerfileContent string) string {
	lines := strings.Split(dockerfileContent, "\n")
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "FROM ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return parts[1]
			}
		}
	}
	return "unknown"
}

// FormatBytes formats bytes into human-readable format
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// GetStringFromMap safely extracts a string value from a map
// Deprecated: Use genericutils.MapGetWithDefault[string] instead
func GetStringFromMap(m map[string]interface{}, key string) string {
	return genericutils.MapGetWithDefault[string](m, key, "")
}

// GetIntFromMap safely extracts an int value from a map
// Deprecated: Use genericutils.MapGetWithDefault[int] instead
func GetIntFromMap(m map[string]interface{}, key string) int {
	// Try direct int first
	if val, ok := genericutils.MapGet[int](m, key); ok {
		return val
	}
	// Try float64 (common in JSON)
	if val, ok := genericutils.MapGet[float64](m, key); ok {
		return int(val)
	}
	// Try int64
	if val, ok := genericutils.MapGet[int64](m, key); ok {
		return int(val)
	}
	return 0
}

// GetBoolFromMap safely extracts a bool value from a map
// Deprecated: Use genericutils.MapGetWithDefault[bool] instead
func GetBoolFromMap(m map[string]interface{}, key string) bool {
	return genericutils.MapGetWithDefault[bool](m, key, false)
}
