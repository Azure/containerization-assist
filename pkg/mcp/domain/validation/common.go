package validation

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// StringLengthValidator validates string length
type StringLengthValidator struct {
	MinLength int
	MaxLength int
	FieldName string
}

func NewStringLengthValidator(fieldName string, minLength, maxLength int) *StringLengthValidator {
	return &StringLengthValidator{
		MinLength: minLength,
		MaxLength: maxLength,
		FieldName: fieldName,
	}
}

func (v *StringLengthValidator) Validate(ctx context.Context, value string) ValidationResult {
	result := ValidationResult{Valid: true, Errors: make([]error, 0)}

	if len(value) < v.MinLength {
		result.Valid = false
		result.Errors = append(result.Errors, errors.NewValidationFailed(
			v.FieldName,
			fmt.Sprintf("length %d is less than minimum %d", len(value), v.MinLength),
		))
	}

	if len(value) > v.MaxLength {
		result.Valid = false
		result.Errors = append(result.Errors, errors.NewValidationFailed(
			v.FieldName,
			fmt.Sprintf("length %d exceeds maximum %d", len(value), v.MaxLength),
		))
	}

	return result
}

func (v *StringLengthValidator) Name() string {
	return "StringLengthValidator"
}

// PatternValidator validates string patterns
type PatternValidator struct {
	Pattern   *regexp.Regexp
	FieldName string
}

func NewPatternValidator(fieldName, pattern string) (*PatternValidator, error) {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return nil, errors.NewValidationFailed("pattern", fmt.Sprintf("invalid regex: %v", err))
	}

	return &PatternValidator{
		Pattern:   regex,
		FieldName: fieldName,
	}, nil
}

func (v *PatternValidator) Validate(ctx context.Context, value string) ValidationResult {
	result := ValidationResult{Valid: true, Errors: make([]error, 0)}

	if !v.Pattern.MatchString(value) {
		result.Valid = false
		result.Errors = append(result.Errors, errors.NewValidationFailed(
			v.FieldName,
			fmt.Sprintf("value does not match pattern %s", v.Pattern.String()),
		))
	}

	return result
}

func (v *PatternValidator) Name() string {
	return "PatternValidator"
}

// RequiredValidator validates required fields
type RequiredValidator struct {
	FieldName string
}

func NewRequiredValidator(fieldName string) *RequiredValidator {
	return &RequiredValidator{FieldName: fieldName}
}

func (v *RequiredValidator) Validate(ctx context.Context, value string) ValidationResult {
	result := ValidationResult{Valid: true, Errors: make([]error, 0)}

	if strings.TrimSpace(value) == "" {
		result.Valid = false
		result.Errors = append(result.Errors, errors.NewMissingParam(v.FieldName))
	}

	return result
}

func (v *RequiredValidator) Name() string {
	return "RequiredValidator"
}

// EmailValidator validates email format
type EmailValidator struct {
	FieldName string
}

func NewEmailValidator(fieldName string) *EmailValidator {
	return &EmailValidator{FieldName: fieldName}
}

func (v *EmailValidator) Validate(ctx context.Context, value string) ValidationResult {
	result := ValidationResult{Valid: true, Errors: make([]error, 0)}

	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(value) {
		result.Valid = false
		result.Errors = append(result.Errors, errors.NewValidationFailed(
			v.FieldName,
			"invalid email format",
		))
	}

	return result
}

func (v *EmailValidator) Name() string {
	return "EmailValidator"
}

// URLValidator validates URL format
type URLValidator struct {
	FieldName string
}

func NewURLValidator(fieldName string) *URLValidator {
	return &URLValidator{FieldName: fieldName}
}

func (v *URLValidator) Validate(ctx context.Context, value string) ValidationResult {
	result := ValidationResult{Valid: true, Errors: make([]error, 0)}

	urlRegex := regexp.MustCompile(`^https?://[^\s/$.?#].[^\s]*$`)
	if !urlRegex.MatchString(value) {
		result.Valid = false
		result.Errors = append(result.Errors, errors.NewValidationFailed(
			v.FieldName,
			"invalid URL format",
		))
	}

	return result
}

func (v *URLValidator) Name() string {
	return "URLValidator"
}

// NetworkPortValidator validates network ports
type NetworkPortValidator struct {
	FieldName string
	AllowZero bool
}

func NewNetworkPortValidator(fieldName string, allowZero bool) *NetworkPortValidator {
	return &NetworkPortValidator{
		FieldName: fieldName,
		AllowZero: allowZero,
	}
}

func (v *NetworkPortValidator) Validate(ctx context.Context, value int) ValidationResult {
	result := ValidationResult{Valid: true, Errors: make([]error, 0)}

	minPort := 1
	if v.AllowZero {
		minPort = 0
	}

	if value < minPort || value > 65535 {
		result.Valid = false
		result.Errors = append(result.Errors, errors.NewValidationFailed(
			v.FieldName,
			fmt.Sprintf("port %d must be between %d and 65535", value, minPort),
		))
	}

	return result
}

func (v *NetworkPortValidator) Name() string {
	return "NetworkPortValidator"
}

// IPAddressValidator validates IP addresses (IPv4/IPv6)
type IPAddressValidator struct {
	FieldName     string
	AllowIPv4     bool
	AllowIPv6     bool
	AllowLoopback bool
}

func NewIPAddressValidator(fieldName string, allowIPv4, allowIPv6, allowLoopback bool) *IPAddressValidator {
	return &IPAddressValidator{
		FieldName:     fieldName,
		AllowIPv4:     allowIPv4,
		AllowIPv6:     allowIPv6,
		AllowLoopback: allowLoopback,
	}
}

func (v *IPAddressValidator) Validate(ctx context.Context, value string) ValidationResult {
	result := ValidationResult{Valid: true, Errors: make([]error, 0)}

	// Basic IPv4 pattern
	ipv4Regex := regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}$`)
	// Basic IPv6 pattern
	ipv6Regex := regexp.MustCompile(`^([0-9a-fA-F]{0,4}:){1,7}[0-9a-fA-F]{0,4}$`)

	isIPv4 := ipv4Regex.MatchString(value)
	isIPv6 := ipv6Regex.MatchString(value)
	isLoopback := value == "127.0.0.1" || value == "::1" || value == "localhost"

	if !isIPv4 && !isIPv6 {
		result.Valid = false
		result.Errors = append(result.Errors, errors.NewValidationFailed(
			v.FieldName,
			"invalid IP address format",
		))
		return result
	}

	if isIPv4 && !v.AllowIPv4 {
		result.Valid = false
		result.Errors = append(result.Errors, errors.NewValidationFailed(
			v.FieldName,
			"IPv4 addresses not allowed",
		))
	}

	if isIPv6 && !v.AllowIPv6 {
		result.Valid = false
		result.Errors = append(result.Errors, errors.NewValidationFailed(
			v.FieldName,
			"IPv6 addresses not allowed",
		))
	}

	if isLoopback && !v.AllowLoopback {
		result.Valid = false
		result.Errors = append(result.Errors, errors.NewValidationFailed(
			v.FieldName,
			"loopback addresses not allowed",
		))
	}

	return result
}

func (v *IPAddressValidator) Name() string {
	return "IPAddressValidator"
}
