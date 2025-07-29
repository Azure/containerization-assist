// Package sampling provides simplified validation functionality for AI-generated content
package sampling

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// Simplified validation functions - no interface abstraction needed

type securityPattern struct {
	pattern     *regexp.Regexp
	severity    Severity
	description string
	category    string
}

var (
	globalPatterns []*securityPattern
	patternsOnce   sync.Once
)

// initializeSecurityPatterns sets up global security validation patterns
func initializeSecurityPatterns() {
	patterns := []struct {
		pattern     string
		severity    Severity
		description string
		category    string
	}{
		// Critical security patterns
		{
			pattern:     `privileged:\s*true`,
			severity:    SeverityCritical,
			description: "Privileged containers pose significant security risks",
			category:    "k8s-security",
		},
		{
			pattern:     `(?i)(password|passwd|secret|key|token|api[_-]?key)\s*[:=]\s*["']?[a-zA-Z0-9+/=]{8,}`,
			severity:    SeverityCritical,
			description: "Potential secret or credential exposure",
			category:    "credential-exposure",
		},
		{
			pattern:     `curl\s+.*\|\s*sh`,
			severity:    SeverityCritical,
			description: "Downloading and executing scripts via curl|sh is extremely risky",
			category:    "unsafe-practices",
		},
		// High severity patterns
		{
			pattern:     `hostNetwork:\s*true`,
			severity:    SeverityHigh,
			description: "hostNetwork: true can expose pod to host networking",
			category:    "k8s-security",
		},
		{
			pattern:     `USER\s+root(\s|$)`,
			severity:    SeverityHigh,
			description: "Running as root user is a security risk",
			category:    "docker-security",
		},
		{
			pattern:     `RUN\s+.*sudo\s+`,
			severity:    SeverityHigh,
			description: "Using sudo in Docker containers is potentially unsafe",
			category:    "docker-security",
		},
		// Medium severity patterns
		{
			pattern:     `:latest(\s|$)`,
			severity:    SeverityMedium,
			description: "Using 'latest' tag is discouraged in production",
			category:    "docker-best-practices",
		},
		{
			pattern:     `ADD\s+https?://`,
			severity:    SeverityMedium,
			description: "Using ADD with URLs can be a security risk, prefer COPY",
			category:    "docker-security",
		},
	}

	for _, p := range patterns {
		regex, err := regexp.Compile(p.pattern)
		if err != nil {
			continue // Skip invalid patterns
		}
		globalPatterns = append(globalPatterns, &securityPattern{
			pattern:     regex,
			severity:    p.severity,
			description: p.description,
			category:    p.category,
		})
	}
}

// ValidateManifestContent validates Kubernetes manifest content
func ValidateManifestContent(content string) ValidationResult {
	patternsOnce.Do(initializeSecurityPatterns)

	result := ValidationResult{
		IsValid:       true,
		SyntaxValid:   true,
		BestPractices: true,
		Errors:        []string{},
		Warnings:      []string{},
	}

	// Check if content is empty
	if strings.TrimSpace(content) == "" {
		result.IsValid = false
		result.SyntaxValid = false
		result.Errors = append(result.Errors, "manifest content is empty")
		return result
	}

	// Basic YAML syntax validation
	var yamlContent interface{}
	if err := yaml.Unmarshal([]byte(content), &yamlContent); err != nil {
		result.IsValid = false
		result.SyntaxValid = false
		result.Errors = append(result.Errors, fmt.Sprintf("invalid YAML syntax: %v", err))
	}

	// Check for required Kubernetes fields
	if !strings.Contains(content, "apiVersion:") {
		result.IsValid = false
		result.Errors = append(result.Errors, "missing required field: apiVersion")
	}

	if !strings.Contains(content, "kind:") {
		result.IsValid = false
		result.Errors = append(result.Errors, "missing required field: kind")
	}

	// Apply security validation
	applySecurityValidation(content, &result)

	return result
}

