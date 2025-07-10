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
	name      string
}

func NewStringLengthValidator(name, fieldName string, minLength, maxLength int) *StringLengthValidator {
	return &StringLengthValidator{
		MinLength: minLength,
		MaxLength: maxLength,
		FieldName: fieldName,
		name:      name,
	}
}

func (v *StringLengthValidator) Validate(_ context.Context, value interface{}) ValidationResult {
	str, ok := value.(string)
	if !ok {
		return ValidationResult{
			Valid:  false,
			Errors: []error{errors.NewValidationFailed("input", "expected string input")},
		}
	}
	result := ValidationResult{Valid: true, Errors: make([]error, 0)}

	if len(str) < v.MinLength {
		result.Valid = false
		result.Errors = append(result.Errors, errors.NewValidationFailed(
			v.FieldName,
			fmt.Sprintf("length %d is less than minimum %d", len(str), v.MinLength),
		))
	}

	if v.MaxLength > 0 && len(str) > v.MaxLength {
		result.Valid = false
		result.Errors = append(result.Errors, errors.NewValidationFailed(
			v.FieldName,
			fmt.Sprintf("length %d exceeds maximum %d", len(str), v.MaxLength),
		))
	}

	return result
}

func (v *StringLengthValidator) Name() string {
	return v.name
}

func (v *StringLengthValidator) Domain() string {
	return "common"
}

func (v *StringLengthValidator) Category() string {
	return "string"
}

func (v *StringLengthValidator) Priority() int {
	return 50
}

func (v *StringLengthValidator) Dependencies() []string {
	return []string{}
}

// PatternValidator validates string patterns
type PatternValidator struct {
	Pattern   *regexp.Regexp
	FieldName string
	name      string
}

func NewPatternValidator(name, fieldName, pattern string) (*PatternValidator, error) {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return nil, errors.NewValidationFailed("pattern", fmt.Sprintf("invalid regex: %v", err))
	}

	return &PatternValidator{
		Pattern:   regex,
		FieldName: fieldName,
		name:      name,
	}, nil
}

func (v *PatternValidator) Validate(_ context.Context, value interface{}) ValidationResult {
	str, ok := value.(string)
	if !ok {
		return ValidationResult{
			Valid:  false,
			Errors: []error{errors.NewValidationFailed("input", "expected string input")},
		}
	}
	result := ValidationResult{Valid: true, Errors: make([]error, 0)}

	if !v.Pattern.MatchString(str) {
		result.Valid = false
		result.Errors = append(result.Errors, errors.NewValidationFailed(
			v.FieldName,
			fmt.Sprintf("value does not match pattern %s", v.Pattern.String()),
		))
	}

	return result
}

func (v *PatternValidator) Name() string {
	return v.name
}

func (v *PatternValidator) Domain() string {
	return "common"
}

func (v *PatternValidator) Category() string {
	return "pattern"
}

func (v *PatternValidator) Priority() int {
	return 55
}

func (v *PatternValidator) Dependencies() []string {
	return []string{}
}

// RequiredValidator validates required fields
type RequiredValidator struct {
	FieldName string
	name      string
}

func NewRequiredValidator(name, fieldName string) *RequiredValidator {
	return &RequiredValidator{FieldName: fieldName, name: name}
}

func (v *RequiredValidator) Validate(_ context.Context, value interface{}) ValidationResult {
	str, ok := value.(string)
	if !ok {
		return ValidationResult{
			Valid:  false,
			Errors: []error{errors.NewValidationFailed("input", "expected string input")},
		}
	}
	result := ValidationResult{Valid: true, Errors: make([]error, 0)}

	if strings.TrimSpace(str) == "" {
		result.Valid = false
		result.Errors = append(result.Errors, errors.NewMissingParam(v.FieldName))
	}

	return result
}

func (v *RequiredValidator) Name() string {
	return v.name
}

func (v *RequiredValidator) Domain() string {
	return "common"
}

func (v *RequiredValidator) Category() string {
	return "required"
}

func (v *RequiredValidator) Priority() int {
	return 100
}

func (v *RequiredValidator) Dependencies() []string {
	return []string{}
}

// EmailValidator validates email format
type EmailValidator struct {
	FieldName string
	name      string
}

func NewEmailValidator(name, fieldName string) *EmailValidator {
	return &EmailValidator{FieldName: fieldName, name: name}
}

func (v *EmailValidator) Validate(_ context.Context, value interface{}) ValidationResult {
	str, ok := value.(string)
	if !ok {
		return ValidationResult{
			Valid:  false,
			Errors: []error{errors.NewValidationFailed("input", "expected string input")},
		}
	}
	result := ValidationResult{Valid: true, Errors: make([]error, 0)}

	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(str) {
		result.Valid = false
		result.Errors = append(result.Errors, errors.NewValidationFailed(
			v.FieldName,
			"invalid email format",
		))
	}

	return result
}

