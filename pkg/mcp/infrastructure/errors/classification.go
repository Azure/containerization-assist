// Package errors provides error classification utilities
package errors

import (
	"fmt"
	"strings"
	"time"
)

// Classification provides error pattern matching and classification utilities

// Common error patterns for automatic classification
var (
	// Infrastructure patterns
	ImageNotFoundPatterns = []string{
		"no such image",
		"image not found",
		"pull access denied",
		"repository does not exist",
		"failed to pull image",
	}

	ResourceNotFoundPatterns = []string{
		"not found",
		"notfound",
		"does not exist",
		"could not find",
		"no such resource",
	}

	PermissionDeniedPatterns = []string{
		"permission denied",
		"access denied",
		"forbidden",
		"unauthorized",
		"insufficient permissions",
		"access forbidden",
	}

	NetworkErrorPatterns = []string{
		"connection refused",
		"connection timeout",
		"network unreachable",
		"no route to host",
		"timeout",
		"dial tcp",
		"i/o timeout",
	}

	ValidationErrorPatterns = []string{
		"validation failed",
		"invalid",
		"malformed",
		"bad format",
		"parse error",
		"syntax error",
	}

	SecurityErrorPatterns = []string{
		"security violation",
		"privilege escalation",
		"unsafe operation",
		"potential security risk",
		"blocked by security policy",
	}
)

// ClassifyError automatically classifies an error based on its message and type
func ClassifyError(err error) (ErrorCategory, ErrorSeverity) {
	if err == nil {
		return CategoryInfrastructure, SeverityInfo
	}

	// If it's already a structured error, return its classification
	if structErr, ok := err.(*StructuredError); ok {
		return structErr.Category, structErr.Severity
	}

	errMsg := strings.ToLower(err.Error())

	// Check for specific patterns
	if matchesAny(errMsg, SecurityErrorPatterns) {
		return CategorySecurity, SeverityCritical
	}

	if matchesAny(errMsg, NetworkErrorPatterns) {
		return CategoryNetwork, SeverityHigh
	}

	if matchesAny(errMsg, ValidationErrorPatterns) {
		return CategoryValidation, SeverityMedium
	}

	if matchesAny(errMsg, ImageNotFoundPatterns) {
		return CategoryDocker, SeverityMedium
	}

	if matchesAny(errMsg, ResourceNotFoundPatterns) {
		return CategoryKubernetes, SeverityMedium
	}

	if matchesAny(errMsg, PermissionDeniedPatterns) {
		return CategorySecurity, SeverityHigh
	}

	// Default classification
	return CategoryInfrastructure, SeverityMedium
}

// IsRecoverableError determines if an error can be automatically recovered
func IsRecoverableError(err error) bool {
	if err == nil {
		return true
	}

	// If it's a structured error, use its recoverable flag
	if structErr, ok := err.(*StructuredError); ok {
		return structErr.Recoverable
	}

	errMsg := strings.ToLower(err.Error())

	// Security errors are generally not recoverable
	if matchesAny(errMsg, SecurityErrorPatterns) {
		return false
	}

	// Permission errors might be recoverable with different credentials
	if matchesAny(errMsg, PermissionDeniedPatterns) {
		return false
	}

	// Network errors are usually recoverable with retry
	if matchesAny(errMsg, NetworkErrorPatterns) {
		return true
	}

	// Validation errors are usually not recoverable without input changes
	if matchesAny(errMsg, ValidationErrorPatterns) {
		return false
	}

	// Resource not found might be recoverable if resource is created
	if matchesAny(errMsg, ResourceNotFoundPatterns) {
		return true
	}

	// Default to recoverable
	return true
}

// GetRetryDelay calculates appropriate retry delay based on error type
func GetRetryDelay(err error, attempt int) time.Duration {
	if err == nil {
		return 0
	}

	// If it's a structured error with retry delay, use it
	if structErr, ok := err.(*StructuredError); ok && structErr.RetryAfter != nil {
		return *structErr.RetryAfter
	}

	category, _ := ClassifyError(err)

	baseDelay := time.Second

	switch category {
	case CategoryNetwork:
		baseDelay = time.Second * 2
	case CategoryDatabase:
		baseDelay = time.Second * 3
	case CategoryAI:
		baseDelay = time.Second * 10
	case CategoryDocker:
		baseDelay = time.Second * 5
	case CategoryKubernetes:
		baseDelay = time.Second * 3
	default:
		baseDelay = time.Second * 2
	}

	// Exponential backoff with jitter
	delay := time.Duration(float64(baseDelay) * (1.5 * float64(attempt)))

	// Cap at 30 seconds
	if delay > time.Second*30 {
		delay = time.Second * 30
	}

	return delay
}

// Error analysis functions

// IsImageNotFound checks if error indicates Docker image not found
func IsImageNotFound(err error) bool {
	if err == nil {
		return false
	}

	// Check structured error category
	if structErr, ok := err.(*StructuredError); ok {
		return structErr.Category == CategoryDocker &&
			(strings.Contains(strings.ToLower(structErr.Message), "image not found") ||
				strings.Contains(strings.ToLower(structErr.Message), "no such image"))
	}

	return matchesAny(strings.ToLower(err.Error()), ImageNotFoundPatterns)
}