// ValidateDockerfileContent validates Dockerfile content
func ValidateDockerfileContent(content string) ValidationResult {
	patternsOnce.Do(initializeSecurityPatterns)

	result := ValidationResult{
		IsValid:       true,
		SyntaxValid:   true,
		BestPractices: true,
		Errors:        []string{},
		Warnings:      []string{},
	}

	// Check if content is empty
	if strings.TrimSpace(content) == "" {
		result.IsValid = false
		result.SyntaxValid = false
		result.Errors = append(result.Errors, "dockerfile content is empty")
		return result
	}

	// Check for required FROM instruction
	if !strings.Contains(content, "FROM ") {
		result.IsValid = false
		result.SyntaxValid = false
		result.Errors = append(result.Errors, "dockerfile must contain FROM instruction")
	}

	// Check basic best practices
	if !strings.Contains(content, "WORKDIR") {
		result.BestPractices = false
		result.Warnings = append(result.Warnings, "consider using WORKDIR for better practice")
	}

	// Apply security validation
	applySecurityValidation(content, &result)

	return result
}

// ValidateSecurityContent validates security-related content
func ValidateSecurityContent(content string) ValidationResult {
	patternsOnce.Do(initializeSecurityPatterns)

	result := ValidationResult{
		IsValid:       true,
		SyntaxValid:   true,
		BestPractices: true,
		Errors:        []string{},
		Warnings:      []string{},
	}

	if strings.TrimSpace(content) == "" {
		result.IsValid = false
		result.Errors = append(result.Errors, "security content is empty")
		return result
	}

	// Apply security validation
	applySecurityValidation(content, &result)

	return result
}

// ValidateRepositoryContent validates repository analysis content
func ValidateRepositoryContent(content string) ValidationResult {
	patternsOnce.Do(initializeSecurityPatterns)

	result := ValidationResult{
		IsValid:       true,
		SyntaxValid:   true,
		BestPractices: true,
		Errors:        []string{},
		Warnings:      []string{},
	}

	if strings.TrimSpace(content) == "" {
		result.IsValid = false
		result.Errors = append(result.Errors, "repository content is empty")
		return result
	}

	// Apply security validation
	applySecurityValidation(content, &result)

	return result
}

// applySecurityValidation applies security pattern validation
func applySecurityValidation(content string, result *ValidationResult) {
	for _, pattern := range globalPatterns {
		if pattern.pattern.MatchString(content) {
			switch pattern.severity {
			case SeverityCritical, SeverityHigh:
				result.IsValid = false
				result.Errors = append(result.Errors, fmt.Sprintf("%s: %s", pattern.category, pattern.description))
			case SeverityMedium, SeverityLow:
				result.BestPractices = false
				result.Warnings = append(result.Warnings, fmt.Sprintf("%s: %s", pattern.category, pattern.description))
			}
		}
	}
}

// ContentSanitizer provides content sanitization (simplified)
type ContentSanitizer struct{}

// NewContentSanitizer creates a new content sanitizer
func NewContentSanitizer() *ContentSanitizer {
	return &ContentSanitizer{}
}

// SanitizeContent performs basic content sanitization
func (s *ContentSanitizer) SanitizeContent(content string) string {
	// Basic sanitization - remove common problematic patterns
	sanitized := content

	// Remove potential script injections
	sanitized = regexp.MustCompile(`<script[^>]*>.*?</script>`).ReplaceAllString(sanitized, "")

	// Remove potential command injections
	sanitized = regexp.MustCompile(`\$\([^)]+\)`).ReplaceAllString(sanitized, "")

	return sanitized
}

// ValidationMetrics provides simple validation metrics
type ValidationMetrics struct {
	TotalValidations  int
	PassedValidations int
	FailedValidations int
	mutex             sync.RWMutex
}

// RecordValidation records a validation result
func (m *ValidationMetrics) RecordValidation(passed bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.TotalValidations++
	if passed {
		m.PassedValidations++
	} else {
		m.FailedValidations++
	}
}

// GetMetrics returns current metrics
func (m *ValidationMetrics) GetMetrics() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return map[string]interface{}{
		"total_validations":  m.TotalValidations,
		"passed_validations": m.PassedValidations,
		"failed_validations": m.FailedValidations,
	}
}
