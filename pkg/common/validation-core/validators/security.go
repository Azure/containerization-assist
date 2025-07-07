package validators

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/Azure/container-kit/pkg/common/validation-core/core"
)

// SecurityValidator validates security-related aspects like secrets, permissions, and sensitive data
type SecurityValidator struct {
	*BaseValidatorImpl
	secretPatterns      []*regexp.Regexp
	sensitiveKeywords   []string
	filePermissionRegex *regexp.Regexp
}

// NewSecurityValidator creates a new security validator
func NewSecurityValidator() *SecurityValidator {
	return &SecurityValidator{
		BaseValidatorImpl: NewBaseValidator("security", "1.0.0", []string{"secrets", "permissions", "sensitive_data", "compliance"}),
		secretPatterns: []*regexp.Regexp{
			// AWS Access Key ID
			regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
			// AWS Secret Access Key
			regexp.MustCompile(`[a-zA-Z0-9/+=]{40}`),
			// GitHub Token
			regexp.MustCompile(`ghp_[a-zA-Z0-9]{36}`),
			regexp.MustCompile(`gho_[a-zA-Z0-9]{36}`),
			regexp.MustCompile(`ghu_[a-zA-Z0-9]{36}`),
			regexp.MustCompile(`ghs_[a-zA-Z0-9]{36}`),
			regexp.MustCompile(`ghr_[a-zA-Z0-9]{36}`),
			// Generic API Key patterns
			regexp.MustCompile(`api[_-]?key[_-]?[:=]\s*["']?[a-zA-Z0-9]{32,}["']?`),
			regexp.MustCompile(`secret[_-]?key[_-]?[:=]\s*["']?[a-zA-Z0-9]{32,}["']?`),
			// JWT tokens
			regexp.MustCompile(`eyJ[a-zA-Z0-9_-]+\.eyJ[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+`),
			// Basic auth in URLs
			regexp.MustCompile(`https?://[^:]+:[^@]+@`),
			// Private key headers
			regexp.MustCompile(`-----BEGIN (RSA |EC |DSA |OPENSSH )?PRIVATE KEY-----`),
			// Base64 encoded secrets (minimum 20 chars)
			regexp.MustCompile(`[a-zA-Z0-9+/]{20,}={0,2}`),
		},
		sensitiveKeywords: []string{
			"password", "passwd", "pwd", "secret", "token", "api_key", "apikey",
			"access_key", "private_key", "privatekey", "auth", "credential",
		},
		filePermissionRegex: regexp.MustCompile(`^[0-7]{3,4}$`),
	}
}

// Validate validates security aspects of the data
func (s *SecurityValidator) Validate(ctx context.Context, data interface{}, options *core.ValidationOptions) *core.NonGenericResult {
	result := s.BaseValidatorImpl.Validate(ctx, data, options)

	// Check context for security validation type
	securityType := ""
	if options != nil && options.Context != nil {
		if st, ok := options.Context["security_type"].(string); ok {
			securityType = st
		}
	}

	switch securityType {
	case "secrets":
		s.validateSecrets(data, result, options)
	case "permissions":
		s.validatePermissions(data, result)
	case "sensitive_data":
		s.validateSensitiveData(data, result)
	case "compliance":
		s.validateCompliance(data, result, options)
	default:
		// Run all security validations
		if !options.ShouldSkipRule("secrets") {
			s.validateSecrets(data, result, options)
		}
		if !options.ShouldSkipRule("permissions") {
			s.validatePermissions(data, result)
		}
		if !options.ShouldSkipRule("sensitive_data") {
			s.validateSensitiveData(data, result)
		}
	}

	return result
}

// validateSecrets checks for exposed secrets and credentials
func (s *SecurityValidator) validateSecrets(data interface{}, result *core.NonGenericResult, options *core.ValidationOptions) {
	var content string
	switch v := data.(type) {
	case string:
		content = v
	case []byte:
		content = string(v)
	case map[string]interface{}:
		// Check map keys and values for secrets
		s.validateMapForSecrets(v, result)
		return
	default:
		return
	}

	// Skip if content is too short to contain meaningful secrets
	if len(content) < 10 {
		return
	}

	// Check against secret patterns
	for _, pattern := range s.secretPatterns {
		if matches := pattern.FindAllStringIndex(content, -1); len(matches) > 0 {
			for _, match := range matches {
				// Get surrounding context for better reporting
				start := match[0] - 20
				if start < 0 {
					start = 0
				}
				end := match[1] + 20
				if end > len(content) {
					end = len(content)
				}

				// Mask the actual secret in the error message
				context := content[start:end]
				masked := maskSecret(context, match[0]-start, match[1]-start)

				securityErr := core.NewError(
					"EXPOSED_SECRET",
					fmt.Sprintf("Potential secret or credential detected: %s", masked),
					core.ErrTypeSecurity,
					core.SeverityCritical,
				)

				// Add context and suggestions for remediation
				securityErr.Context = map[string]interface{}{
					"pattern_type":     "credential_pattern",
					"line_context":     context,
					"security_impact":  "high",
					"detection_method": "regex_pattern_match",
				}
				securityErr.Suggestions = []string{
					"Move secrets to environment variables or secure configuration",
					"Use a secrets management system (e.g., Azure Key Vault, HashiCorp Vault)",
					"Remove hardcoded credentials from source code",
					"Add this file to .gitignore if it contains test data only",
					"Consider using placeholder values for examples",
				}

				result.AddError(securityErr)
			}
		}
	}

	// Check for sensitive keywords with values
	lowerContent := strings.ToLower(content)
	for _, keyword := range s.sensitiveKeywords {
		if idx := strings.Index(lowerContent, keyword); idx != -1 {
			// Look for assignment patterns after the keyword
			afterKeyword := lowerContent[idx+len(keyword):]
			if assignmentRegex := regexp.MustCompile(`^\s*[:=]\s*["']?([^"'\s]{8,})["']?`); assignmentRegex.MatchString(afterKeyword) {
				warning := core.NewWarning(
					"SENSITIVE_VALUE_ASSIGNMENT",
					fmt.Sprintf("Potential sensitive value assigned to '%s'", keyword),
				)
				warning.Error.Suggestions = append(warning.Error.Suggestions, "Consider using environment variables for sensitive configuration")
				result.AddWarning(warning)
			}
		}
	}
}

