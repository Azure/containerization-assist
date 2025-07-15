// Package sampling provides validation functionality for AI-generated content
package sampling

import (
	"fmt"
	"regexp"
	"strings"

	"sync"

	"gopkg.in/yaml.v3"
)

// ContentValidator provides validation for AI-generated content
type ContentValidator interface {
	ValidateManifestContent(content string) ValidationResult
	ValidateDockerfileContent(content string) ValidationResult
	ValidateSecurityContent(content string) ValidationResult
	ValidateRepositoryContent(content string) ValidationResult
}

// DefaultValidator implements ContentValidator with comprehensive validation rules
type DefaultValidator struct {
	// Security patterns to detect and block
	securityPatterns []*SecurityPattern
}

// SecurityPattern represents a security rule for content validation
type SecurityPattern struct {
	Pattern     *regexp.Regexp
	Severity    Severity
	Description string
	Category    string
}

// NewDefaultValidator creates a new validator with default security rules
func NewDefaultValidator() *DefaultValidator {
	validator := &DefaultValidator{
		securityPatterns: []*SecurityPattern{},
	}
	validator.initializeSecurityPatterns()
	return validator
}

// initializeSecurityPatterns sets up security validation patterns
func (v *DefaultValidator) initializeSecurityPatterns() {
	patterns := []struct {
		pattern     string
		severity    Severity
		description string
		category    string
	}{
		// Docker security patterns
		{
			pattern:     `FROM\s+[^:\s]+:latest`,
			severity:    SeverityMedium,
			description: "Using 'latest' tag is discouraged in production",
			category:    "docker-best-practices",
		},
		{
			pattern:     `RUN\s+.*sudo\s+`,
			severity:    SeverityHigh,
			description: "Using sudo in Docker containers is potentially unsafe",
			category:    "docker-security",
		},
		{
			pattern:     `USER\s+root(\s|$)`,
			severity:    SeverityHigh,
			description: "Running as root user is a security risk",
			category:    "docker-security",
		},
		{
			pattern:     `ADD\s+https?://`,
			severity:    SeverityMedium,
			description: "Using ADD with URLs can be a security risk, prefer COPY",
			category:    "docker-security",
		},

		// Kubernetes security patterns
		{
			pattern:     `privileged:\s*true`,
			severity:    SeverityCritical,
			description: "Privileged containers pose significant security risks",
			category:    "k8s-security",
		},
		{
			pattern:     `hostNetwork:\s*true`,
			severity:    SeverityHigh,
			description: "hostNetwork: true can expose pod to host networking",
			category:    "k8s-security",
		},
		{
			pattern:     `allowPrivilegeEscalation:\s*true`,
			severity:    SeverityHigh,
			description: "Allowing privilege escalation is a security risk",
			category:    "k8s-security",
		},
		{
			pattern:     `runAsRoot:\s*true`,
			severity:    SeverityHigh,
			description: "Running as root is discouraged",
			category:    "k8s-security",
		},

		// Secret exposure patterns
		{
			pattern:     `(?i)(password|passwd|secret|key|token|api[_-]?key)\s*[:=]\s*["']?[a-zA-Z0-9+/=]{8,}`,
			severity:    SeverityCritical,
			description: "Potential secret or credential exposure",
			category:    "credential-exposure",
		},
		{
			pattern:     `(?i)aws[_-]?(access[_-]?key|secret)[_-]?(id|key)?\s*[:=]\s*["']?[A-Z0-9]{16,}`,
			severity:    SeverityCritical,
			description: "AWS credentials exposure",
			category:    "credential-exposure",
		},

		// General security patterns
		{
			pattern:     `curl\s+.*\|\s*sh`,
			severity:    SeverityCritical,
			description: "Downloading and executing scripts via curl|sh is extremely risky",
			category:    "unsafe-practices",
		},
		{
			pattern:     `wget\s+.*\|\s*sh`,
			severity:    SeverityCritical,
			description: "Downloading and executing scripts via wget|sh is extremely risky",
			category:    "unsafe-practices",
		},
		{
			pattern:     `chmod\s+777`,
			severity:    SeverityHigh,
			description: "Setting permissions to 777 is insecure",
			category:    "permissions",
		},
	}

	for _, p := range patterns {
		regex, err := regexp.Compile(p.pattern)
		if err != nil {
			continue // Skip invalid patterns
		}
		v.securityPatterns = append(v.securityPatterns, &SecurityPattern{
			Pattern:     regex,
			Severity:    p.severity,
			Description: p.description,
			Category:    p.category,
		})
	}
}

