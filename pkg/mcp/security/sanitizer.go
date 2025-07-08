// Package validation - Data sanitization utilities
// This file consolidates security sanitization functionality from across pkg/mcp
package validation

import (
	"fmt"
	"regexp"
	"strings"
)

// Sanitizer provides data sanitization capabilities
type Sanitizer struct {
	credentialPatterns    []*regexp.Regexp
	sensitivePathPatterns []*regexp.Regexp
	customPatterns        []*SanitizationRule
	config                SanitizerConfig
}

// SanitizerConfig configures sanitization behavior
type SanitizerConfig struct {
	// EnableCredentialSanitization controls credential removal
	EnableCredentialSanitization bool `json:"enable_credential_sanitization"`

	// EnablePathSanitization controls sensitive path removal
	EnablePathSanitization bool `json:"enable_path_sanitization"`

	// EnableCustomRules controls custom pattern sanitization
	EnableCustomRules bool `json:"enable_custom_rules"`

	// RedactionText is the text used to replace sensitive data
	RedactionText string `json:"redaction_text"`

	// PathRedactionText is the text used to replace sensitive paths
	PathRedactionText string `json:"path_redaction_text"`

	// CaseSensitive controls whether pattern matching is case sensitive
	CaseSensitive bool `json:"case_sensitive"`
}

// SanitizationRule represents a custom sanitization rule
type SanitizationRule struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Pattern     *regexp.Regexp `json:"-"`
	PatternStr  string         `json:"pattern"`
	Replacement string         `json:"replacement"`
	Enabled     bool           `json:"enabled"`
}

// NewSanitizer creates a new sanitizer with default configuration
func NewSanitizer() *Sanitizer {
	config := SanitizerConfig{
		EnableCredentialSanitization: true,
		EnablePathSanitization:       true,
		EnableCustomRules:            true,
		RedactionText:                "[REDACTED]",
		PathRedactionText:            "[REDACTED_PATH]",
		CaseSensitive:                false,
	}

	return NewSanitizerWithConfig(config)
}

// NewSanitizerWithConfig creates a sanitizer with custom configuration
func NewSanitizerWithConfig(config SanitizerConfig) *Sanitizer {
	s := &Sanitizer{
		config:         config,
		customPatterns: make([]*SanitizationRule, 0),
	}

	s.initializeDefaultPatterns()
	return s
}

// initializeDefaultPatterns sets up the default sanitization patterns
func (s *Sanitizer) initializeDefaultPatterns() {
	flags := ""
	if !s.config.CaseSensitive {
		flags = "(?i)"
	}

	// Credential patterns (consolidated from security/error_sanitizer.go)
	credentialPatterns := []string{
		flags + `(password|passwd|pwd|secret|key|token|auth)\s*[=:]\s*["']?([^"'\s\n\r]+)["']?`,
		flags + `(api_key|apikey|access_key|secret_key|private_key)\s*[=:]\s*["']?([^"'\s\n\r]+)["']?`,
		flags + `(bearer\s+[a-zA-Z0-9._-]+)`,
		flags + `(basic\s+[a-zA-Z0-9+/=]+)`,
		flags + `(authorization:\s*[^\s\n\r]+)`,
		flags + `(x-api-key:\s*[^\s\n\r]+)`,
		flags + `(client_secret["\s]*[:=]["\s]*[^"\s\n\r]+)`,
		flags + `(aws_access_key_id["\s]*[:=]["\s]*[^"\s\n\r]+)`,
		flags + `(aws_secret_access_key["\s]*[:=]["\s]*[^"\s\n\r]+)`,
		flags + `(github_token["\s]*[:=]["\s]*[^"\s\n\r]+)`,
		flags + `(ssh-rsa\s+[A-Za-z0-9+/=]+)`,
		flags + `(ssh-ed25519\s+[A-Za-z0-9+/=]+)`,
		flags + `("token":\s*"[^"]+")`,
		flags + `("secret":\s*"[^"]+")`,
		flags + `("key":\s*"[^"]+")`,
	}

	s.credentialPatterns = make([]*regexp.Regexp, 0, len(credentialPatterns))
	for _, pattern := range credentialPatterns {
		if compiled, err := regexp.Compile(pattern); err == nil {
			s.credentialPatterns = append(s.credentialPatterns, compiled)
		}
	}

	// Sensitive path patterns
	pathPatterns := []string{
		flags + `/home/[^/\s]+/\.(ssh|aws|docker|kube|config)(/[^\s]*)?`,
		flags + `C:\\Users\\[^\\]+\\AppData\\[^\s]*`,
		flags + `/var/lib/docker(/[^\s]*)?`,
		flags + `/etc/(shadow|passwd|sudoers|ssl)(/[^\s]*)?`,
		flags + `/root/\.[^\s]*`,
		flags + `\.ssh/[^\s]*`,
		flags + `\.aws/[^\s]*`,
		flags + `\.kube/[^\s]*`,
		flags + `\.docker/[^\s]*`,
		flags + `/tmp/[a-zA-Z0-9_-]+\.(key|pem|crt|p12|pfx)`,
		flags + `/var/secrets/[^\s]*`,
		flags + `/etc/secrets/[^\s]*`,
	}

	s.sensitivePathPatterns = make([]*regexp.Regexp, 0, len(pathPatterns))
	for _, pattern := range pathPatterns {
		if compiled, err := regexp.Compile(pattern); err == nil {
			s.sensitivePathPatterns = append(s.sensitivePathPatterns, compiled)
		}
	}
}

