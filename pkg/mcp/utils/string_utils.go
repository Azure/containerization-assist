package utils

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// StringUtils provides common string manipulation utilities
// This file consolidates duplicate string functions found across the codebase

// ToSnakeCase converts CamelCase or PascalCase to snake_case
// This is the consolidated version of multiple toSnakeCase implementations
func ToSnakeCase(s string) string {
	if s == "" {
		return ""
	}

	// Handle acronyms and consecutive uppercase letters
	var result strings.Builder
	runes := []rune(s)

	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) {
			// Check if the previous character is lowercase or if this is the start of a new word
			if unicode.IsLower(runes[i-1]) {
				result.WriteRune('_')
			} else if i < len(runes)-1 && unicode.IsLower(runes[i+1]) {
				// This handles cases like "XMLParser" -> "xml_parser"
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
			// Capitalize first letter of each part
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

// ToPascalCase converts snake_case to PascalCase (same as CamelCase)
func ToPascalCase(s string) string {
	return ToCamelCase(s)
}

// ToKebabCase converts CamelCase or snake_case to kebab-case
func ToKebabCase(s string) string {
	snake := ToSnakeCase(s)
	return strings.ReplaceAll(snake, "_", "-")
}

// NormalizeString removes extra whitespace and normalizes string
func NormalizeString(s string) string {
	// Replace multiple whitespace with single space
	re := regexp.MustCompile(`\s+`)
	normalized := re.ReplaceAllString(strings.TrimSpace(s), " ")
	return normalized
}

// TruncateString truncates a string to maxLength with optional suffix
func TruncateString(s string, maxLength int, suffix string) string {
	if len(s) <= maxLength {
		return s
	}

	if len(suffix) >= maxLength {
		return suffix[:maxLength]
	}

	return s[:maxLength-len(suffix)] + suffix
}

// ContainsAny checks if string contains any of the given substrings
func ContainsAny(s string, substrings []string) bool {
	for _, substr := range substrings {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}

// ContainsAll checks if string contains all of the given substrings
func ContainsAll(s string, substrings []string) bool {
	for _, substr := range substrings {
		if !strings.Contains(s, substr) {
			return false
		}
	}
	return true
}

// SplitAndTrim splits string by separator and trims whitespace from each part
func SplitAndTrim(s, sep string) []string {
	if s == "" {
		return []string{}
	}

	parts := strings.Split(s, sep)
	result := make([]string, 0, len(parts))

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

// ReverseString reverses a string
func ReverseString(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

// IsBlank checks if string is empty or contains only whitespace
func IsBlank(s string) bool {
	return strings.TrimSpace(s) == ""
}

// FirstNonEmpty returns the first non-empty string from the arguments
func FirstNonEmpty(strings ...string) string {
	for _, s := range strings {
		if s != "" {
			return s
		}
	}
	return ""
}

// RemoveEmptyLines removes empty lines from a multi-line string
func RemoveEmptyLines(s string) string {
	lines := strings.Split(s, "\n")
	result := make([]string, 0, len(lines))

	for _, line := range lines {
		if !IsBlank(line) {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n")
}

// IndentLines adds indentation to each line
func IndentLines(s string, indent string) string {
	if s == "" {
		return ""
	}

	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = indent + line
		}
	}

	return strings.Join(lines, "\n")
}

// EscapeForShell escapes a string for safe use in shell commands
func EscapeForShell(s string) string {
	// Simple escaping - wrap in single quotes and escape internal single quotes
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

// SlugifyString converts a string to a URL-friendly slug
func SlugifyString(s string) string {
	// Convert to lowercase
	s = strings.ToLower(s)

	// Replace non-alphanumeric characters with hyphens
	re := regexp.MustCompile(`[^a-z0-9]+`)
	s = re.ReplaceAllString(s, "-")

	// Remove leading/trailing hyphens
	s = strings.Trim(s, "-")

	return s
}

// PadString pads a string to a specific length
func PadString(s string, length int, padChar rune, leftPad bool) string {
	if len(s) >= length {
		return s
	}

	padding := strings.Repeat(string(padChar), length-len(s))
	if leftPad {
		return padding + s
	}
	return s + padding
}

// WrapText wraps text to specified line length
func WrapText(text string, lineLength int) string {
	if lineLength <= 0 {
		return text
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return text
	}

	var lines []string
	var currentLine strings.Builder

	for _, word := range words {
		// If adding this word would exceed line length, start new line
		if currentLine.Len() > 0 && currentLine.Len()+1+len(word) > lineLength {
			lines = append(lines, currentLine.String())
			currentLine.Reset()
		}

		// Add word to current line
		if currentLine.Len() > 0 {
			currentLine.WriteRune(' ')
		}
		currentLine.WriteString(word)
	}

	// Add the last line
	if currentLine.Len() > 0 {
		lines = append(lines, currentLine.String())
	}

	return strings.Join(lines, "\n")
}

// FormatBytes formats bytes into human-readable format (consolidated from common.go)
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