// ValidateManifestContent validates Kubernetes manifest content
func (v *DefaultValidator) ValidateManifestContent(content string) ValidationResult {
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

	if !strings.Contains(content, "metadata:") {
		result.IsValid = false
		result.Errors = append(result.Errors, "missing required field: metadata")
	}

	// Check for best practices
	if !strings.Contains(content, "resources:") {
		result.BestPractices = false
		result.Warnings = append(result.Warnings, "no resource limits specified (best practice)")
	}

	if !strings.Contains(content, "livenessProbe:") && !strings.Contains(content, "readinessProbe:") {
		result.BestPractices = false
		result.Warnings = append(result.Warnings, "no health checks configured (best practice)")
	}

	// Apply security validation
	v.applySecurityValidation(content, &result)

	return result
}

// ValidateDockerfileContent validates Dockerfile content
func (v *DefaultValidator) ValidateDockerfileContent(content string) ValidationResult {
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
		result.BestPractices = false
		result.Errors = append(result.Errors, "dockerfile content is empty")
		return result
	}

	lines := strings.Split(content, "\n")

	// Check for required FROM instruction
	hasFrom := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "FROM ") {
			hasFrom = true
			break
		}
	}

	if !hasFrom {
		result.IsValid = false
		result.SyntaxValid = false
		result.Errors = append(result.Errors, "dockerfile must start with FROM instruction")
	}

	// Check for best practices
	if !v.containsInstruction(content, "WORKDIR") {
		result.BestPractices = false
		result.Warnings = append(result.Warnings, "no WORKDIR specified (best practice)")
	}

	if !v.containsInstruction(content, "USER") {
		result.BestPractices = false
		result.Warnings = append(result.Warnings, "no non-root USER specified")
	}

	if !v.containsInstruction(content, "HEALTHCHECK") {
		result.BestPractices = false
		result.Warnings = append(result.Warnings, "no HEALTHCHECK specified (best practice)")
	}

	// Check for multi-stage build indicators
	fromCount := 0
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "FROM ") {
			fromCount++
		}
	}
	if fromCount == 1 {
		result.BestPractices = false
		result.Warnings = append(result.Warnings, "consider using multi-stage build for smaller images")
	}

	// Apply security validation
	v.applySecurityValidation(content, &result)

	return result
}

// ValidateSecurityContent validates security analysis content
func (v *DefaultValidator) ValidateSecurityContent(content string) ValidationResult {
	result := ValidationResult{
		IsValid:       true,
		SyntaxValid:   true,
		BestPractices: true,
		Errors:        []string{},
		Warnings:      []string{},
	}

	if strings.TrimSpace(content) == "" {
		result.IsValid = false
		result.Errors = append(result.Errors, "security analysis content is empty")
		return result
	}

	// Check for expected security analysis sections
	expectedSections := []string{"vulnerability", "risk", "remediation", "recommendation"}
	contentLower := strings.ToLower(content)

	foundSections := 0
	for _, section := range expectedSections {
		if strings.Contains(contentLower, section) {
			foundSections++
		}
	}

	if foundSections == 0 {
		result.IsValid = false
		result.Errors = append(result.Errors, "security analysis should contain vulnerability, risk, or remediation information")
	}

	// Check for actionable content
	if !strings.Contains(contentLower, "fix") && !strings.Contains(contentLower, "update") &&
		!strings.Contains(contentLower, "upgrade") && !strings.Contains(contentLower, "patch") {
		result.BestPractices = false
		result.Warnings = append(result.Warnings, "security analysis should provide actionable remediation steps")
	}

	return result
}

