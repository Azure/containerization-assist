package validators

import (
	"context"
	"encoding/json"
	"fmt"
	"net/mail"
	"net/url"
	"regexp"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/validation/core"
	"gopkg.in/yaml.v3"
)

// FormatValidator validates various data formats like email, URL, JSON, YAML
type FormatValidator struct {
	*BaseValidatorImpl
	emailRegex *regexp.Regexp
}

// NewFormatValidator creates a new format validator
func NewFormatValidator() *FormatValidator {
	return &FormatValidator{
		BaseValidatorImpl: NewBaseValidator("format", "1.0.0", []string{"email", "url", "json", "yaml"}),
		emailRegex:        regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`),
	}
}

// Validate validates data based on its format type
func (f *FormatValidator) Validate(ctx context.Context, data interface{}, options *core.ValidationOptions) *core.ValidationResult {
	result := f.BaseValidatorImpl.Validate(ctx, data, options)

	// Check context for format type
	formatType := ""
	if options != nil && options.Context != nil {
		if ft, ok := options.Context["format_type"].(string); ok {
			formatType = ft
		}
	}

	switch formatType {
	case "email":
		f.validateEmail(data, result)
	case "url":
		f.validateURL(data, result)
	case "json":
		f.validateJSON(data, result)
	case "yaml":
		f.validateYAML(data, result)
	default:
		// Try to detect format automatically
		f.autoDetectAndValidate(data, result)
	}

	return result
}

// validateEmail validates email format
func (f *FormatValidator) validateEmail(data interface{}, result *core.ValidationResult) {
	emailStr, ok := data.(string)
	if !ok {
		result.AddError(core.NewValidationError(
			"INVALID_EMAIL_TYPE",
			"Email must be a string",
			core.ErrTypeFormat,
			core.SeverityHigh,
		))
		return
	}

	// Try standard library email parser first
	if _, err := mail.ParseAddress(emailStr); err != nil {
		// Fallback to regex for simpler validation
		if !f.emailRegex.MatchString(emailStr) {
			result.AddError(core.NewValidationError(
				"INVALID_EMAIL_FORMAT",
				fmt.Sprintf("Invalid email format: %s", emailStr),
				core.ErrTypeFormat,
				core.SeverityMedium,
			).WithSuggestion("Email should be in format: user@domain.com"))
		}
	}
}

// validateURL validates URL format
func (f *FormatValidator) validateURL(data interface{}, result *core.ValidationResult) {
	urlStr, ok := data.(string)
	if !ok {
		result.AddError(core.NewValidationError(
			"INVALID_URL_TYPE",
			"URL must be a string",
			core.ErrTypeFormat,
			core.SeverityHigh,
		))
		return
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		result.AddError(core.NewValidationError(
			"INVALID_URL_FORMAT",
			fmt.Sprintf("Invalid URL format: %v", err),
			core.ErrTypeFormat,
			core.SeverityMedium,
		))
		return
	}

	// Check for required components
	if parsedURL.Scheme == "" {
		result.AddWarning(core.NewValidationWarning(
			"MISSING_URL_SCHEME",
			"URL is missing scheme (http/https)",
		))
	}

	if parsedURL.Host == "" {
		result.AddError(core.NewValidationError(
			"MISSING_URL_HOST",
			"URL is missing host",
			core.ErrTypeFormat,
			core.SeverityMedium,
		))
	}
}

// validateJSON validates JSON format
func (f *FormatValidator) validateJSON(data interface{}, result *core.ValidationResult) {
	var jsonStr string
	switch v := data.(type) {
	case string:
		jsonStr = v
	case []byte:
		jsonStr = string(v)
	default:
		// Try to marshal and unmarshal to validate structure
		if _, err := json.Marshal(data); err != nil {
			result.AddError(core.NewValidationError(
				"INVALID_JSON_STRUCTURE",
				fmt.Sprintf("Cannot marshal data to JSON: %v", err),
				core.ErrTypeFormat,
				core.SeverityHigh,
			))
		}
		return
	}

	var jsonData interface{}
	if err := json.Unmarshal([]byte(jsonStr), &jsonData); err != nil {
		// Try to provide more specific error location
		if syntaxErr, ok := err.(*json.SyntaxError); ok {
			result.AddError(core.NewValidationError(
				"INVALID_JSON_SYNTAX",
				fmt.Sprintf("JSON syntax error at offset %d: %v", syntaxErr.Offset, err),
				core.ErrTypeFormat,
				core.SeverityHigh,
			).WithContext("offset", syntaxErr.Offset))
		} else {
			result.AddError(core.NewValidationError(
				"INVALID_JSON_FORMAT",
				fmt.Sprintf("Invalid JSON format: %v", err),
				core.ErrTypeFormat,
				core.SeverityHigh,
			))
		}
	}
}

// validateYAML validates YAML format
func (f *FormatValidator) validateYAML(data interface{}, result *core.ValidationResult) {
	var yamlStr string
	switch v := data.(type) {
	case string:
		yamlStr = v
	case []byte:
		yamlStr = string(v)
	default:
		// Try to marshal and unmarshal to validate structure
		if _, err := yaml.Marshal(data); err != nil {
			result.AddError(core.NewValidationError(
				"INVALID_YAML_STRUCTURE",
				fmt.Sprintf("Cannot marshal data to YAML: %v", err),
				core.ErrTypeFormat,
				core.SeverityHigh,
			))
		}
		return
	}

	var yamlData interface{}
	if err := yaml.Unmarshal([]byte(yamlStr), &yamlData); err != nil {
		result.AddError(core.NewValidationError(
			"INVALID_YAML_FORMAT",
			fmt.Sprintf("Invalid YAML format: %v", err),
			core.ErrTypeFormat,
			core.SeverityHigh,
		))
	}
}

// autoDetectAndValidate tries to detect the format and validate accordingly
func (f *FormatValidator) autoDetectAndValidate(data interface{}, result *core.ValidationResult) {
	str, ok := data.(string)
	if !ok {
		// For non-string data, try JSON validation
		f.validateJSON(data, result)
		return
	}

	str = strings.TrimSpace(str)

	// Check if it's an email
	if strings.Contains(str, "@") && !strings.Contains(str, " ") {
		f.validateEmail(str, result)
		return
	}

	// Check if it's a URL
	if strings.HasPrefix(str, "http://") || strings.HasPrefix(str, "https://") ||
		strings.HasPrefix(str, "ftp://") || strings.HasPrefix(str, "file://") {
		f.validateURL(str, result)
		return
	}

	// Check if it's JSON
	if (strings.HasPrefix(str, "{") && strings.HasSuffix(str, "}")) ||
		(strings.HasPrefix(str, "[") && strings.HasSuffix(str, "]")) {
		f.validateJSON(str, result)
		return
	}

	// Check if it's YAML (basic detection)
	if strings.Contains(str, ":") && !strings.HasPrefix(str, "http") {
		f.validateYAML(str, result)
		return
	}

	// If we can't detect the format, add a warning
	result.AddWarning(core.NewValidationWarning(
		"UNKNOWN_FORMAT",
		"Unable to detect data format for validation",
	))
}
