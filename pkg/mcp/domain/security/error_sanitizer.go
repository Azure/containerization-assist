package security

import (
	"regexp"
)

// ErrorSanitizer provides functionality to sanitize error messages
// by removing sensitive information like secrets, paths, and credentials
type ErrorSanitizer struct {
	patterns []SanitizationPattern
}

// NewErrorSanitizer creates a new error sanitizer with default patterns
func NewErrorSanitizer() *ErrorSanitizer {
	return &ErrorSanitizer{
		patterns: getDefaultSanitizationPatterns(),
	}
}

// SanitizeErrorMessage removes sensitive information from error messages
func (es *ErrorSanitizer) SanitizeErrorMessage(message string) string {
	if message == "" {
		return ""
	}

	sanitized := message

	// Apply all sanitization patterns
	for _, pattern := range es.patterns {
		re, err := regexp.Compile(pattern.Pattern)
		if err != nil {
			continue // Skip invalid patterns
		}
		sanitized = re.ReplaceAllString(sanitized, pattern.Replacement)
	}

	// Additional sanitization for common sensitive patterns
	sanitized = es.sanitizeCommonPatterns(sanitized)

	return sanitized
}

// RemoveSensitiveData removes specific types of sensitive data
func (es *ErrorSanitizer) RemoveSensitiveData(message string, dataType string) string {
	switch dataType {
	case "paths":
		return es.sanitizePaths(message)
	case "credentials":
		return es.sanitizeCredentials(message)
	case "tokens":
		return es.sanitizeTokens(message)
	case "urls":
		return es.sanitizeURLs(message)
	default:
		return es.SanitizeErrorMessage(message)
	}
}

// sanitizeCommonPatterns applies common sanitization patterns
func (es *ErrorSanitizer) sanitizeCommonPatterns(message string) string {
	// Remove potential file paths
	pathPattern := regexp.MustCompile(`(/[a-zA-Z0-9_\-./]+)+`)
	message = pathPattern.ReplaceAllString(message, "[PATH]")

	// Remove potential IP addresses
	ipPattern := regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	message = ipPattern.ReplaceAllString(message, "[IP]")

	// Remove potential email addresses
	emailPattern := regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
	message = emailPattern.ReplaceAllString(message, "[EMAIL]")

	// Remove potential URLs
	urlPattern := regexp.MustCompile(`https?://[^\s]+`)
	message = urlPattern.ReplaceAllString(message, "[URL]")

	return message
}

// sanitizePaths removes file system paths
func (es *ErrorSanitizer) sanitizePaths(message string) string {
	patterns := []string{
		`(/[a-zA-Z0-9_\-./]+)+`,                        // Unix paths
		`[a-zA-Z]:\\[a-zA-Z0-9_\-.\\ ]+`,               // Windows paths
		`\\\\[a-zA-Z0-9_\-.\\ ]+\\[a-zA-Z0-9_\-.\\ ]+`, // UNC paths
	}

	sanitized := message
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		sanitized = re.ReplaceAllString(sanitized, "[PATH]")
	}

	return sanitized
}

// sanitizeCredentials removes potential credentials
func (es *ErrorSanitizer) sanitizeCredentials(message string) string {
	patterns := []string{
		`password\s*[:=]\s*['"]?[^'"\s]+['"]?`,
		`token\s*[:=]\s*['"]?[^'"\s]+['"]?`,
		`api[_-]?key\s*[:=]\s*['"]?[^'"\s]+['"]?`,
		`secret\s*[:=]\s*['"]?[^'"\s]+['"]?`,
		`Bearer\s+[a-zA-Z0-9_\-\.]+`,
	}

	sanitized := message
	for _, pattern := range patterns {
		re := regexp.MustCompile(`(?i)` + pattern)
		sanitized = re.ReplaceAllString(sanitized, "[CREDENTIAL]")
	}

	return sanitized
}

// sanitizeTokens removes various types of tokens
func (es *ErrorSanitizer) sanitizeTokens(message string) string {
	patterns := []string{
		`[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}`, // UUIDs
		`[a-zA-Z0-9]{20,}`, // Long alphanumeric tokens
		`eyJ[a-zA-Z0-9_\-]+\.[a-zA-Z0-9_\-]+\.[a-zA-Z0-9_\-]+`, // JWT tokens
	}

	sanitized := message
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		sanitized = re.ReplaceAllString(sanitized, "[TOKEN]")
	}

	return sanitized
}

// sanitizeURLs removes URL parameters that might contain sensitive data
func (es *ErrorSanitizer) sanitizeURLs(message string) string {
	// Remove query parameters from URLs
	urlParamPattern := regexp.MustCompile(`(\?|&)[a-zA-Z0-9_\-]+=([^&\s]+)`)
	message = urlParamPattern.ReplaceAllString(message, "$1[PARAM]=[VALUE]")

	// Remove basic auth from URLs
	basicAuthPattern := regexp.MustCompile(`(https?://)([^:]+):([^@]+)@`)
	message = basicAuthPattern.ReplaceAllString(message, "$1[USER]:[PASS]@")

	return message
}

// AddPattern adds a custom sanitization pattern
func (es *ErrorSanitizer) AddPattern(pattern SanitizationPattern) {
	es.patterns = append(es.patterns, pattern)
}

// getDefaultSanitizationPatterns returns default sanitization patterns
func getDefaultSanitizationPatterns() []SanitizationPattern {
	return []SanitizationPattern{
		{
			Pattern:     `(?i)(password|passwd|pwd)\s*[:=]\s*['"]?[^'"\s]+['"]?`,
			Replacement: "[PASSWORD]",
			Type:        "credential",
		},
		{
			Pattern:     `(?i)(token|api[_-]?key|secret)\s*[:=]\s*['"]?[^'"\s]+['"]?`,
			Replacement: "[SECRET]",
			Type:        "credential",
		},
		{
			Pattern:     `(?i)Bearer\s+[a-zA-Z0-9_\-\.]+`,
			Replacement: "Bearer [TOKEN]",
			Type:        "auth_header",
		},
		{
			Pattern:     `/home/[^/\s]+`,
			Replacement: "/home/[USER]",
			Type:        "path",
		},
		{
			Pattern:     `C:\\Users\\[^\\]+`,
			Replacement: "C:\\Users\\[USER]",
			Type:        "path",
		},
	}
}