// ValidateRepositoryContent validates repository analysis content
func (v *DefaultValidator) ValidateRepositoryContent(content string) ValidationResult {
	result := ValidationResult{
		IsValid:       true,
		SyntaxValid:   true,
		BestPractices: true,
		Errors:        []string{},
		Warnings:      []string{},
	}

	if strings.TrimSpace(content) == "" {
		result.IsValid = false
		result.Errors = append(result.Errors, "repository analysis content is empty")
		return result
	}

	contentLower := strings.ToLower(content)

	// Check for essential analysis components
	if !strings.Contains(contentLower, "language") {
		result.IsValid = false
		result.Errors = append(result.Errors, "repository analysis must identify programming language")
	}

	// Check for useful analysis components
	analysisComponents := []string{"framework", "dependencies", "build", "port"}
	foundComponents := 0
	for _, component := range analysisComponents {
		if strings.Contains(contentLower, component) {
			foundComponents++
		}
	}

	if foundComponents < 2 {
		result.BestPractices = false
		result.Warnings = append(result.Warnings, "repository analysis should identify framework, dependencies, build tools, or ports")
	}

	return result
}

// applySecurityValidation applies security pattern matching to content
func (v *DefaultValidator) applySecurityValidation(content string, result *ValidationResult) {
	for _, pattern := range v.securityPatterns {
		if pattern.Pattern.MatchString(content) {
			switch pattern.Severity {
			case SeverityCritical, SeverityHigh:
				result.IsValid = false
				result.Errors = append(result.Errors,
					fmt.Sprintf("SECURITY: %s", pattern.Description))
			case SeverityMedium, SeverityLow:
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("SECURITY: %s", pattern.Description))
			}
		}
	}
}

// containsInstruction checks if a Dockerfile contains a specific instruction
func (v *DefaultValidator) containsInstruction(content, instruction string) bool {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, instruction+" ") || strings.HasPrefix(trimmed, instruction+"\t") {
			return true
		}
	}
	return false
}

// EnhancedValidationConfig provides configuration for advanced validation
type EnhancedValidationConfig struct {
	EnableSecurityScan  bool
	EnableBestPractices bool
	EnableSyntaxCheck   bool
	MaxContentLength    int
	AllowedImageSources []string
	BlockedInstructions []string
	RequiredLabels      []string
}

// NewEnhancedValidationConfig creates default enhanced validation config
func NewEnhancedValidationConfig() EnhancedValidationConfig {
	return EnhancedValidationConfig{
		EnableSecurityScan:  true,
		EnableBestPractices: true,
		EnableSyntaxCheck:   true,
		MaxContentLength:    100000, // 100KB limit
		AllowedImageSources: []string{
			"docker.io",
			"gcr.io",
			"quay.io",
			"registry.k8s.io",
		},
		BlockedInstructions: []string{
			"--privileged",
			"--cap-add=ALL",
		},
		RequiredLabels: []string{
			"app",
		},
	}
}

// EnhancedValidator provides configurable validation with additional checks
type EnhancedValidator struct {
	*DefaultValidator
	config EnhancedValidationConfig
}

// NewEnhancedValidator creates a new enhanced validator
func NewEnhancedValidator(config EnhancedValidationConfig) *EnhancedValidator {
	return &EnhancedValidator{
		DefaultValidator: NewDefaultValidator(),
		config:           config,
	}
}

