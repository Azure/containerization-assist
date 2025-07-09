package validation

import (
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// CustomValidator defines a custom validation function
type CustomValidator func(value interface{}, params map[string]string) error

// CustomValidatorRegistry holds all custom validators
type CustomValidatorRegistry struct {
	validators map[string]CustomValidator
}

// NewCustomValidatorRegistry creates a new registry with default validators
func NewCustomValidatorRegistry() *CustomValidatorRegistry {
	registry := &CustomValidatorRegistry{
		validators: make(map[string]CustomValidator),
	}

	// Register default custom validators
	registry.registerDefaultValidators()
	return registry
}

// RegisterValidator registers a custom validator
func (r *CustomValidatorRegistry) RegisterValidator(name string, validator CustomValidator) {
	r.validators[name] = validator
}

// GetValidator retrieves a custom validator by name
func (r *CustomValidatorRegistry) GetValidator(name string) (CustomValidator, bool) {
	validator, exists := r.validators[name]
	return validator, exists
}

// ValidateWithCustom validates a value using a custom validator
func (r *CustomValidatorRegistry) ValidateWithCustom(name string, value interface{}, params map[string]string) error {
	validator, exists := r.validators[name]
	if !exists {
		return errors.NewError().
			Code(errors.CodeNotFound).
			Type(errors.ErrTypeValidation).
			Severity(errors.SeverityMedium).
			Messagef("custom validator '%s' not found", name).
			Context("validator_name", name).
			Suggestion("Check that the validator is registered or use a built-in validator").
			WithLocation().
			Build()
	}

	return validator(value, params)
}

// registerDefaultValidators registers all default custom validators
func (r *CustomValidatorRegistry) registerDefaultValidators() {
	// Docker/Container validators
	r.RegisterValidator("image_name", validateImageName)
	r.RegisterValidator("tag_format", validateTagFormat)
	r.RegisterValidator("platform_format", validatePlatformFormat)
	r.RegisterValidator("dockerfile_syntax", validateDockerfileSyntax)

	// Kubernetes validators
	r.RegisterValidator("k8s_name", validateK8sName)
	r.RegisterValidator("k8s_labels", validateK8sLabels)
	r.RegisterValidator("k8s_resource", validateK8sResource)
	r.RegisterValidator("k8s_selector", validateK8sSelector)

	// File/Path validators
	r.RegisterValidator("file_exists", validateFileExists)
	r.RegisterValidator("dir_exists", validateDirExists)
	r.RegisterValidator("abs_path", validateAbsPath)
	r.RegisterValidator("rel_path", validateRelPath)

	// Network validators
	r.RegisterValidator("port", validatePort)
	r.RegisterValidator("cidr", validateCIDR)
	r.RegisterValidator("ipv4", validateIPv4)
	r.RegisterValidator("ipv6", validateIPv6)

	// Additional validators
	r.RegisterValidator("base64", validateBase64)
	r.RegisterValidator("hexadecimal", validateHexadecimal)
	r.RegisterValidator("uuid4", validateUUID4)
}

// Docker/Container Validators

// validateImageName validates Docker image name format
func validateImageName(value interface{}, params map[string]string) error {
	str, ok := value.(string)
	if !ok {
		return errors.NewError().
			Code(errors.CodeInvalidType).
			Type(errors.ErrTypeValidation).
			Message("image_name validator requires string value").
			WithLocation().
			Build()
	}

	// Docker image name regex: [hostname[:port]/]username/repository[:tag]
	// Simplified regex for common cases
	imageRegex := regexp.MustCompile(`^([a-z0-9]+([._-][a-z0-9]+)*\/)*[a-z0-9]+([._-][a-z0-9]+)*(:[a-zA-Z0-9._-]+)?$`)

	if !imageRegex.MatchString(str) {
		return errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Message("invalid Docker image name format").
			Context("value", str).
			Suggestion("Use format: [registry/]namespace/repository[:tag]").
			WithLocation().
			Build()
	}

	return nil
}

// validateTagFormat validates Docker tag format
func validateTagFormat(value interface{}, params map[string]string) error {
	str, ok := value.(string)
	if !ok {
		return errors.NewError().
			Code(errors.CodeInvalidType).
			Type(errors.ErrTypeValidation).
			Message("tag_format validator requires string value").
			WithLocation().
			Build()
	}

	// Docker tag must be valid ASCII and cannot contain uppercase letters
	tagRegex := regexp.MustCompile(`^[a-z0-9._-]+$`)

	if !tagRegex.MatchString(str) {
		return errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Message("invalid Docker tag format").
			Context("value", str).
			Suggestion("Use lowercase letters, numbers, dots, dashes, and underscores only").
			WithLocation().
			Build()
	}

	return nil
}

// validatePlatformFormat validates platform string format
func validatePlatformFormat(value interface{}, params map[string]string) error {
	str, ok := value.(string)
	if !ok {
		return errors.NewError().
			Code(errors.CodeInvalidType).
			Type(errors.ErrTypeValidation).
			Message("platform_format validator requires string value").
			WithLocation().
			Build()
	}

	// Platform format: os/architecture[/variant]
	platformRegex := regexp.MustCompile(`^[a-z]+\/[a-z0-9]+(\/.+)?$`)

	if !platformRegex.MatchString(str) {
		return errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Message("invalid platform format").
			Context("value", str).
			Suggestion("Use format: os/architecture (e.g., linux/amd64, windows/amd64)").
			WithLocation().
			Build()
	}

	return nil
}

// validateDockerfileSyntax validates basic Dockerfile syntax
func validateDockerfileSyntax(value interface{}, params map[string]string) error {
	str, ok := value.(string)
	if !ok {
		return errors.NewError().
			Code(errors.CodeInvalidType).
			Type(errors.ErrTypeValidation).
			Message("dockerfile_syntax validator requires string value").
			WithLocation().
			Build()
	}

	// Check if file exists first
	if _, err := os.Stat(str); os.IsNotExist(err) {
		return errors.NewError().
			Code(errors.CodeFileNotFound).
			Type(errors.ErrTypeValidation).
			Message("Dockerfile not found").
			Context("path", str).
			WithLocation().
			Build()
	}

	// Basic syntax validation would go here
	// For now, just check file extension
	if !strings.HasSuffix(strings.ToLower(str), "dockerfile") && !strings.Contains(strings.ToLower(str), "dockerfile") {
		return errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Message("invalid Dockerfile name").
			Context("path", str).
			Suggestion("Dockerfile should be named 'Dockerfile' or contain 'dockerfile' in the name").
			WithLocation().
			Build()
	}

	return nil
}

// Kubernetes Validators

// validateK8sName validates Kubernetes resource name (RFC 1123 DNS subdomain)
func validateK8sName(value interface{}, params map[string]string) error {
	str, ok := value.(string)
	if !ok {
		return errors.NewError().
			Code(errors.CodeInvalidType).
			Type(errors.ErrTypeValidation).
			Message("k8s_name validator requires string value").
			WithLocation().
			Build()
	}

	// RFC 1123 DNS subdomain: lowercase letters, numbers, hyphens
	// Must start and end with alphanumeric character
	k8sNameRegex := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

	if len(str) > 253 {
		return errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Message("Kubernetes name too long").
			Context("value", str).
			Context("length", len(str)).
			Suggestion("Kubernetes names must be 253 characters or less").
			WithLocation().
			Build()
	}

	if !k8sNameRegex.MatchString(str) {
		return errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Message("invalid Kubernetes name format").
			Context("value", str).
			Suggestion("Use lowercase letters, numbers, and hyphens only. Must start and end with alphanumeric character").
			WithLocation().
			Build()
	}

	return nil
}

// validateK8sLabels validates Kubernetes labels format
func validateK8sLabels(value interface{}, params map[string]string) error {
	labels, ok := value.(map[string]string)
	if !ok {
		return errors.NewError().
			Code(errors.CodeInvalidType).
			Type(errors.ErrTypeValidation).
			Message("k8s_labels validator requires map[string]string value").
			WithLocation().
			Build()
	}

	for key, val := range labels {
		// Validate key
		if err := validateK8sLabelKey(key); err != nil {
			return err
		}

		// Validate value
		if err := validateK8sLabelValue(val); err != nil {
			return err
		}
	}

	return nil
}

func validateK8sLabelKey(key string) error {
	if len(key) > 63 {
		return errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Message("Kubernetes label key too long").
			Context("key", key).
			Context("length", len(key)).
			WithLocation().
			Build()
	}

	labelKeyRegex := regexp.MustCompile(`^[a-z0-9A-Z]([-a-z0-9A-Z_.]*[a-z0-9A-Z])?$`)
	if !labelKeyRegex.MatchString(key) {
		return errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Message("invalid Kubernetes label key format").
			Context("key", key).
			WithLocation().
			Build()
	}

	return nil
}

func validateK8sLabelValue(value string) error {
	if len(value) > 63 {
		return errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Message("Kubernetes label value too long").
			Context("value", value).
			Context("length", len(value)).
			WithLocation().
			Build()
	}

	labelValueRegex := regexp.MustCompile(`^[a-z0-9A-Z]([-a-z0-9A-Z_.]*[a-z0-9A-Z])?$`)
	if !labelValueRegex.MatchString(value) {
		return errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Message("invalid Kubernetes label value format").
			Context("value", value).
			WithLocation().
			Build()
	}

	return nil
}

// validateK8sResource validates Kubernetes resource quantity
func validateK8sResource(value interface{}, params map[string]string) error {
	str, ok := value.(string)
	if !ok {
		return errors.NewError().
			Code(errors.CodeInvalidType).
			Type(errors.ErrTypeValidation).
			Message("k8s_resource validator requires string value").
			WithLocation().
			Build()
	}

	// Basic resource quantity regex (simplified)
	resourceRegex := regexp.MustCompile(`^[0-9]+(\.[0-9]+)?[mMgGtTkK]?i?$`)

	if !resourceRegex.MatchString(str) {
		return errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Message("invalid Kubernetes resource quantity format").
			Context("value", str).
			Suggestion("Use format like '100m', '1Gi', '500Mi', '2'").
			WithLocation().
			Build()
	}

	return nil
}

// validateK8sSelector validates label selector format
func validateK8sSelector(value interface{}, params map[string]string) error {
	str, ok := value.(string)
	if !ok {
		return errors.NewError().
			Code(errors.CodeInvalidType).
			Type(errors.ErrTypeValidation).
			Message("k8s_selector validator requires string value").
			WithLocation().
			Build()
	}

	// Basic selector validation (simplified)
	if strings.TrimSpace(str) == "" {
		return errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Message("empty label selector").
			WithLocation().
			Build()
	}

	return nil
}

// File/Path Validators

// validateFileExists validates that a file exists
func validateFileExists(value interface{}, params map[string]string) error {
	str, ok := value.(string)
	if !ok {
		return errors.NewError().
			Code(errors.CodeInvalidType).
			Type(errors.ErrTypeValidation).
			Message("file_exists validator requires string value").
			WithLocation().
			Build()
	}

	if _, err := os.Stat(str); os.IsNotExist(err) {
		return errors.NewError().
			Code(errors.CodeFileNotFound).
			Type(errors.ErrTypeValidation).
			Message("file does not exist").
			Context("path", str).
			WithLocation().
			Build()
	}

	return nil
}

// validateDirExists validates that a directory exists
func validateDirExists(value interface{}, params map[string]string) error {
	str, ok := value.(string)
	if !ok {
		return errors.NewError().
			Code(errors.CodeInvalidType).
			Type(errors.ErrTypeValidation).
			Message("dir_exists validator requires string value").
			WithLocation().
			Build()
	}

	stat, err := os.Stat(str)
	if os.IsNotExist(err) {
		return errors.NewError().
			Code(errors.CodeFileNotFound).
			Type(errors.ErrTypeValidation).
			Message("directory does not exist").
			Context("path", str).
			WithLocation().
			Build()
	}

	if !stat.IsDir() {
		return errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Message("path is not a directory").
			Context("path", str).
			WithLocation().
			Build()
	}

	return nil
}

// validateAbsPath validates absolute path format
func validateAbsPath(value interface{}, params map[string]string) error {
	str, ok := value.(string)
	if !ok {
		return errors.NewError().
			Code(errors.CodeInvalidType).
			Type(errors.ErrTypeValidation).
			Message("abs_path validator requires string value").
			WithLocation().
			Build()
	}

	if !filepath.IsAbs(str) {
		return errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Message("path is not absolute").
			Context("path", str).
			WithLocation().
			Build()
	}

	return nil
}

// validateRelPath validates relative path format
func validateRelPath(value interface{}, params map[string]string) error {
	str, ok := value.(string)
	if !ok {
		return errors.NewError().
			Code(errors.CodeInvalidType).
			Type(errors.ErrTypeValidation).
			Message("rel_path validator requires string value").
			WithLocation().
			Build()
	}

	if filepath.IsAbs(str) {
		return errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Message("path is not relative").
			Context("path", str).
			WithLocation().
			Build()
	}

	return nil
}

// Network Validators

// validatePort validates port number
func validatePort(value interface{}, params map[string]string) error {
	var port int64
	var err error

	switch v := value.(type) {
	case int:
		port = int64(v)
	case int32:
		port = int64(v)
	case int64:
		port = v
	case string:
		port, err = strconv.ParseInt(v, 10, 64)
		if err != nil {
			return errors.NewError().
				Code(errors.CodeInvalidParameter).
				Type(errors.ErrTypeValidation).
				Message("invalid port number format").
				Context("value", v).
				WithLocation().
				Build()
		}
	default:
		return errors.NewError().
			Code(errors.CodeInvalidType).
			Type(errors.ErrTypeValidation).
			Message("port validator requires numeric value").
			WithLocation().
			Build()
	}

	if port < 1 || port > 65535 {
		return errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Message("port number out of valid range").
			Context("value", port).
			Suggestion("Port must be between 1 and 65535").
			WithLocation().
			Build()
	}

	return nil
}

// validateCIDR validates CIDR notation
func validateCIDR(value interface{}, params map[string]string) error {
	str, ok := value.(string)
	if !ok {
		return errors.NewError().
			Code(errors.CodeInvalidType).
			Type(errors.ErrTypeValidation).
			Message("cidr validator requires string value").
			WithLocation().
			Build()
	}

	_, _, err := net.ParseCIDR(str)
	if err != nil {
		return errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Message("invalid CIDR notation").
			Context("value", str).
			Cause(err).
			WithLocation().
			Build()
	}

	return nil
}

// validateIPv4 validates IPv4 address
func validateIPv4(value interface{}, params map[string]string) error {
	str, ok := value.(string)
	if !ok {
		return errors.NewError().
			Code(errors.CodeInvalidType).
			Type(errors.ErrTypeValidation).
			Message("ipv4 validator requires string value").
			WithLocation().
			Build()
	}

	ip := net.ParseIP(str)
	if ip == nil || ip.To4() == nil {
		return errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Message("invalid IPv4 address").
			Context("value", str).
			WithLocation().
			Build()
	}

	return nil
}

// validateIPv6 validates IPv6 address
func validateIPv6(value interface{}, params map[string]string) error {
	str, ok := value.(string)
	if !ok {
		return errors.NewError().
			Code(errors.CodeInvalidType).
			Type(errors.ErrTypeValidation).
			Message("ipv6 validator requires string value").
			WithLocation().
			Build()
	}

	ip := net.ParseIP(str)
	if ip == nil || ip.To4() != nil {
		return errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Message("invalid IPv6 address").
			Context("value", str).
			WithLocation().
			Build()
	}

	return nil
}

// Additional Validators

// validateBase64 validates base64 encoding
func validateBase64(value interface{}, params map[string]string) error {
	str, ok := value.(string)
	if !ok {
		return errors.NewError().
			Code(errors.CodeInvalidType).
			Type(errors.ErrTypeValidation).
			Message("base64 validator requires string value").
			WithLocation().
			Build()
	}

	base64Regex := regexp.MustCompile(`^[A-Za-z0-9+/]*={0,2}$`)
	if !base64Regex.MatchString(str) {
		return errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Message("invalid base64 format").
			Context("value", str).
			WithLocation().
			Build()
	}

	return nil
}

// validateHexadecimal validates hexadecimal string
func validateHexadecimal(value interface{}, params map[string]string) error {
	str, ok := value.(string)
	if !ok {
		return errors.NewError().
			Code(errors.CodeInvalidType).
			Type(errors.ErrTypeValidation).
			Message("hexadecimal validator requires string value").
			WithLocation().
			Build()
	}

	hexRegex := regexp.MustCompile(`^[0-9a-fA-F]+$`)
	if !hexRegex.MatchString(str) {
		return errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Message("invalid hexadecimal format").
			Context("value", str).
			WithLocation().
			Build()
	}

	return nil
}

// validateUUID4 validates UUID v4 format
func validateUUID4(value interface{}, params map[string]string) error {
	str, ok := value.(string)
	if !ok {
		return errors.NewError().
			Code(errors.CodeInvalidType).
			Type(errors.ErrTypeValidation).
			Message("uuid4 validator requires string value").
			WithLocation().
			Build()
	}

	uuid4Regex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if !uuid4Regex.MatchString(strings.ToLower(str)) {
		return errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Message("invalid UUID v4 format").
			Context("value", str).
			WithLocation().
			Build()
	}

	return nil
}

// Global registry instance
var DefaultCustomValidatorRegistry = NewCustomValidatorRegistry()