// IsResourceNotFound checks if error indicates Kubernetes resource not found
func IsResourceNotFound(err error) bool {
	if err == nil {
		return false
	}

	// Check structured error category
	if structErr, ok := err.(*StructuredError); ok {
		return structErr.Category == CategoryKubernetes &&
			strings.Contains(strings.ToLower(structErr.Message), "not found")
	}

	return matchesAny(strings.ToLower(err.Error()), ResourceNotFoundPatterns)
}

// IsPermissionDenied checks if error indicates permission denied
func IsPermissionDenied(err error) bool {
	if err == nil {
		return false
	}

	// Check structured error category
	if structErr, ok := err.(*StructuredError); ok {
		return structErr.Category == CategorySecurity &&
			(strings.Contains(strings.ToLower(structErr.Message), "permission") ||
				strings.Contains(strings.ToLower(structErr.Message), "forbidden"))
	}

	return matchesAny(strings.ToLower(err.Error()), PermissionDeniedPatterns)
}

// IsNetworkError checks if error is network-related
func IsNetworkError(err error) bool {
	if err == nil {
		return false
	}

	// Check structured error category
	if structErr, ok := err.(*StructuredError); ok {
		return structErr.Category == CategoryNetwork
	}

	return matchesAny(strings.ToLower(err.Error()), NetworkErrorPatterns)
}

// IsValidationError checks if error is validation-related
func IsValidationError(err error) bool {
	if err == nil {
		return false
	}

	// Check structured error category
	if structErr, ok := err.(*StructuredError); ok {
		return structErr.Category == CategoryValidation
	}

	return matchesAny(strings.ToLower(err.Error()), ValidationErrorPatterns)
}

// IsSecurityError checks if error is security-related
func IsSecurityError(err error) bool {
	if err == nil {
		return false
	}

	// Check structured error category
	if structErr, ok := err.(*StructuredError); ok {
		return structErr.Category == CategorySecurity
	}

	return matchesAny(strings.ToLower(err.Error()), SecurityErrorPatterns)
}

// IsCritical checks if error is critical severity
func IsCritical(err error) bool {
	if err == nil {
		return false
	}

	if structErr, ok := err.(*StructuredError); ok {
		return structErr.Severity == SeverityCritical
	}

	_, severity := ClassifyError(err)
	return severity == SeverityCritical
}

// ErrorSummary provides a summary of error characteristics
type ErrorSummary struct {
	Category    ErrorCategory `json:"category"`
	Severity    ErrorSeverity `json:"severity"`
	Recoverable bool          `json:"recoverable"`
	RetryDelay  time.Duration `json:"retry_delay"`
	Patterns    []string      `json:"matched_patterns"`
}

// SummarizeError provides a comprehensive analysis of an error
func SummarizeError(err error, attempt int) *ErrorSummary {
	if err == nil {
		return &ErrorSummary{
			Category:    CategoryInfrastructure,
			Severity:    SeverityInfo,
			Recoverable: true,
			RetryDelay:  0,
			Patterns:    []string{},
		}
	}

	category, severity := ClassifyError(err)
	recoverable := IsRecoverableError(err)
	retryDelay := GetRetryDelay(err, attempt)

	// Find matched patterns
	patterns := findMatchedPatterns(err)

	return &ErrorSummary{
		Category:    category,
		Severity:    severity,
		Recoverable: recoverable,
		RetryDelay:  retryDelay,
		Patterns:    patterns,
	}
}

// Helper functions

// matchesAny checks if a string matches any pattern in a list
func matchesAny(s string, patterns []string) bool {
	for _, pattern := range patterns {
		if strings.Contains(s, pattern) {
			return true
		}
	}
	return false
}

// findMatchedPatterns returns all patterns that match an error
func findMatchedPatterns(err error) []string {
	if err == nil {
		return []string{}
	}

	errMsg := strings.ToLower(err.Error())
	var matched []string

	allPatterns := map[string][]string{
		"image_not_found":    ImageNotFoundPatterns,
		"resource_not_found": ResourceNotFoundPatterns,
		"permission_denied":  PermissionDeniedPatterns,
		"network_error":      NetworkErrorPatterns,
		"validation_error":   ValidationErrorPatterns,
		"security_error":     SecurityErrorPatterns,
	}

	for category, patterns := range allPatterns {
		if matchesAny(errMsg, patterns) {
			matched = append(matched, category)
		}
	}

	return matched
}

// Format returns a human-readable error description
func Format(err error) string {
	if err == nil {
		return "no error"
	}

	if structErr, ok := err.(*StructuredError); ok {
		return fmt.Sprintf("[%s] %s", structErr.Category, structErr.Message)
	}

	category, _ := ClassifyError(err)
	return fmt.Sprintf("[%s] %s", category, err.Error())
}