// ValidateManifestContent provides enhanced manifest validation
func (v *EnhancedValidator) ValidateManifestContent(content string) ValidationResult {
	// Start with default validation
	result := v.DefaultValidator.ValidateManifestContent(content)

	if !v.config.EnableSecurityScan {
		// Remove security-related errors if disabled
		filteredErrors := []string{}
		for _, err := range result.Errors {
			if !strings.Contains(err, "SECURITY:") {
				filteredErrors = append(filteredErrors, err)
			}
		}
		result.Errors = filteredErrors
	}

	// Check content length
	if len(content) > v.config.MaxContentLength {
		result.IsValid = false
		result.Errors = append(result.Errors,
			fmt.Sprintf("content exceeds maximum length of %d characters", v.config.MaxContentLength))
	}

	// Check for required labels in metadata.labels section
	if strings.Contains(content, "metadata:") && strings.Contains(content, "labels:") {
		// Extract labels section
		labelSection := extractLabelSection(content)
		for _, label := range v.config.RequiredLabels {
			if !strings.Contains(labelSection, fmt.Sprintf("%s:", label)) {
				result.BestPractices = false
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("missing recommended label: %s", label))
			}
		}
	} else if len(v.config.RequiredLabels) > 0 {
		// No labels section at all
		result.BestPractices = false
		for _, label := range v.config.RequiredLabels {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("missing recommended label: %s", label))
		}
	}

	// Check image sources for containers
	if strings.Contains(content, "image:") && !v.containsAllowedImageSource(content, v.config.AllowedImageSources) {
		result.Warnings = append(result.Warnings,
			"image source not in allowed list, verify registry security")
	}

	return result
}

// extractLabelSection extracts the labels section from YAML content
func extractLabelSection(content string) string {
	lines := strings.Split(content, "\n")
	inLabels := false
	labelSection := ""
	indentLevel := -1

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		currentIndent := len(line) - len(strings.TrimLeft(line, " "))

		if trimmed == "labels:" {
			inLabels = true
			indentLevel = currentIndent
			continue
		}

		if inLabels {
			// Check if we've left the labels section
			if currentIndent <= indentLevel && trimmed != "" {
				break
			}
			if currentIndent > indentLevel {
				labelSection += line + "\n"
			}
		}
	}

	return labelSection
}

// containsAllowedImageSource checks if images use allowed registries
func (v *EnhancedValidator) containsAllowedImageSource(content string, allowedSources []string) bool {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.Contains(line, "image:") {
			for _, source := range allowedSources {
				if strings.Contains(line, source) {
					return true
				}
			}
		}
	}
	return false
}

// ValidateDockerfileContent provides enhanced Dockerfile validation
func (v *EnhancedValidator) ValidateDockerfileContent(content string) ValidationResult {
	// Start with default validation
	result := v.DefaultValidator.ValidateDockerfileContent(content)

	// Check for blocked instructions
	for _, blocked := range v.config.BlockedInstructions {
		if strings.Contains(content, blocked) {
			result.IsValid = false
			result.Errors = append(result.Errors,
				fmt.Sprintf("blocked instruction detected: %s", blocked))
		}
	}

	// Validate image sources
	if !v.containsAllowedDockerImageSource(content, v.config.AllowedImageSources) {
		result.Warnings = append(result.Warnings,
			"base image not from allowed registry sources")
	}

	return result
}

// containsAllowedDockerImageSource checks if FROM instructions use allowed registries
func (v *EnhancedValidator) containsAllowedDockerImageSource(content string, allowedSources []string) bool {
	lines := strings.Split(content, "\n")
	hasFrom := false
	allFromAllowed := true

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "FROM ") {
			hasFrom = true
			imageName := strings.TrimPrefix(line, "FROM ")
			imageName = strings.Split(imageName, " ")[0] // Remove any additional arguments

			// Skip scratch and stage references
			if imageName == "scratch" || strings.HasPrefix(imageName, "--") {
				continue
			}

			// If no registry specified, assume docker.io
			if !strings.Contains(imageName, "/") || (!strings.Contains(strings.Split(imageName, "/")[0], ".") && !strings.Contains(strings.Split(imageName, "/")[0], ":")) {
				imageName = "docker.io/" + imageName
			}

			isAllowed := false
			for _, source := range allowedSources {
				if strings.HasPrefix(imageName, source) {
					isAllowed = true
					break
				}
			}

			if !isAllowed {
				allFromAllowed = false
				break
			}
		}
	}

	return !hasFrom || allFromAllowed
}