// validatePermissions checks file and directory permissions
func (s *SecurityValidator) validatePermissions(data interface{}, result *core.NonGenericResult) {
	switch v := data.(type) {
	case string:
		// Validate octal permission string
		s.validatePermissionString(v, result)
	case int:
		// Validate numeric permission
		s.validateNumericPermission(v, result)
	case os.FileMode:
		// Validate FileMode
		s.validateFileMode(v, result)
	case map[string]interface{}:
		// Check for permission-related keys
		if perm, ok := v["permissions"]; ok {
			s.validatePermissions(perm, result)
		}
		if mode, ok := v["mode"]; ok {
			s.validatePermissions(mode, result)
		}
	}
}

// validatePermissionString validates octal permission strings
func (s *SecurityValidator) validatePermissionString(perm string, result *core.NonGenericResult) {
	if !s.filePermissionRegex.MatchString(perm) {
		result.AddError(core.NewError(
			"INVALID_PERMISSION_FORMAT",
			fmt.Sprintf("Invalid permission format: %s", perm),
			core.ErrTypeSecurity,
			core.SeverityMedium,
		))
		return
	}

	// Convert to numeric for validation
	permInt, err := strconv.ParseInt(perm, 8, 32)
	if err != nil {
		result.AddError(core.NewError(
			"INVALID_PERMISSION_VALUE",
			fmt.Sprintf("Cannot parse permission value: %v", err),
			core.ErrTypeSecurity,
			core.SeverityMedium,
		))
		return
	}

	s.validateNumericPermission(int(permInt), result)
}

// validateNumericPermission validates numeric permissions
func (s *SecurityValidator) validateNumericPermission(perm int, result *core.NonGenericResult) {
	// Check for overly permissive permissions
	if perm&0o777 == 0o777 {
		result.AddError(core.NewError(
			"OVERLY_PERMISSIVE",
			"File/directory has full permissions (777) for all users",
			core.ErrTypeSecurity,
			core.SeverityHigh,
		))
	}

	// Check world-writable
	if perm&0o002 != 0 {
		warning := core.NewWarning(
			"WORLD_WRITABLE",
			"File/directory is world-writable",
		)
		warning.Error.Suggestions = append(warning.Error.Suggestions, "Remove write permission for others unless specifically required")
		result.AddWarning(warning)
	}

	// Check for executable permissions on sensitive files
	if perm&0o111 != 0 {
		warning := core.NewWarning(
			"EXECUTABLE_PERMISSION",
			"File has executable permissions",
		)
		warning.Error.Context["permissions"] = fmt.Sprintf("%04o", perm)
		result.AddWarning(warning)
	}
}

// validateFileMode validates os.FileMode
func (s *SecurityValidator) validateFileMode(mode os.FileMode, result *core.NonGenericResult) {
	perm := int(mode.Perm())
	s.validateNumericPermission(perm, result)

	// Additional checks for special modes
	if mode&os.ModeSetuid != 0 {
		warning := core.NewWarning(
			"SETUID_BIT_SET",
			"File has setuid bit set",
		)
		warning.Error.Suggestions = append(warning.Error.Suggestions, "Ensure setuid is necessary for the file's operation")
		result.AddWarning(warning)
	}

	if mode&os.ModeSetgid != 0 {
		warning := core.NewWarning(
			"SETGID_BIT_SET",
			"File has setgid bit set",
		)
		warning.Error.Suggestions = append(warning.Error.Suggestions, "Ensure setgid is necessary for the file's operation")
		result.AddWarning(warning)
	}

	if mode&os.ModeSticky != 0 {
		result.AddWarning(core.NewWarning(
			"STICKY_BIT_SET",
			"File has sticky bit set",
		))
	}
}

