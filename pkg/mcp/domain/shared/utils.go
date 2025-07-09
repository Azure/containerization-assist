package shared

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors/codes"
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

// ToSnakeCase converts CamelCase or PascalCase to snake_case
func ToSnakeCase(s string) string {
	if s == "" {
		return ""
	}

	var result strings.Builder
	runes := []rune(s)

	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) {
			if unicode.IsLower(runes[i-1]) {
				result.WriteRune('_')
			} else if i < len(runes)-1 && unicode.IsLower(runes[i+1]) {
				result.WriteRune('_')
			}
		}
		result.WriteRune(unicode.ToLower(r))
	}

	return result.String()
}

// ToCamelCase converts snake_case to CamelCase
func ToCamelCase(s string) string {
	if s == "" {
		return ""
	}

	parts := strings.Split(s, "_")
	var result strings.Builder

	for _, part := range parts {
		if part != "" {
			if len(part) > 0 {
				result.WriteRune(unicode.ToUpper(rune(part[0])))
				if len(part) > 1 {
					result.WriteString(part[1:])
				}
			}
		}
	}

	return result.String()
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

// WrapError wraps an error with a message
func WrapError(err error, message string) error {
	if err == nil {
		return nil
	}
	systemErr := errors.SystemError(
		codes.SYSTEM_ERROR,
		message,
		err,
	)
	systemErr.Context["component"] = "utils_common"
	return systemErr
}

// ConsolidatedErrorContext represents typed context for errors
type ConsolidatedErrorContext struct {
	Operation string
	Resource  string
	Details   map[string]string
}

// NewError creates a new error with the given message
// Deprecated: Use rich.NewError() for typed error handling
func NewError(message string, context ...interface{}) error {
	// Maintain backward compatibility with existing map[string]interface{} usage
	// while gradually migrating to typed contexts
	systemErr := errors.SystemError(
		codes.SYSTEM_ERROR,
		message,
		nil,
	)
	systemErr.Context["component"] = "utils_common"
	return systemErr
}

// NewTypedError creates a new error with typed context
func NewTypedError(message string, context ...ConsolidatedErrorContext) error {
	// New function for typed error contexts
	systemErr := errors.SystemError(
		codes.SYSTEM_ERROR,
		message,
		nil,
	)
	systemErr.Context["component"] = "utils_common"
	return systemErr
}
