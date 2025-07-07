package utils

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/Azure/container-kit/pkg/common/validation-core/core"
)

// String validation utilities consolidated from multiple packages

var (
	// Email validation regex
	emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

	// URL validation regex
	urlRegex = regexp.MustCompile(`^https?://[a-zA-Z0-9.-]+(?:\.[a-zA-Z]{2,})?(?:/.*)?$`)

	// Docker image name regex
	dockerImageRegex = regexp.MustCompile(`^(?:[a-zA-Z0-9._-]+/)?[a-zA-Z0-9._-]+(?::[a-zA-Z0-9._-]+)?$`)

	// Kubernetes resource name regex
	k8sNameRegex = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

	// Alphanumeric regex
	alphanumericRegex = regexp.MustCompile(`^[a-zA-Z0-9]+$`)

	// Alphanumeric with hyphens regex
	alphanumericHyphenRegex = regexp.MustCompile(`^[a-zA-Z0-9-]+$`)
)

// ValidateRequired validates that a string is not empty
func ValidateRequired(value, fieldName string) *core.Error {
	if strings.TrimSpace(value) == "" {
		return core.NewFieldError(fieldName, "is required and cannot be empty")
	}
	return nil
}

// ValidateMinLength validates minimum string length
func ValidateMinLength(value, fieldName string, minLength int) *core.Error {
	if len(value) < minLength {
		return core.NewFieldError(fieldName,
			"must be at least "+string(rune(minLength))+" characters long")
	}
	return nil
}

// ValidateMaxLength validates maximum string length
func ValidateMaxLength(value, fieldName string, maxLength int) *core.Error {
	if len(value) > maxLength {
		return core.NewFieldError(fieldName,
			"must be no more than "+string(rune(maxLength))+" characters long")
	}
	return nil
}

// ValidateLength validates string length within a range
func ValidateLength(value, fieldName string, minLength, maxLength int) *core.Error {
	if err := ValidateMinLength(value, fieldName, minLength); err != nil {
		return err
	}
	if err := ValidateMaxLength(value, fieldName, maxLength); err != nil {
		return err
	}
	return nil
}

// ValidateEmail validates email format
func ValidateEmail(email, fieldName string) *core.Error {
	if email == "" {
		return nil // Allow empty emails unless required validation is also used
	}

	if !emailRegex.MatchString(email) {
		return core.NewFieldError(fieldName, "must be a valid email address").
			WithSuggestion("Use format: user@domain.com")
	}
	return nil
}

// ValidateURL validates URL format
func ValidateURL(url, fieldName string) *core.Error {
	if url == "" {
		return nil // Allow empty URLs unless required validation is also used
	}

	if !urlRegex.MatchString(url) {
		return core.NewFieldError(fieldName, "must be a valid HTTP or HTTPS URL").
			WithSuggestion("Use format: https://example.com")
	}
	return nil
}

// ValidateDockerImageName validates Docker image name format
func ValidateDockerImageName(imageName, fieldName string) *core.Error {
	if imageName == "" {
		return nil
	}

	if !dockerImageRegex.MatchString(imageName) {
		return core.NewFieldError(fieldName, "must be a valid Docker image name").
			WithSuggestion("Use format: [registry/]image[:tag]")
	}

	// Additional validation for length and specific rules
	if len(imageName) > 255 {
		return core.NewFieldError(fieldName, "Docker image name is too long (max 255 characters)")
	}

	return nil
}

// ValidateKubernetesResourceName validates Kubernetes resource name format
func ValidateKubernetesResourceName(name, fieldName string) *core.Error {
	if name == "" {
		return nil
	}

	if len(name) > 253 {
		return core.NewFieldError(fieldName, "Kubernetes resource name is too long (max 253 characters)")
	}

	if !k8sNameRegex.MatchString(name) {
		return core.NewFieldError(fieldName,
			"must be a valid Kubernetes resource name (lowercase alphanumeric with hyphens)").
			WithSuggestion("Use format: my-resource-name")
	}

	return nil
}

// ValidateAlphanumeric validates that string contains only alphanumeric characters
func ValidateAlphanumeric(value, fieldName string) *core.Error {
	if value == "" {
		return nil
	}

	if !alphanumericRegex.MatchString(value) {
		return core.NewFieldError(fieldName, "must contain only alphanumeric characters (a-z, A-Z, 0-9)")
	}
	return nil
}

