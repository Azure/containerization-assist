package validators

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/validation/core"
)

// DeploymentValidator validates deployment configurations and results
type DeploymentValidator struct {
	*BaseValidatorImpl
	healthValidator *HealthValidator
	timeoutLimits   TimeoutLimits
}

// TimeoutLimits defines limits for deployment operations
type TimeoutLimits struct {
	MaxDeploymentTimeout time.Duration // Maximum allowed deployment timeout
	MaxHealthTimeout     time.Duration // Maximum allowed health check timeout
	DefaultTimeout       time.Duration // Default timeout for operations
}

// NewDeploymentValidator creates a new deployment validator
func NewDeploymentValidator() *DeploymentValidator {
	return &DeploymentValidator{
		BaseValidatorImpl: NewBaseValidator("deployment", "1.0.0", []string{"deployment", "kubernetes", "health"}),
		healthValidator:   NewHealthValidator(),
		timeoutLimits: TimeoutLimits{
			MaxDeploymentTimeout: 30 * time.Minute,
			MaxHealthTimeout:     10 * time.Minute,
			DefaultTimeout:       5 * time.Minute,
		},
	}
}

// WithTimeoutLimits sets custom timeout limits
func (d *DeploymentValidator) WithTimeoutLimits(limits TimeoutLimits) *DeploymentValidator {
	d.timeoutLimits = limits
	return d
}

// Validate validates deployment data
func (d *DeploymentValidator) Validate(ctx context.Context, data interface{}, options *core.ValidationOptions) *core.ValidationResult {
	startTime := time.Now()
	result := d.BaseValidatorImpl.Validate(ctx, data, options)

	switch v := data.(type) {
	case map[string]interface{}:
		d.validateDeploymentData(v, result, options)
	case DeploymentData:
		d.validateDeploymentStruct(v, result, options)
	case DeploymentArgs:
		d.validateDeploymentArgs(v, result, options)
	case DeploymentResult:
		d.validateDeploymentResult(v, result, options)
	default:
		result.AddError(&core.ValidationError{
			Code:     "INVALID_DEPLOYMENT_DATA",
			Message:  fmt.Sprintf("Expected deployment data, got %T", data),
			Type:     core.ErrTypeValidation,
			Severity: core.SeverityHigh,
		})
	}

	result.Duration = time.Since(startTime)
	return result
}