// SanitizeString sanitizes a string by removing sensitive information
func (s *Sanitizer) SanitizeString(input string) string {
	result := input

	// Apply credential sanitization
	if s.config.EnableCredentialSanitization {
		result = s.sanitizeCredentials(result)
	}

	// Apply path sanitization
	if s.config.EnablePathSanitization {
		result = s.sanitizePaths(result)
	}

	// Apply custom rules
	if s.config.EnableCustomRules {
		result = s.applyCustomRules(result)
	}

	return result
}

// sanitizeCredentials removes credential information from strings
func (s *Sanitizer) sanitizeCredentials(input string) string {
	result := input

	for _, pattern := range s.credentialPatterns {
		result = pattern.ReplaceAllStringFunc(result, func(match string) string {
			// Find the credential part and replace it
			parts := pattern.FindStringSubmatch(match)
			if len(parts) >= 2 {
				// Replace the last capturing group (the actual credential)
				credentialValue := parts[len(parts)-1]
				return strings.Replace(match, credentialValue, s.config.RedactionText, 1)
			}
			return s.config.RedactionText
		})
	}

	return result
}

// sanitizePaths removes sensitive path information from strings
func (s *Sanitizer) sanitizePaths(input string) string {
	result := input

	for _, pattern := range s.sensitivePathPatterns {
		result = pattern.ReplaceAllString(result, s.config.PathRedactionText)
	}

	return result
}

// applyCustomRules applies custom sanitization rules
func (s *Sanitizer) applyCustomRules(input string) string {
	result := input

	for _, rule := range s.customPatterns {
		if rule.Enabled && rule.Pattern != nil {
			result = rule.Pattern.ReplaceAllString(result, rule.Replacement)
		}
	}

	return result
}

// AddCustomRule adds a custom sanitization rule
func (s *Sanitizer) AddCustomRule(rule SanitizationRule) error {
	// Compile the pattern
	compiled, err := regexp.Compile(rule.PatternStr)
	if err != nil {
		return err
	}

	rule.Pattern = compiled
	s.customPatterns = append(s.customPatterns, &rule)
	return nil
}

// RemoveCustomRule removes a custom sanitization rule by name
func (s *Sanitizer) RemoveCustomRule(name string) bool {
	for i, rule := range s.customPatterns {
		if rule.Name == name {
			s.customPatterns = append(s.customPatterns[:i], s.customPatterns[i+1:]...)
			return true
		}
	}
	return false
}

// GetCustomRules returns all custom sanitization rules
func (s *Sanitizer) GetCustomRules() []*SanitizationRule {
	return s.customPatterns
}

// SanitizeMap sanitizes all string values in a map
func (s *Sanitizer) SanitizeMap(input map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	for key, value := range input {
		switch v := value.(type) {
		case string:
			result[key] = s.SanitizeString(v)
		case map[string]interface{}:
			result[key] = s.SanitizeMap(v)
		case []interface{}:
			result[key] = s.SanitizeSlice(v)
		default:
			result[key] = value
		}
	}

	return result
}

// SanitizeSlice sanitizes all string values in a slice
func (s *Sanitizer) SanitizeSlice(input []interface{}) []interface{} {
	result := make([]interface{}, len(input))

	for i, value := range input {
		switch v := value.(type) {
		case string:
			result[i] = s.SanitizeString(v)
		case map[string]interface{}:
			result[i] = s.SanitizeMap(v)
		case []interface{}:
			result[i] = s.SanitizeSlice(v)
		default:
			result[i] = value
		}
	}

	return result
}

