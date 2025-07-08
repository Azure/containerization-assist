// Package validation - Field validation utilities
// This file consolidates scattered field validation functions from across pkg/mcp
package security

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Common validation patterns (consolidated from multiple locations)
var (
	// Name patterns
	NamePattern       = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)
	LabelKeyPattern   = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)
	LabelValuePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)
	DNSLabelPattern   = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

	// Container and image patterns
	ImageNamePattern = regexp.MustCompile(`^[a-z0-9]+([._-][a-z0-9]+)*(/[a-z0-9]+([._-][a-z0-9]+)*)*$`)
	TagPattern       = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._/-]*$`)
	SessionIDPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

	// Kubernetes patterns
	NamespacePattern    = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	ResourceNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

	// Path and file patterns
	PathPattern     = regexp.MustCompile(`^[^<>:"|?*\x00-\x1f]*$`)
	FileNamePattern = regexp.MustCompile(`^[^<>:"/\\|?*\x00-\x1f]*$`)

	// Network patterns
	HostnamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9.-]*[a-zA-Z0-9]$`)

	// Security patterns for sanitization
	CredentialPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(password|passwd|pwd|secret|key|token|auth)\s*[=:]\s*["']?([^"'\s]+)["']?`),
		regexp.MustCompile(`(?i)(api_key|apikey|access_key|secret_key)\s*[=:]\s*["']?([^"'\s]+)["']?`),
		regexp.MustCompile(`(?i)(bearer\s+[a-zA-Z0-9._-]+)`),
		regexp.MustCompile(`(?i)(basic\s+[a-zA-Z0-9+/=]+)`),
	}

	SensitivePathPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)/home/[^/\s]+/\.(ssh|aws|docker|kube)`),
		regexp.MustCompile(`(?i)C:\\Users\\[^\\]+\\AppData`),
		regexp.MustCompile(`(?i)/var/lib/docker`),
		regexp.MustCompile(`(?i)/etc/(shadow|passwd|sudoers)`),
	}
)

// String validation functions (consolidated from multiple files)

// ValidateRequired checks if a string value is not empty
func ValidateRequired(value, fieldName string) *Error {
	if strings.TrimSpace(value) == "" {
		return &Error{
			Field:    fieldName,
			Message:  fmt.Sprintf("%s is required", fieldName),
			Code:     "FIELD_REQUIRED",
			Value:    value,
			Severity: SeverityHigh,
		}
	}
	return nil
}

// ValidateLength checks if a string length is within bounds
func ValidateLength(value, fieldName string, minLen, maxLen int) *Error {
	length := len(value)
	if length < minLen {
		return &Error{
			Field:      fieldName,
			Message:    fmt.Sprintf("%s must be at least %d characters", fieldName, minLen),
			Code:       "LENGTH_TOO_SHORT",
			Value:      value,
			Severity:   SeverityMedium,
			Constraint: fmt.Sprintf("min=%d", minLen),
		}
	}
	if maxLen > 0 && length > maxLen {
		return &Error{
			Field:      fieldName,
			Message:    fmt.Sprintf("%s must be at most %d characters", fieldName, maxLen),
			Code:       "LENGTH_TOO_LONG",
			Value:      value,
			Severity:   SeverityMedium,
			Constraint: fmt.Sprintf("max=%d", maxLen),
		}
	}
	return nil
}

// ValidatePattern checks if a string matches a regex pattern
func ValidatePattern(value, fieldName string, pattern *regexp.Regexp, description string) *Error {
	if !pattern.MatchString(value) {
		return &Error{
			Field:      fieldName,
			Message:    fmt.Sprintf("%s has invalid format: %s", fieldName, description),
			Code:       "INVALID_PATTERN",
			Value:      value,
			Severity:   SeverityMedium,
			Constraint: pattern.String(),
		}
	}
	return nil
}

// Kubernetes-specific validation functions

// ValidateResourceName validates Kubernetes resource names
func ValidateResourceName(name, fieldName string) *Error {
	if err := ValidateRequired(name, fieldName); err != nil {
		return err
	}

	if err := ValidateLength(name, fieldName, 1, 63); err != nil {
		return err
	}

	return ValidatePattern(name, fieldName, ResourceNamePattern, "must be a valid Kubernetes resource name")
}

// ValidateNamespace validates Kubernetes namespace names
func ValidateNamespace(namespace, fieldName string) *Error {
	if err := ValidateRequired(namespace, fieldName); err != nil {
		return err
	}

	if err := ValidateLength(namespace, fieldName, 1, 63); err != nil {
		return err
	}

	return ValidatePattern(namespace, fieldName, NamespacePattern, "must be a valid Kubernetes namespace")
}

// ValidateLabelKey validates Kubernetes label keys
func ValidateLabelKey(key, fieldName string) *Error {
	if err := ValidateLength(key, fieldName, 1, 63); err != nil {
		return err
	}

	return ValidatePattern(key, fieldName, LabelKeyPattern, "must be a valid label key")
}

// ValidateLabelValue validates Kubernetes label values
func ValidateLabelValue(value, fieldName string) *Error {
	if err := ValidateLength(value, fieldName, 0, 63); err != nil {
		return err
	}

	return ValidatePattern(value, fieldName, LabelValuePattern, "must be a valid label value")
}

// Container-specific validation functions (consolidated from build validators)

// ValidateImageReference validates Docker image references
func ValidateImageReference(image, fieldName string) *Error {
	if err := ValidateRequired(image, fieldName); err != nil {
		return err
	}

	// Split image name and tag
	parts := strings.Split(image, ":")
	imageName := parts[0]

	if err := ValidatePattern(imageName, fieldName, ImageNamePattern, "must be a valid image name"); err != nil {
		return err
	}

	// Validate tag if present
	if len(parts) > 1 {
		tag := parts[1]
		if err := ValidatePattern(tag, fieldName+".tag", TagPattern, "must be a valid image tag"); err != nil {
			return err
		}
	}

	return nil
}

// ValidateSessionID validates session identifiers
func ValidateSessionID(sessionID, fieldName string) *Error {
	if err := ValidateRequired(sessionID, fieldName); err != nil {
		return err
	}

	if err := ValidateLength(sessionID, fieldName, 1, 64); err != nil {
		return err
	}

	return ValidatePattern(sessionID, fieldName, SessionIDPattern, "must be a valid session ID")
}

// Network validation functions

// ValidatePort validates port numbers
func ValidatePort(port int, fieldName string) *Error {
	if port < 1 || port > 65535 {
		return &Error{
			Field:      fieldName,
			Message:    fmt.Sprintf("%s must be between 1 and 65535", fieldName),
			Code:       "INVALID_PORT",
			Value:      port,
			Severity:   SeverityMedium,
			Constraint: "1-65535",
		}
	}
	return nil
}

// ValidateHostname validates hostname format
func ValidateHostname(hostname, fieldName string) *Error {
	if err := ValidateRequired(hostname, fieldName); err != nil {
		return err
	}

	if err := ValidateLength(hostname, fieldName, 1, 253); err != nil {
		return err
	}

	return ValidatePattern(hostname, fieldName, HostnamePattern, "must be a valid hostname")
}

// ValidateURL validates URL format
func ValidateURL(urlStr, fieldName string) *Error {
	if err := ValidateRequired(urlStr, fieldName); err != nil {
		return err
	}

	if _, err := url.Parse(urlStr); err != nil {
		return &Error{
			Field:    fieldName,
			Message:  fmt.Sprintf("%s must be a valid URL", fieldName),
			Code:     "INVALID_URL",
			Value:    urlStr,
			Severity: SeverityMedium,
		}
	}

	return nil
}

// ValidateIPAddress validates IP address format
func ValidateIPAddress(ip, fieldName string) *Error {
	if err := ValidateRequired(ip, fieldName); err != nil {
		return err
	}

	if net.ParseIP(ip) == nil {
		return &Error{
			Field:    fieldName,
			Message:  fmt.Sprintf("%s must be a valid IP address", fieldName),
			Code:     "INVALID_IP",
			Value:    ip,
			Severity: SeverityMedium,
		}
	}

	return nil
}

// Numeric validation functions

// ValidateRange validates that a number is within a specified range
func ValidateRange(value float64, fieldName string, minVal, maxVal float64) *Error {
	if value < minVal {
		return &Error{
			Field:      fieldName,
			Message:    fmt.Sprintf("%s must be at least %g", fieldName, minVal),
			Code:       "VALUE_TOO_SMALL",
			Value:      value,
			Severity:   SeverityMedium,
			Constraint: fmt.Sprintf("min=%g", minVal),
		}
	}
	if value > maxVal {
		return &Error{
			Field:      fieldName,
			Message:    fmt.Sprintf("%s must be at most %g", fieldName, maxVal),
			Code:       "VALUE_TOO_LARGE",
			Value:      value,
			Severity:   SeverityMedium,
			Constraint: fmt.Sprintf("max=%g", maxVal),
		}
	}
	return nil
}

// ValidatePositive validates that a number is positive
func ValidatePositive(value float64, fieldName string) *Error {
	if value <= 0 {
		return &Error{
			Field:    fieldName,
			Message:  fmt.Sprintf("%s must be positive", fieldName),
			Code:     "VALUE_NOT_POSITIVE",
			Value:    value,
			Severity: SeverityMedium,
		}
	}
	return nil
}

// Time validation functions

// ValidateDuration validates time duration format
func ValidateDuration(duration, fieldName string) *Error {
	if err := ValidateRequired(duration, fieldName); err != nil {
		return err
	}

	if _, err := time.ParseDuration(duration); err != nil {
		return &Error{
			Field:    fieldName,
			Message:  fmt.Sprintf("%s must be a valid duration (e.g., '30s', '5m', '1h')", fieldName),
			Code:     "INVALID_DURATION",
			Value:    duration,
			Severity: SeverityMedium,
		}
	}

	return nil
}

// ValidateTimeout validates timeout values
func ValidateTimeout(timeout time.Duration, fieldName string, minTimeout, maxTimeout time.Duration) *Error {
	if timeout < minTimeout {
		return &Error{
			Field:      fieldName,
			Message:    fmt.Sprintf("%s must be at least %v", fieldName, minTimeout),
			Code:       "TIMEOUT_TOO_SHORT",
			Value:      timeout,
			Severity:   SeverityMedium,
			Constraint: fmt.Sprintf("min=%v", minTimeout),
		}
	}
	if maxTimeout > 0 && timeout > maxTimeout {
		return &Error{
			Field:      fieldName,
			Message:    fmt.Sprintf("%s must be at most %v", fieldName, maxTimeout),
			Code:       "TIMEOUT_TOO_LONG",
			Value:      timeout,
			Severity:   SeverityMedium,
			Constraint: fmt.Sprintf("max=%v", maxTimeout),
		}
	}
	return nil
}

// Security validation functions (consolidated from error sanitizer)

// SanitizeErrorMessage removes sensitive information from error messages
func SanitizeErrorMessage(message string) string {
	result := message

	// Remove credentials
	for _, pattern := range CredentialPatterns {
		result = pattern.ReplaceAllStringFunc(result, func(match string) string {
			parts := pattern.FindStringSubmatch(match)
			if len(parts) >= 2 {
				return strings.Replace(match, parts[len(parts)-1], "[REDACTED]", 1)
			}
			return "[REDACTED]"
		})
	}

	// Remove sensitive paths
	for _, pattern := range SensitivePathPatterns {
		result = pattern.ReplaceAllString(result, "[REDACTED_PATH]")
	}

	return result
}

// ValidateNoSensitiveData checks if a string contains sensitive information
func ValidateNoSensitiveData(value, fieldName string) *Error {
	for _, pattern := range CredentialPatterns {
		if pattern.MatchString(value) {
			return &Error{
				Field:    fieldName,
				Message:  fmt.Sprintf("%s appears to contain sensitive data", fieldName),
				Code:     "SENSITIVE_DATA_DETECTED",
				Value:    "[REDACTED]",
				Severity: SeverityHigh,
			}
		}
	}

	return nil
}

// Enum validation functions

// ValidateEnum validates that a value is one of the allowed choices
func ValidateEnum(value, fieldName string, choices []string) *Error {
	for _, choice := range choices {
		if value == choice {
			return nil
		}
	}

	return &Error{
		Field:      fieldName,
		Message:    fmt.Sprintf("%s must be one of: %s", fieldName, strings.Join(choices, ", ")),
		Code:       "INVALID_CHOICE",
		Value:      value,
		Severity:   SeverityMedium,
		Constraint: fmt.Sprintf("choices=%s", strings.Join(choices, ",")),
	}
}

// Type conversion validation functions

// ValidateAndParseInt validates and converts a string to int
func ValidateAndParseInt(value, fieldName string) (int, *Error) {
	if err := ValidateRequired(value, fieldName); err != nil {
		return 0, err
	}

	intValue, err := strconv.Atoi(value)
	if err != nil {
		return 0, &Error{
			Field:    fieldName,
			Message:  fmt.Sprintf("%s must be a valid integer", fieldName),
			Code:     "INVALID_INTEGER",
			Value:    value,
			Severity: SeverityMedium,
		}
	}

	return intValue, nil
}

// ValidateAndParseFloat validates and converts a string to float64
func ValidateAndParseFloat(value, fieldName string) (float64, *Error) {
	if err := ValidateRequired(value, fieldName); err != nil {
		return 0, err
	}

	floatValue, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, &Error{
			Field:    fieldName,
			Message:  fmt.Sprintf("%s must be a valid number", fieldName),
			Code:     "INVALID_NUMBER",
			Value:    value,
			Severity: SeverityMedium,
		}
	}

	return floatValue, nil
}

// ValidateAndParseBool validates and converts a string to bool
func ValidateAndParseBool(value, fieldName string) (bool, *Error) {
	if err := ValidateRequired(value, fieldName); err != nil {
		return false, err
	}

	boolValue, err := strconv.ParseBool(value)
	if err != nil {
		return false, &Error{
			Field:    fieldName,
			Message:  fmt.Sprintf("%s must be a valid boolean (true/false)", fieldName),
			Code:     "INVALID_BOOLEAN",
			Value:    value,
			Severity: SeverityMedium,
		}
	}

	return boolValue, nil
}