// DeploymentData represents general deployment data
type DeploymentData struct {
	ImageRef     string                 `json:"image_ref"`
	Namespace    string                 `json:"namespace"`
	AppName      string                 `json:"app_name"`
	Timeout      time.Duration          `json:"timeout"`
	WaitForReady bool                   `json:"wait_for_ready"`
	HealthCheck  bool                   `json:"health_check"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// DeploymentArgs represents deployment arguments
type DeploymentArgs struct {
	ImageRef        string        `json:"image_ref"`
	SessionID       string        `json:"session_id"`
	Namespace       string        `json:"namespace"`
	AppName         string        `json:"app_name"`
	WaitTimeout     int           `json:"wait_timeout"`
	SkipHealthCheck bool          `json:"skip_health_check"`
	ManifestPath    string        `json:"manifest_path"`
	DryRun          bool          `json:"dry_run"`
	Force           bool          `json:"force"`
	Timeout         time.Duration `json:"timeout"`
}

// DeploymentResult represents deployment result
type DeploymentResult struct {
	Success             bool                   `json:"success"`
	ImageRef            string                 `json:"image_ref"`
	Namespace           string                 `json:"namespace"`
	AppName             string                 `json:"app_name"`
	TotalDuration       time.Duration          `json:"total_duration"`
	DeploymentDuration  time.Duration          `json:"deployment_duration"`
	HealthCheckDuration time.Duration          `json:"health_check_duration"`
	GenerationDuration  time.Duration          `json:"generation_duration"`
	HealthResult        interface{}            `json:"health_result"`
	Error               error                  `json:"error"`
	Metadata            map[string]interface{} `json:"metadata"`
}

// validateDeploymentData validates general deployment data
func (d *DeploymentValidator) validateDeploymentData(data map[string]interface{}, result *core.ValidationResult, options *core.ValidationOptions) {
	// Validate image reference
	if imageRef, exists := data["image_ref"]; exists {
		if imageStr, ok := imageRef.(string); ok {
			d.validateImageRef(imageStr, "image_ref", result)
		} else {
			result.AddFieldError("image_ref", "Image reference must be a string")
		}
	} else {
		result.AddFieldError("image_ref", "Image reference is required")
	}

	// Validate namespace
	if namespace, exists := data["namespace"]; exists {
		if namespaceStr, ok := namespace.(string); ok {
			d.validateNamespace(namespaceStr, "namespace", result)
		}
	}

	// Validate app name
	if appName, exists := data["app_name"]; exists {
		if appNameStr, ok := appName.(string); ok {
			d.validateAppName(appNameStr, "app_name", result)
		}
	}

	// Validate timeout
	if timeout, exists := data["timeout"]; exists {
		d.validateTimeout(timeout, "timeout", result)
	}
}

// validateDeploymentStruct validates structured deployment data
func (d *DeploymentValidator) validateDeploymentStruct(data DeploymentData, result *core.ValidationResult, options *core.ValidationOptions) {
	// Validate image reference
	d.validateImageRef(data.ImageRef, "image_ref", result)

	// Validate namespace
	if data.Namespace != "" {
		d.validateNamespace(data.Namespace, "namespace", result)
	}

	// Validate app name
	if data.AppName != "" {
		d.validateAppName(data.AppName, "app_name", result)
	}

	// Validate timeout
	if data.Timeout > 0 {
		d.validateTimeoutDuration(data.Timeout, "timeout", result)
	}
}

// validateDeploymentArgs validates deployment arguments
func (d *DeploymentValidator) validateDeploymentArgs(args DeploymentArgs, result *core.ValidationResult, options *core.ValidationOptions) {
	// Required fields
	if args.ImageRef == "" {
		result.AddFieldError("image_ref", "Image reference is required")
	} else {
		d.validateImageRef(args.ImageRef, "image_ref", result)
	}

	if args.SessionID == "" {
		result.AddFieldError("session_id", "Session ID is required")
	}

	// Optional but validated fields
	if args.Namespace != "" {
		d.validateNamespace(args.Namespace, "namespace", result)
	}

	if args.AppName != "" {
		d.validateAppName(args.AppName, "app_name", result)
	}

	// Validate timeout
	if args.WaitTimeout > 0 {
		timeout := time.Duration(args.WaitTimeout) * time.Second
		d.validateTimeoutDuration(timeout, "wait_timeout", result)
	}

	if args.Timeout > 0 {
		d.validateTimeoutDuration(args.Timeout, "timeout", result)
	}

	// Validate manifest path if provided
	if args.ManifestPath != "" {
		d.validateManifestPath(args.ManifestPath, "manifest_path", result)
	}

	// Logical validations
	if args.SkipHealthCheck && args.WaitTimeout > 0 {
		result.AddWarning(&core.ValidationWarning{
			ValidationError: &core.ValidationError{
				Code:     "CONFLICTING_OPTIONS",
				Message:  "Skip health check is enabled but wait timeout is set",
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityMedium,
			},
		})
	}
}

// validateDeploymentResult validates deployment result
func (d *DeploymentValidator) validateDeploymentResult(result DeploymentResult, validationResult *core.ValidationResult, options *core.ValidationOptions) {
	// Validate durations are reasonable
	if result.TotalDuration > d.timeoutLimits.MaxDeploymentTimeout {
		validationResult.AddWarning(&core.ValidationWarning{
			ValidationError: &core.ValidationError{
				Code:     "LONG_DEPLOYMENT_DURATION",
				Message:  fmt.Sprintf("Deployment took %v, which exceeds recommended maximum %v", result.TotalDuration, d.timeoutLimits.MaxDeploymentTimeout),
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityMedium,
				Field:    "total_duration",
			},
		})
	}

	// Validate consistency
	if result.Success && result.Error != nil {
		validationResult.AddError(&core.ValidationError{
			Code:     "INCONSISTENT_RESULT",
			Message:  "Deployment marked as successful but error is present",
			Type:     core.ErrTypeValidation,
			Severity: core.SeverityHigh,
		})
	}

	if !result.Success && result.Error == nil {
		validationResult.AddWarning(&core.ValidationWarning{
			ValidationError: &core.ValidationError{
				Code:     "MISSING_ERROR_DETAILS",
				Message:  "Deployment failed but no error details provided",
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityMedium,
			},
		})
	}

	// Validate health check duration
	if result.HealthCheckDuration > d.timeoutLimits.MaxHealthTimeout {
		validationResult.AddWarning(&core.ValidationWarning{
			ValidationError: &core.ValidationError{
				Code:     "LONG_HEALTH_CHECK",
				Message:  fmt.Sprintf("Health check took %v, which exceeds recommended maximum %v", result.HealthCheckDuration, d.timeoutLimits.MaxHealthTimeout),
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityMedium,
				Field:    "health_check_duration",
			},
		})
	}

	// Validate health result if present
	if result.HealthResult != nil {
		healthValidationResult := d.healthValidator.Validate(context.Background(), result.HealthResult, options)
		validationResult.Merge(healthValidationResult)
	}
}

// validateImageRef validates image reference format
func (d *DeploymentValidator) validateImageRef(imageRef, field string, result *core.ValidationResult) {
	if imageRef == "" {
		result.AddFieldError(field, "Image reference cannot be empty")
		return
	}

	// Basic image reference validation
	if !strings.Contains(imageRef, ":") {
		result.AddWarning(&core.ValidationWarning{
			ValidationError: &core.ValidationError{
				Code:     "MISSING_IMAGE_TAG",
				Message:  "Image reference should include a tag",
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityLow,
				Field:    field,
			},
		})
	}

	// Check for latest tag
	if strings.HasSuffix(imageRef, ":latest") {
		result.AddWarning(&core.ValidationWarning{
			ValidationError: &core.ValidationError{
				Code:     "LATEST_TAG_WARNING",
				Message:  "Using 'latest' tag is not recommended for production deployments",
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityLow,
				Field:    field,
			},
		})
	}

	// Validate format (basic registry/image:tag)
	parts := strings.Split(imageRef, ":")
	if len(parts) > 2 {
		result.AddWarning(&core.ValidationWarning{
			ValidationError: &core.ValidationError{
				Code:     "COMPLEX_IMAGE_REF",
				Message:  "Image reference has complex format - ensure it's correct",
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityLow,
				Field:    field,
			},
		})
	}
}

// validateNamespace validates Kubernetes namespace
func (d *DeploymentValidator) validateNamespace(namespace, field string, result *core.ValidationResult) {
	if namespace == "" {
		return // Empty namespace is allowed (uses default)
	}

	// Kubernetes namespace validation rules
	if len(namespace) > 63 {
		result.AddFieldError(field, "Namespace cannot exceed 63 characters")
	}

	// Check valid characters (lowercase alphanumeric and hyphens)
	for i, char := range namespace {
		if !((char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-') {
			result.AddError(&core.ValidationError{
				Code:     "INVALID_NAMESPACE_CHARACTER",
				Message:  fmt.Sprintf("Invalid character '%c' at position %d in namespace", char, i),
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityHigh,
				Field:    field,
			})
			break
		}
	}

	// Cannot start or end with hyphen
	if strings.HasPrefix(namespace, "-") || strings.HasSuffix(namespace, "-") {
		result.AddFieldError(field, "Namespace cannot start or end with hyphen")
	}

	// Reserved namespaces
	reservedNamespaces := []string{"kube-system", "kube-public", "kube-node-lease"}
	for _, reserved := range reservedNamespaces {
		if namespace == reserved {
			result.AddWarning(&core.ValidationWarning{
				ValidationError: &core.ValidationError{
					Code:     "RESERVED_NAMESPACE",
					Message:  fmt.Sprintf("Using reserved namespace '%s' is not recommended", namespace),
					Type:     core.ErrTypeValidation,
					Severity: core.SeverityMedium,
					Field:    field,
				},
			})
		}
	}
}

// validateAppName validates application name
func (d *DeploymentValidator) validateAppName(appName, field string, result *core.ValidationResult) {
	if appName == "" {
		return // Empty app name might be allowed depending on context
	}

	// Similar to namespace validation
	if len(appName) > 63 {
		result.AddFieldError(field, "App name cannot exceed 63 characters")
	}

	// Check valid characters
	for i, char := range appName {
		if !((char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-') {
			result.AddError(&core.ValidationError{
				Code:     "INVALID_APP_NAME_CHARACTER",
				Message:  fmt.Sprintf("Invalid character '%c' at position %d in app name", char, i),
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityHigh,
				Field:    field,
			})
			break
		}
	}

	// Cannot start or end with hyphen
	if strings.HasPrefix(appName, "-") || strings.HasSuffix(appName, "-") {
		result.AddFieldError(field, "App name cannot start or end with hyphen")
	}
}

// validateTimeout validates timeout value from interface
func (d *DeploymentValidator) validateTimeout(timeout interface{}, field string, result *core.ValidationResult) {
	switch v := timeout.(type) {
	case int:
		if v <= 0 {
			result.AddFieldError(field, "Timeout must be positive")
		} else if time.Duration(v)*time.Second > d.timeoutLimits.MaxDeploymentTimeout {
			result.AddWarning(&core.ValidationWarning{
				ValidationError: &core.ValidationError{
					Code:     "EXCESSIVE_TIMEOUT",
					Message:  fmt.Sprintf("Timeout %ds exceeds recommended maximum %v", v, d.timeoutLimits.MaxDeploymentTimeout),
					Type:     core.ErrTypeValidation,
					Severity: core.SeverityMedium,
					Field:    field,
				},
			})
		}
	case time.Duration:
		d.validateTimeoutDuration(v, field, result)
	case string:
		if duration, err := time.ParseDuration(v); err != nil {
			result.AddFieldError(field, fmt.Sprintf("Invalid timeout duration format: %v", err))
		} else {
			d.validateTimeoutDuration(duration, field, result)
		}
	default:
		result.AddFieldError(field, "Timeout must be number (seconds), duration string, or time.Duration")
	}
}

// validateTimeoutDuration validates timeout duration
func (d *DeploymentValidator) validateTimeoutDuration(timeout time.Duration, field string, result *core.ValidationResult) {
	if timeout <= 0 {
		result.AddFieldError(field, "Timeout must be positive")
	} else if timeout > d.timeoutLimits.MaxDeploymentTimeout {
		result.AddWarning(&core.ValidationWarning{
			ValidationError: &core.ValidationError{
				Code:     "EXCESSIVE_TIMEOUT",
				Message:  fmt.Sprintf("Timeout %v exceeds recommended maximum %v", timeout, d.timeoutLimits.MaxDeploymentTimeout),
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityMedium,
				Field:    field,
			},
		})
	}
}

// validateManifestPath validates manifest file path
func (d *DeploymentValidator) validateManifestPath(path, field string, result *core.ValidationResult) {
	if path == "" {
		return
	}

	// Check for absolute vs relative path
	if strings.HasPrefix(path, "/") {
		result.AddWarning(&core.ValidationWarning{
			ValidationError: &core.ValidationError{
				Code:     "ABSOLUTE_PATH_WARNING",
				Message:  "Using absolute path for manifests - ensure it's accessible",
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityLow,
				Field:    field,
			},
		})
	}

	// Check for dangerous path patterns
	dangerousPatterns := []string{"../", "./", "~/", "~\\"}
	for _, pattern := range dangerousPatterns {
		if strings.Contains(path, pattern) {
			result.AddWarning(&core.ValidationWarning{
				ValidationError: &core.ValidationError{
					Code:     "POTENTIALLY_UNSAFE_PATH",
					Message:  fmt.Sprintf("Path contains potentially unsafe pattern '%s'", pattern),
					Type:     core.ErrTypeValidation,
					Severity: core.SeverityMedium,
					Field:    field,
				},
			})
			break
		}
	}
}

// ValidateDeploymentConfig validates deployment configuration
func ValidateDeploymentConfig(config map[string]interface{}) *core.ValidationResult {
	validator := NewDeploymentValidator()
	ctx := context.Background()
	options := core.NewValidationOptions()

	return validator.Validate(ctx, config, options)
}

// ValidateDeploymentArgs validates deployment arguments
func ValidateDeploymentArgs(args DeploymentArgs) *core.ValidationResult {
	validator := NewDeploymentValidator()
	ctx := context.Background()
	options := core.NewValidationOptions()

	return validator.Validate(ctx, args, options)
}

// ValidateDeploymentResult validates deployment result
func ValidateDeploymentResult(result DeploymentResult) *core.ValidationResult {
	validator := NewDeploymentValidator()
	ctx := context.Background()
	options := core.NewValidationOptions()

	return validator.Validate(ctx, result, options)
}