// ContentSanitizer provides content sanitization capabilities
type ContentSanitizer struct {
	validator ContentValidator
}

// NewContentSanitizer creates a new content sanitizer
func NewContentSanitizer(validator ContentValidator) *ContentSanitizer {
	return &ContentSanitizer{
		validator: validator,
	}
}

// SanitizeAndValidate sanitizes content and performs validation
func (s *ContentSanitizer) SanitizeAndValidate(content string, contentType string) (string, ValidationResult) {
	// Basic sanitization
	sanitized := s.sanitizeContent(content)

	// Validate based on content type
	var result ValidationResult
	switch contentType {
	case "manifest", "kubernetes":
		result = s.validator.ValidateManifestContent(sanitized)
	case "dockerfile", "docker":
		result = s.validator.ValidateDockerfileContent(sanitized)
	case "security":
		result = s.validator.ValidateSecurityContent(sanitized)
	case "repository":
		result = s.validator.ValidateRepositoryContent(sanitized)
	default:
		result = ValidationResult{
			IsValid: false,
			Errors:  []string{"unknown content type"},
		}
	}

	return sanitized, result
}

// sanitizeContent performs basic content sanitization
func (s *ContentSanitizer) sanitizeContent(content string) string {
	// Remove null bytes
	content = strings.ReplaceAll(content, "\x00", "")

	// Normalize line endings
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")

	// Remove excessive whitespace
	lines := strings.Split(content, "\n")
	var cleanLines []string
	for _, line := range lines {
		// Remove trailing whitespace but preserve leading whitespace for indentation
		cleanLine := strings.TrimRight(line, " \t")
		cleanLines = append(cleanLines, cleanLine)
	}

	// Join lines and trim final newline if present
	result := strings.Join(cleanLines, "\n")
	return strings.TrimSuffix(result, "\n")
}

// ValidationMetrics tracks validation statistics
type ValidationMetrics struct {
	mu                    sync.RWMutex
	totalValidations      int64
	successfulValidations int64
	failedValidations     int64
	securityIssuesFound   int64
	bestPracticeWarnings  int64
}

// NewValidationMetrics creates new validation metrics
func NewValidationMetrics() *ValidationMetrics {
	return &ValidationMetrics{}
}

// RecordValidation records a validation attempt
func (m *ValidationMetrics) RecordValidation(result ValidationResult) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.totalValidations++

	if result.IsValid {
		m.successfulValidations++
	} else {
		m.failedValidations++
	}

	// Count security issues
	for _, err := range result.Errors {
		if strings.Contains(err, "SECURITY:") {
			m.securityIssuesFound++
		}
	}

	// Count best practice warnings
	for _, warning := range result.Warnings {
		if strings.Contains(warning, "best practice") || strings.Contains(warning, "SECURITY:") {
			m.bestPracticeWarnings++
		}
	}
}

// GetSuccessRate returns the validation success rate
func (m *ValidationMetrics) GetSuccessRate() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.totalValidations == 0 {
		return 0.0
	}
	return float64(m.successfulValidations) / float64(m.totalValidations)
}

// GetMetrics returns current metrics as a map
func (m *ValidationMetrics) GetMetrics() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Calculate success rate inline to avoid nested lock acquisition
	var successRate float64
	if m.totalValidations > 0 {
		successRate = float64(m.successfulValidations) / float64(m.totalValidations)
	}

	return map[string]interface{}{
		"total_validations":      m.totalValidations,
		"successful_validations": m.successfulValidations,
		"failed_validations":     m.failedValidations,
		"security_issues_found":  m.securityIssuesFound,
		"best_practice_warnings": m.bestPracticeWarnings,
		"success_rate":           successRate,
	}
}