func (v *EmailValidator) Name() string {
	return v.name
}

func (v *EmailValidator) Domain() string {
	return "common"
}

func (v *EmailValidator) Category() string {
	return "string"
}

func (v *EmailValidator) Priority() int {
	return 60
}

func (v *EmailValidator) Dependencies() []string {
	return []string{}
}

// URLValidator validates URL format
type URLValidator struct {
	FieldName string
	name      string
}

func NewURLValidator(name, fieldName string) *URLValidator {
	return &URLValidator{FieldName: fieldName, name: name}
}

func (v *URLValidator) Validate(_ context.Context, value interface{}) ValidationResult {
	str, ok := value.(string)
	if !ok {
		return ValidationResult{
			Valid:  false,
			Errors: []error{errors.NewValidationFailed("input", "expected string input")},
		}
	}
	result := ValidationResult{Valid: true, Errors: make([]error, 0)}

	urlRegex := regexp.MustCompile(`^https?://[^\s/$.?#].[^\s]*$`)
	if !urlRegex.MatchString(str) {
		result.Valid = false
		result.Errors = append(result.Errors, errors.NewValidationFailed(
			v.FieldName,
			"invalid URL format",
		))
	}

	return result
}

func (v *URLValidator) Name() string {
	return v.name
}

func (v *URLValidator) Domain() string {
	return "common"
}

func (v *URLValidator) Category() string {
	return "string"
}

func (v *URLValidator) Priority() int {
	return 60
}

func (v *URLValidator) Dependencies() []string {
	return []string{}
}

// NetworkPortValidator validates network ports
type NetworkPortValidator struct {
	FieldName string
	AllowZero bool
	name      string
}

func NewNetworkPortValidator(name, fieldName string, allowZero bool) *NetworkPortValidator {
	return &NetworkPortValidator{
		FieldName: fieldName,
		AllowZero: allowZero,
		name:      name,
	}
}

func (v *NetworkPortValidator) Validate(_ context.Context, value interface{}) ValidationResult {
	var port int
	var ok bool

	switch val := value.(type) {
	case int:
		port = val
		ok = true
	case int64:
		port = int(val)
		ok = true
	case int32:
		port = int(val)
		ok = true
	default:
		ok = false
	}

	if !ok {
		return ValidationResult{
			Valid:  false,
			Errors: []error{errors.NewValidationFailed("input", "expected integer input")},
		}
	}
	result := ValidationResult{Valid: true, Errors: make([]error, 0)}

	minPort := 1
	if v.AllowZero {
		minPort = 0
	}

	if port < minPort || port > 65535 {
		result.Valid = false
		result.Errors = append(result.Errors, errors.NewValidationFailed(
			v.FieldName,
			fmt.Sprintf("port %d must be between %d and 65535", port, minPort),
		))
	}

	return result
}

func (v *NetworkPortValidator) Name() string {
	return v.name
}

func (v *NetworkPortValidator) Domain() string {
	return "network"
}

func (v *NetworkPortValidator) Category() string {
	return "port"
}

func (v *NetworkPortValidator) Priority() int {
	return 70
}

func (v *NetworkPortValidator) Dependencies() []string {
	return []string{}
}

// IPAddressValidator validates IP addresses (IPv4/IPv6)
type IPAddressValidator struct {
	FieldName     string
	AllowIPv4     bool
	AllowIPv6     bool
	AllowLoopback bool
	name          string
}

func NewIPAddressValidator(name, fieldName string, allowIPv4, allowIPv6, allowLoopback bool) *IPAddressValidator {
	return &IPAddressValidator{
		FieldName:     fieldName,
		AllowIPv4:     allowIPv4,
		AllowIPv6:     allowIPv6,
		AllowLoopback: allowLoopback,
		name:          name,
	}
}

func (v *IPAddressValidator) Validate(_ context.Context, value interface{}) ValidationResult {
	str, ok := value.(string)
	if !ok {
		return ValidationResult{
			Valid:  false,
			Errors: []error{errors.NewValidationFailed("input", "expected string input")},
		}
	}
	result := ValidationResult{Valid: true, Errors: make([]error, 0)}

	// Basic IPv4 pattern
	ipv4Regex := regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}$`)
	// Basic IPv6 pattern
	ipv6Regex := regexp.MustCompile(`^([0-9a-fA-F]{0,4}:){1,7}[0-9a-fA-F]{0,4}$`)

	isIPv4 := ipv4Regex.MatchString(str)
	isIPv6 := ipv6Regex.MatchString(str)
	isLoopback := str == "127.0.0.1" || str == "::1" || str == "localhost"

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
	return v.name
}

func (v *IPAddressValidator) Domain() string {
	return "network"
}

func (v *IPAddressValidator) Category() string {
	return "address"
}

func (v *IPAddressValidator) Priority() int {
	return 80
}

func (v *IPAddressValidator) Dependencies() []string {
	return []string{}
}