// DetectSensitiveData checks if a string contains sensitive information
func (s *Sanitizer) DetectSensitiveData(input string) []SensitiveDataMatch {
	matches := make([]SensitiveDataMatch, 0)

	// Check credential patterns
	if s.config.EnableCredentialSanitization {
		for i, pattern := range s.credentialPatterns {
			if locations := pattern.FindAllStringIndex(input, -1); len(locations) > 0 {
				for _, loc := range locations {
					matches = append(matches, SensitiveDataMatch{
						Type:        "credential",
						Pattern:     pattern.String(),
						PatternName: fmt.Sprintf("credential_pattern_%d", i),
						Start:       loc[0],
						End:         loc[1],
						Value:       input[loc[0]:loc[1]],
					})
				}
			}
		}
	}

	// Check path patterns
	if s.config.EnablePathSanitization {
		for i, pattern := range s.sensitivePathPatterns {
			if locations := pattern.FindAllStringIndex(input, -1); len(locations) > 0 {
				for _, loc := range locations {
					matches = append(matches, SensitiveDataMatch{
						Type:        "sensitive_path",
						Pattern:     pattern.String(),
						PatternName: fmt.Sprintf("path_pattern_%d", i),
						Start:       loc[0],
						End:         loc[1],
						Value:       input[loc[0]:loc[1]],
					})
				}
			}
		}
	}

	// Check custom patterns
	if s.config.EnableCustomRules {
		for _, rule := range s.customPatterns {
			if rule.Enabled && rule.Pattern != nil {
				if locations := rule.Pattern.FindAllStringIndex(input, -1); len(locations) > 0 {
					for _, loc := range locations {
						matches = append(matches, SensitiveDataMatch{
							Type:        "custom",
							Pattern:     rule.PatternStr,
							PatternName: rule.Name,
							Start:       loc[0],
							End:         loc[1],
							Value:       input[loc[0]:loc[1]],
						})
					}
				}
			}
		}
	}

	return matches
}

// SensitiveDataMatch represents a detected sensitive data match
type SensitiveDataMatch struct {
	Type        string `json:"type"`
	Pattern     string `json:"pattern"`
	PatternName string `json:"pattern_name"`
	Start       int    `json:"start"`
	End         int    `json:"end"`
	Value       string `json:"value"`
}

// HasSensitiveData returns true if the input contains sensitive data
func (s *Sanitizer) HasSensitiveData(input string) bool {
	return len(s.DetectSensitiveData(input)) > 0
}

// GetSanitizationReport generates a report of sanitization actions
func (s *Sanitizer) GetSanitizationReport(input string) SanitizationReport {
	original := input
	sanitized := s.SanitizeString(input)
	matches := s.DetectSensitiveData(input)

	return SanitizationReport{
		Original:         original,
		Sanitized:        sanitized,
		HasSensitiveData: len(matches) > 0,
		Matches:          matches,
		RulesApplied:     s.getRulesApplied(),
		RedactionCount:   strings.Count(sanitized, s.config.RedactionText) + strings.Count(sanitized, s.config.PathRedactionText),
	}
}

// SanitizationReport provides detailed information about sanitization
type SanitizationReport struct {
	Original         string               `json:"original"`
	Sanitized        string               `json:"sanitized"`
	HasSensitiveData bool                 `json:"has_sensitive_data"`
	Matches          []SensitiveDataMatch `json:"matches"`
	RulesApplied     []string             `json:"rules_applied"`
	RedactionCount   int                  `json:"redaction_count"`
}

// getRulesApplied returns a list of rules that are enabled
func (s *Sanitizer) getRulesApplied() []string {
	rules := make([]string, 0)

	if s.config.EnableCredentialSanitization {
		rules = append(rules, "credential_sanitization")
	}

	if s.config.EnablePathSanitization {
		rules = append(rules, "path_sanitization")
	}

	if s.config.EnableCustomRules {
		for _, rule := range s.customPatterns {
			if rule.Enabled {
				rules = append(rules, rule.Name)
			}
		}
	}

	return rules
}

// UpdateConfig updates the sanitizer configuration
func (s *Sanitizer) UpdateConfig(config SanitizerConfig) {
	s.config = config
	// Re-initialize patterns if case sensitivity changed
	s.initializeDefaultPatterns()
}