// ValidateAlphanumericWithHyphens validates alphanumeric with hyphens
func ValidateAlphanumericWithHyphens(value, fieldName string) *core.Error {
	if value == "" {
		return nil
	}

	if !alphanumericHyphenRegex.MatchString(value) {
		return core.NewFieldError(fieldName, "must contain only alphanumeric characters and hyphens")
	}
	return nil
}

// ValidateNoWhitespace validates that string contains no whitespace
func ValidateNoWhitespace(value, fieldName string) *core.Error {
	if value == "" {
		return nil
	}

	for _, r := range value {
		if unicode.IsSpace(r) {
			return core.NewFieldError(fieldName, "must not contain whitespace characters")
		}
	}
	return nil
}

// ValidatePattern validates string against a custom regex pattern
func ValidatePattern(value, fieldName, pattern, description string) *core.Error {
	if value == "" {
		return nil
	}

	regex, err := regexp.Compile(pattern)
	if err != nil {
		return core.NewError(
			"INVALID_PATTERN",
			"Invalid validation pattern: "+err.Error(),
			core.ErrTypeValidation,
			core.SeverityHigh,
		).WithField(fieldName)
	}

	if !regex.MatchString(value) {
		error := core.NewFieldError(fieldName, "does not match required pattern")
		if description != "" {
			error.Suggestions = append(error.Suggestions, description)
		}
		return error
	}
	return nil
}

// ValidateOneOf validates that string is one of the allowed values
func ValidateOneOf(value, fieldName string, allowedValues []string) *core.Error {
	if value == "" {
		return nil
	}

	for _, allowed := range allowedValues {
		if value == allowed {
			return nil
		}
	}

	return core.NewFieldError(fieldName,
		"must be one of: "+strings.Join(allowedValues, ", "))
}

// ValidateNotOneOf validates that string is not one of the forbidden values
func ValidateNotOneOf(value, fieldName string, forbiddenValues []string) *core.Error {
	if value == "" {
		return nil
	}

	for _, forbidden := range forbiddenValues {
		if value == forbidden {
			return core.NewFieldError(fieldName,
				"must not be one of: "+strings.Join(forbiddenValues, ", "))
		}
	}
	return nil
}

// ValidateStartsWith validates that string starts with a specific prefix
func ValidateStartsWith(value, fieldName, prefix string) *core.Error {
	if value == "" {
		return nil
	}

	if !strings.HasPrefix(value, prefix) {
		return core.NewFieldError(fieldName, "must start with '"+prefix+"'")
	}
	return nil
}

// ValidateEndsWith validates that string ends with a specific suffix
func ValidateEndsWith(value, fieldName, suffix string) *core.Error {
	if value == "" {
		return nil
	}

	if !strings.HasSuffix(value, suffix) {
		return core.NewFieldError(fieldName, "must end with '"+suffix+"'")
	}
	return nil
}

// ValidateContains validates that string contains a specific substring
func ValidateContains(value, fieldName, substring string) *core.Error {
	if value == "" {
		return nil
	}

	if !strings.Contains(value, substring) {
		return core.NewFieldError(fieldName, "must contain '"+substring+"'")
	}
	return nil
}

// ValidateNotContains validates that string does not contain a specific substring
func ValidateNotContains(value, fieldName, substring string) *core.Error {
	if value == "" {
		return nil
	}

	if strings.Contains(value, substring) {
		return core.NewFieldError(fieldName, "must not contain '"+substring+"'")
	}
	return nil
}

// ValidateNoSpecialChars validates that string contains no special characters
func ValidateNoSpecialChars(value, fieldName string) *core.Error {
	if value == "" {
		return nil
	}

	for _, r := range value {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' && r != '_' {
			return core.NewFieldError(fieldName,
				"must contain only letters, numbers, hyphens, and underscores")
		}
	}
	return nil
}

// ValidateStringSlice validates a slice of strings using the provided validator
func ValidateStringSlice(values []string, fieldName string,
	validator func(string, string) *core.Error) []*core.Error {
	var errors []*core.Error

	for i, value := range values {
		elementFieldName := fieldName + "[" + string(rune(i)) + "]"
		if err := validator(value, elementFieldName); err != nil {
			errors = append(errors, err)
		}
	}

	return errors
}