// validateSensitiveData checks for potentially sensitive data patterns
func (s *SecurityValidator) validateSensitiveData(data interface{}, result *core.NonGenericResult) {
	content := ""
	switch v := data.(type) {
	case string:
		content = v
	case []byte:
		content = string(v)
	default:
		return
	}

	// Check for credit card patterns (simplified)
	if creditCardRegex := regexp.MustCompile(`\b\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}\b`); creditCardRegex.MatchString(content) {
		result.AddError(core.NewError(
			"POTENTIAL_CREDIT_CARD",
			"Potential credit card number detected",
			core.ErrTypeSecurity,
			core.SeverityCritical,
		))
	}

	// Check for SSN patterns
	if ssnRegex := regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`); ssnRegex.MatchString(content) {
		result.AddError(core.NewError(
			"POTENTIAL_SSN",
			"Potential Social Security Number detected",
			core.ErrTypeSecurity,
			core.SeverityCritical,
		))
	}

	// Check for email addresses in bulk (potential data leak)
	if emailRegex := regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`); len(emailRegex.FindAllString(content, -1)) > 10 {
		warning := core.NewWarning(
			"BULK_EMAIL_ADDRESSES",
			"Multiple email addresses detected",
		)
		warning.Error.Suggestions = append(warning.Error.Suggestions, "Ensure email addresses are properly protected and authorized for use")
		result.AddWarning(warning)
	}

	// Check for potential base64 encoded data
	if len(content) > 100 && isLikelyBase64(content) {
		warning := core.NewWarning(
			"BASE64_ENCODED_DATA",
			"Potential base64 encoded data detected",
		)
		warning.Error.Suggestions = append(warning.Error.Suggestions, "Ensure encoded data doesn't contain sensitive information")
		result.AddWarning(warning)
	}
}

// validateCompliance checks for compliance-related issues
func (s *SecurityValidator) validateCompliance(data interface{}, result *core.NonGenericResult, options *core.ValidationOptions) {
	// Check for specific compliance requirements from options
	if options != nil && options.Context != nil {
		if compliance, ok := options.Context["compliance_standard"].(string); ok {
			switch compliance {
			case "PCI":
				s.validatePCICompliance(data, result)
			case "HIPAA":
				s.validateHIPAACompliance(data, result)
			case "GDPR":
				s.validateGDPRCompliance(data, result)
			default:
				result.AddWarning(core.NewWarning(
					"UNKNOWN_COMPLIANCE_STANDARD",
					fmt.Sprintf("Unknown compliance standard: %s", compliance),
				))
			}
		}
	}
}

// validateMapForSecrets recursively checks maps for secrets
func (s *SecurityValidator) validateMapForSecrets(m map[string]interface{}, result *core.NonGenericResult) {
	for key, value := range m {
		// Check if key contains sensitive keywords
		lowerKey := strings.ToLower(key)
		for _, keyword := range s.sensitiveKeywords {
			if strings.Contains(lowerKey, keyword) {
				// Check the value
				if str, ok := value.(string); ok && len(str) > 0 && str != "***" && str != "<redacted>" {
					warning := core.NewWarning(
						"SENSITIVE_KEY_WITH_VALUE",
						fmt.Sprintf("Sensitive key '%s' contains a value", key),
					)
					warning.Error.Suggestions = append(warning.Error.Suggestions, "Use environment variables or secret management for sensitive values")
					result.AddWarning(warning)
				}
			}
		}

		// Recursively check nested maps
		if nestedMap, ok := value.(map[string]interface{}); ok {
			s.validateMapForSecrets(nestedMap, result)
		}
	}
}

// Helper functions

func maskSecret(content string, start, end int) string {
	if start < 0 || end > len(content) || start >= end {
		return content
	}
	masked := content[:start] + strings.Repeat("*", end-start) + content[end:]
	return masked
}

func isLikelyBase64(s string) bool {
	// Remove whitespace
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\t", "")

	// Check if it's valid base64
	if _, err := base64.StdEncoding.DecodeString(s); err == nil {
		return true
	}

	// Check URL-safe base64
	if _, err := base64.URLEncoding.DecodeString(s); err == nil {
		return true
	}

	return false
}

// Compliance validation methods (simplified examples)

func (s *SecurityValidator) validatePCICompliance(data interface{}, result *core.NonGenericResult) {
	// PCI DSS related checks
	result.Metadata.RulesApplied = append(result.Metadata.RulesApplied, "PCI-DSS")
}

func (s *SecurityValidator) validateHIPAACompliance(data interface{}, result *core.NonGenericResult) {
	// HIPAA related checks
	result.Metadata.RulesApplied = append(result.Metadata.RulesApplied, "HIPAA")
}

func (s *SecurityValidator) validateGDPRCompliance(data interface{}, result *core.NonGenericResult) {
	// GDPR related checks
	result.Metadata.RulesApplied = append(result.Metadata.RulesApplied, "GDPR")
}
