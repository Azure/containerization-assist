package utils

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// ToSnakeCase converts camelCase or PascalCase to snake_case
func ToSnakeCase(input string) string {
	if input == "" {
		return ""
	}

	var result strings.Builder
	for i, r := range input {
		if i > 0 && unicode.IsUpper(r) {
			result.WriteRune('_')
		}
		result.WriteRune(unicode.ToLower(r))
	}

	return result.String()
}

// ToCamelCase converts snake_case or kebab-case to camelCase
func ToCamelCase(input string) string {
	if input == "" {
		return ""
	}

	parts := regexp.MustCompile(`[_-]+`).Split(input, -1)
	if len(parts) == 0 {
		return input
	}

	var result strings.Builder

	result.WriteString(strings.ToLower(parts[0]))

	for _, part := range parts[1:] {
		if part != "" {
			result.WriteString(strings.Title(strings.ToLower(part)))
		}
	}

	return result.String()
}

// ToPascalCase converts snake_case or kebab-case to PascalCase
func ToPascalCase(input string) string {
	if input == "" {
		return ""
	}

	parts := regexp.MustCompile(`[_-]+`).Split(input, -1)
	if len(parts) == 0 {
		return input
	}

	var result strings.Builder

	for _, part := range parts {
		if part != "" {
			result.WriteString(strings.Title(strings.ToLower(part)))
		}
	}

	return result.String()
}

// ToKebabCase converts camelCase, PascalCase, or snake_case to kebab-case
func ToKebabCase(input string) string {
	if input == "" {
		return ""
	}

	input = strings.ReplaceAll(input, "_", "-")

	var result strings.Builder
	for i, r := range input {
		if i > 0 && unicode.IsUpper(r) {
			result.WriteRune('-')
		}
		result.WriteRune(unicode.ToLower(r))
	}

	return result.String()
}

// FormatBytes formats byte count as human-readable string
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

	units := []string{"KB", "MB", "GB", "TB", "PB"}
	return fmt.Sprintf("%.1f %s", float64(bytes)/float64(div), units[exp])
}

// FormatDuration formats duration in a human-readable way
func FormatDuration(seconds float64) string {
	if seconds < 1 {
		return fmt.Sprintf("%.0fms", seconds*1000)
	} else if seconds < 60 {
		return fmt.Sprintf("%.1fs", seconds)
	} else if seconds < 3600 {
		minutes := int(seconds / 60)
		secs := int(seconds) % 60
		return fmt.Sprintf("%dm%ds", minutes, secs)
	} else {
		hours := int(seconds / 3600)
		minutes := int(seconds/60) % 60
		return fmt.Sprintf("%dh%dm", hours, minutes)
	}
}

// Truncate truncates a string to maxLength, adding ellipsis if needed
func Truncate(input string, maxLength int) string {
	if len(input) <= maxLength {
		return input
	}

	if maxLength <= 3 {
		return input[:maxLength]
	}

	return input[:maxLength-3] + "..."
}

// NormalizeWhitespace replaces multiple whitespace characters with single spaces
func NormalizeWhitespace(input string) string {
	re := regexp.MustCompile(`\s+`)
	normalized := re.ReplaceAllString(strings.TrimSpace(input), " ")
	return normalized
}

// RemoveNonAlphanumeric removes all non-alphanumeric characters
func RemoveNonAlphanumeric(input string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9]`)
	return re.ReplaceAllString(input, "")
}

// SanitizeIdentifier creates a safe identifier from arbitrary string
func SanitizeIdentifier(input string) string {
	if input == "" {
		return ""
	}

	re := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	clean := re.ReplaceAllString(input, "-")

	if len(clean) > 0 && !unicode.IsLetter(rune(clean[0])) && clean[0] != '_' {
		clean = "x" + clean
	}

	re = regexp.MustCompile(`[-_]+`)
	clean = re.ReplaceAllString(clean, "-")

	clean = strings.Trim(clean, "-_")

	return clean
}

// String slicing and manipulation utilities

// SafeSubstring safely extracts substring, preventing panic on out-of-bounds
func SafeSubstring(input string, start, length int) string {
	if start < 0 {
		start = 0
	}

	if start >= len(input) {
		return ""
	}

	end := start + length
	if end > len(input) {
		end = len(input)
	}

	return input[start:end]
}

// Reverse reverses a string
func Reverse(input string) string {
	runes := []rune(input)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

// Contains checks if a string slice contains a specific string
func Contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// ContainsIgnoreCase checks if a string slice contains a string (case-insensitive)
func ContainsIgnoreCase(slice []string, item string) bool {
	lowerItem := strings.ToLower(item)
	for _, s := range slice {
		if strings.ToLower(s) == lowerItem {
			return true
		}
	}
	return false
}

// RemoveDuplicates removes duplicate strings from a slice while preserving order
func RemoveDuplicates(slice []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(slice))

	for _, item := range slice {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}

	return result
}

// SplitAndTrim splits a string and trims whitespace from each part
func SplitAndTrim(input, separator string) []string {
	if input == "" {
		return []string{}
	}

	parts := strings.Split(input, separator)
	result := make([]string, 0, len(parts))

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

// JoinNonEmpty joins non-empty strings with a separator
func JoinNonEmpty(separator string, parts ...string) string {
	var nonEmpty []string
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			nonEmpty = append(nonEmpty, part)
		}
	}
	return strings.Join(nonEmpty, separator)
}

// Indent adds indentation to each line of a multi-line string
func Indent(input string, indent string) string {
	if input == "" {
		return ""
	}

	lines := strings.Split(input, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) != "" {
			lines[i] = indent + line
		}
	}

	return strings.Join(lines, "\n")
}

// PadLeft pads a string to a minimum width with spaces on the left
func PadLeft(input string, width int) string {
	if len(input) >= width {
		return input
	}
	return strings.Repeat(" ", width-len(input)) + input
}

// PadRight pads a string to a minimum width with spaces on the right
func PadRight(input string, width int) string {
	if len(input) >= width {
		return input
	}
	return input + strings.Repeat(" ", width-len(input))
}

// IsStringEmpty checks if a string is empty or contains only whitespace
func IsStringEmpty(input string) bool {
	return strings.TrimSpace(input) == ""
}

// IsStringNotEmpty checks if a string contains non-whitespace characters
func IsStringNotEmpty(input string) bool {
	return strings.TrimSpace(input) != ""
}
