package validation

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/common/interfaces"
	"github.com/Azure/container-kit/pkg/common/validation-core/core"
	"github.com/Azure/container-kit/pkg/mcp/application/api"
	validation "github.com/Azure/container-kit/pkg/mcp/domain/security"
)

// ============================================================================
// WORKSTREAM DELTA: Unified Validation Framework
// Consolidates 25+ validator interfaces into 1 unified validator (96% reduction)
//
// DEPRECATED: This implementation uses reflection and should be replaced with
// the type-safe validation framework in pkg/common/validation-core.
// See generic_validator.go for the new approach that avoids reflection overhead.
// ============================================================================

// UnifiedValidatorImpl implements the UnifiedValidator interface from unified_interfaces.go
// Deprecated: Use validators from pkg/common/validation-core instead
type UnifiedValidatorImpl struct {
	rules         map[string]*UnifiedValidationRule
	policies      map[string]ValidationPolicy
	observability interfaces.UnifiedObservability
	config        ValidatorConfig
}

// ValidatorConfig configures the unified validator
type ValidatorConfig struct {
	EnableStrictMode       bool
	EnableAsyncValidation  bool
	MaxValidationTime      time.Duration
	ValidationLevel        ValidationLevel
	CacheValidationResults bool
}

// ValidationLevel defines the strictness of validation
type ValidationLevel int

const (
	ValidationLevelBasic ValidationLevel = iota
	ValidationLevelStandard
	ValidationLevelStrict
	ValidationLevelParanoid
)

// ValidationPolicy groups multiple rules for a specific use case
type ValidationPolicy struct {
	Name        string
	Description string
	Rules       []string
	Required    bool
	Context     map[string]interface{}
}

// ValidationResult is now an alias to core.NonGenericResult for consistency
type ValidationResult = core.NonGenericResult

// ValidationWarning is now an alias to core.Warning for consistency
type ValidationWarning = core.Warning

// NewUnifiedValidator creates a new unified validator
func NewUnifiedValidator(capabilities []string) interfaces.UnifiedValidator {
	config := ValidatorConfig{
		EnableStrictMode:       contains(capabilities, "business_rules"),
		EnableAsyncValidation:  contains(capabilities, "async_validation"),
		MaxValidationTime:      30 * time.Second,
		ValidationLevel:        ValidationLevelStandard,
		CacheValidationResults: contains(capabilities, "validation_caching"),
	}

	validator := &UnifiedValidatorImpl{
		rules:    make(map[string]*UnifiedValidationRule),
		policies: make(map[string]ValidationPolicy),
		config:   config,
	}

	// Register common validation rules
	validator.registerCommonRules()

	return validator
}

// ValidateInput validates tool input (consolidated from 25+ tool-specific validators)
// Deprecated: Use type-safe validators from pkg/common/validation-core instead of this reflection-based approach
func (v *UnifiedValidatorImpl) ValidateInput(ctx context.Context, toolName string, input api.ToolInput) error {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		if v.observability != nil {
			v.observability.RecordMetric(ctx, "validation_input_duration_microseconds", float64(duration.Microseconds()), map[string]string{
				"tool": toolName,
			})
		}
	}()

	// Get validation policy for tool
	policyName := fmt.Sprintf("tool_%s_input", toolName)
	policy, exists := v.policies[policyName]
	if !exists {
		// Use default input validation policy
		policy = v.getDefaultInputPolicy()
	}

	// Execute validation rules
	result := v.executePolicy(ctx, policy, input)
	if !result.Valid {
		return v.formatValidationError(result)
	}

	return nil
}

// ValidateOutput validates tool output (consolidated from 25+ tool-specific validators)
func (v *UnifiedValidatorImpl) ValidateOutput(ctx context.Context, toolName string, output api.ToolOutput) error {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		if v.observability != nil {
			v.observability.RecordMetric(ctx, "validation_output_duration_microseconds", float64(duration.Microseconds()), map[string]string{
				"tool": toolName,
			})
		}
	}()

	// Get validation policy for tool output
	policyName := fmt.Sprintf("tool_%s_output", toolName)
	policy, exists := v.policies[policyName]
	if !exists {
		// Use default output validation policy
		policy = v.getDefaultOutputPolicy()
	}

	// Execute validation rules
	result := v.executePolicy(ctx, policy, output)
	if !result.Valid {
		return v.formatValidationError(result)
	}

	return nil
}

// ValidateConfig validates configuration objects (consolidated from multiple config validators)
func (v *UnifiedValidatorImpl) ValidateConfig(ctx context.Context, config interface{}) error {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		if v.observability != nil {
			v.observability.RecordMetric(ctx, "validation_config_duration_microseconds", float64(duration.Microseconds()), map[string]string{
				"config_type": reflect.TypeOf(config).String(),
			})
		}
	}()

	// Execute configuration validation rules
	policy := v.getConfigValidationPolicy()
	result := v.executePolicy(ctx, policy, config)
	if !result.Valid {
		return v.formatValidationError(result)
	}

	return nil
}

// ValidateSchema validates schema objects (consolidated from schema validators)
func (v *UnifiedValidatorImpl) ValidateSchema(ctx context.Context, schema interface{}, data interface{}) error {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		if v.observability != nil {
			v.observability.RecordMetric(ctx, "validation_schema_duration_microseconds", float64(duration.Microseconds()), map[string]string{
				"schema_type": reflect.TypeOf(schema).String(),
			})
		}
	}()

	// Execute schema validation
	policy := v.getSchemaValidationPolicy()
	validationInput := map[string]interface{}{
		"schema": schema,
		"data":   data,
	}

	result := v.executePolicy(ctx, policy, validationInput)
	if !result.Valid {
		return v.formatValidationError(result)
	}

	return nil
}

// ValidateHealth performs health validation (consolidated from health validators)
func (v *UnifiedValidatorImpl) ValidateHealth(ctx context.Context) []interfaces.ValidationResult {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		if v.observability != nil {
			v.observability.RecordMetric(ctx, "validation_health_duration_microseconds", float64(duration.Microseconds()), nil)
		}
	}()

	var results []interfaces.ValidationResult

	// Execute health validation rules
	healthPolicy := v.getHealthValidationPolicy()
	result := v.executePolicy(ctx, healthPolicy, nil)

	// Convert core.Result to interfaces.ValidationResult format
	message := "Validation successful"
	severity := "info"
	if !result.Valid {
		message = "Validation failed"
		severity = "error"
		if len(result.Errors) > 0 {
			message = result.Errors[0].Message
		}
	}

	mcpResult := interfaces.ValidationResult{
		Valid:    result.Valid,
		Message:  message,
		Severity: severity,
		Context:  result.Metadata.Context,
	}

	results = append(results, mcpResult)
	return results
}

// ============================================================================
// Internal Implementation Methods
// ============================================================================

// executePolicy executes a validation policy against input data
func (v *UnifiedValidatorImpl) executePolicy(ctx context.Context, policy ValidationPolicy, data interface{}) *core.NonGenericResult {
	result := core.NewNonGenericResult("unified_validator", "1.0")

	start := time.Now()
	defer func() {
		result.Duration = time.Since(start)
	}()

	// Execute each rule in the policy
	for _, ruleName := range policy.Rules {
		rule, exists := v.rules[ruleName]
		if !exists {
			err := core.NewError("UNKNOWN_RULE", fmt.Sprintf("Unknown validation rule: %s", ruleName), core.ErrTypeValidation, core.SeverityHigh)
			err.WithField("policy")
			result.AddError(err)
			continue
		}

		// Execute rule using the validator function
		if rule.Validator != nil {
			ruleResult := rule.Validator(ctx, data)
			if ruleResult != nil && !ruleResult.Valid {
				// Merge rule result into main result
				result.Merge(ruleResult)
			}
		}

		// Stop on first error if required
		if policy.Required && result.HasErrors() {
			break
		}
	}

	return result
}

// UnifiedValidationRule extends framework ValidationRule with a validator function
type UnifiedValidationRule struct {
	Name        string
	Description string
	Category    string
	Severity    core.ErrorSeverity
	Validator   func(ctx context.Context, value interface{}) *core.NonGenericResult
}

// registerCommonRules registers commonly used validation rules
func (v *UnifiedValidatorImpl) registerCommonRules() {
	// Not null validation rule
	v.rules["not_null"] = &UnifiedValidationRule{
		Name:        "not_null",
		Description: "Validates that value is not null or empty",
		Category:    "basic",
		Severity:    core.SeverityHigh,
		Validator: func(ctx context.Context, value interface{}) *core.NonGenericResult {
			result := core.NewNonGenericResult("not_null", "1.0")

			if value == nil {
				err := core.NewError("NULL_VALUE", "Value cannot be null", core.ErrTypeValidation, core.SeverityHigh)
				result.AddError(err)
				return result
			}

			// Check for empty strings
			if str, ok := value.(string); ok && strings.TrimSpace(str) == "" {
				err := core.NewError("EMPTY_VALUE", "Value cannot be empty", core.ErrTypeValidation, core.SeverityHigh)
				result.AddError(err)
				return result
			}

			return result
		},
	}

	// String length validation rule
	v.rules["string_length"] = &UnifiedValidationRule{
		Name:        "string_length",
		Description: "Validates string length constraints",
		Category:    "string",
		Severity:    core.SeverityHigh,
		Validator: func(ctx context.Context, value interface{}) *core.NonGenericResult {
			result := core.NewNonGenericResult("string_length", "1.0")

			str, ok := value.(string)
			if !ok {
				return result // Skip if not string
			}

			if len(str) > 1000 { // Max length check
				err := core.NewError("STRING_TOO_LONG", "String exceeds maximum length of 1000 characters", core.ErrTypeValidation, core.SeverityHigh)
				result.AddError(err)
			}

			return result
		},
	}

	// Security validation rule (consolidated from security validator)
	v.rules["security_check"] = &UnifiedValidationRule{
		Name:        "security_check",
		Description: "Performs basic security validation",
		Category:    "security",
		Severity:    core.SeverityCritical,
		Validator:   v.securityValidator,
	}

	// Performance validation rule
	v.rules["performance_check"] = &UnifiedValidationRule{
		Name:        "performance_check",
		Description: "Validates performance constraints",
		Category:    "performance",
		Severity:    core.SeverityMedium,
		Validator:   v.performanceValidator,
	}

	// Register default policies
	v.registerDefaultPolicies()
}

// securityValidator performs consolidated security validation
func (v *UnifiedValidatorImpl) securityValidator(ctx context.Context, value interface{}) *core.NonGenericResult {
	result := core.NewNonGenericResult("security_check", "1.0")

	// Basic security checks (consolidated from security/validator.go)
	if str, ok := value.(string); ok {
		// Check for common injection patterns
		dangerousPatterns := []string{
			"<script", "javascript:", "onload=", "onerror=",
			"../", "..\\", "/etc/passwd", "/proc/",
			"SELECT", "DROP", "INSERT", "DELETE", "UPDATE",
		}

		for _, pattern := range dangerousPatterns {
			if strings.Contains(strings.ToLower(str), strings.ToLower(pattern)) {
				err := core.NewError("SECURITY_VIOLATION", fmt.Sprintf("Potentially dangerous pattern detected: %s", pattern), core.ErrTypeSecurity, core.SeverityCritical)
				err.WithSuggestion("Remove potentially dangerous content")
				err.WithSuggestion("Use parameterized queries for database operations")
				err.WithSuggestion("Sanitize user input")
				result.AddError(err)
			}
		}
	}

	return result
}

// performanceValidator validates performance constraints
func (v *UnifiedValidatorImpl) performanceValidator(ctx context.Context, value interface{}) *core.NonGenericResult {
	result := core.NewNonGenericResult("performance_check", "1.0")

	// Check for large data structures that could impact performance
	if data, ok := value.(map[string]interface{}); ok {
		if len(data) > 100 { // Arbitrary threshold
			warning := core.NewWarning("LARGE_DATA_STRUCTURE", "Large data structure detected - may impact performance")
			warning.Error.WithSuggestion("Consider paginating large datasets")
			warning.Error.WithSuggestion("Use streaming for large data processing")
			result.AddWarning(warning)
		}
	}

	return result
}

// ============================================================================
// Default Policies
// ============================================================================

func (v *UnifiedValidatorImpl) registerDefaultPolicies() {
	// Default input validation policy
	v.policies["default_input"] = ValidationPolicy{
		Name:        "default_input",
		Description: "Default tool input validation",
		Rules:       []string{"not_null", "string_length", "security_check"},
		Required:    true,
	}

	// Default output validation policy
	v.policies["default_output"] = ValidationPolicy{
		Name:        "default_output",
		Description: "Default tool output validation",
		Rules:       []string{"not_null", "performance_check"},
		Required:    false,
	}

	// Configuration validation policy
	v.policies["config_validation"] = ValidationPolicy{
		Name:        "config_validation",
		Description: "Configuration object validation",
		Rules:       []string{"not_null", "security_check"},
		Required:    true,
	}

	// Schema validation policy
	v.policies["schema_validation"] = ValidationPolicy{
		Name:        "schema_validation",
		Description: "Schema validation",
		Rules:       []string{"not_null"},
		Required:    true,
	}

	// Health validation policy
	v.policies["health_validation"] = ValidationPolicy{
		Name:        "health_validation",
		Description: "System health validation",
		Rules:       []string{"performance_check"},
		Required:    false,
	}
}

// ============================================================================
// Helper Methods
// ============================================================================

func (v *UnifiedValidatorImpl) getDefaultInputPolicy() ValidationPolicy {
	return v.policies["default_input"]
}

func (v *UnifiedValidatorImpl) getDefaultOutputPolicy() ValidationPolicy {
	return v.policies["default_output"]
}

func (v *UnifiedValidatorImpl) getConfigValidationPolicy() ValidationPolicy {
	return v.policies["config_validation"]
}

func (v *UnifiedValidatorImpl) getSchemaValidationPolicy() ValidationPolicy {
	return v.policies["schema_validation"]
}

func (v *UnifiedValidatorImpl) getHealthValidationPolicy() ValidationPolicy {
	return v.policies["health_validation"]
}

func (v *UnifiedValidatorImpl) formatValidationError(result *core.NonGenericResult) error {
	if len(result.Errors) == 0 {
		return nil
	}

	// Create consolidated error message
	var messages []string
	for _, err := range result.Errors {
		messages = append(messages, fmt.Sprintf("[%s] %s", err.Code, err.Message))
	}

	return fmt.Errorf("validation failed: %s", strings.Join(messages, "; "))
}

// SetObservability sets the unified observability system
func (v *UnifiedValidatorImpl) SetObservability(obs interfaces.UnifiedObservability) {
	v.observability = obs
}

// Helper functions

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// convertCoreToAPISeverity converts core.ErrorSeverity to validation.ErrorSeverity
func convertCoreToAPISeverity(severity core.ErrorSeverity) validation.ErrorSeverity {
	switch severity {
	case core.SeverityCritical:
		return validation.SeverityCritical
	case core.SeverityHigh:
		return validation.SeverityHigh
	case core.SeverityMedium:
		return validation.SeverityMedium
	case core.SeverityLow:
		return validation.SeverityLow
	default:
		return validation.SeverityMedium
	}
}

func convertCoreErrorsToAPI(errors []*core.Error) []*api.ValidationError {
	var result []*api.ValidationError
	for _, err := range errors {
		// Convert core.Error to api.ValidationError
		apiErr := &api.ValidationError{
			Field:    err.Field,
			Message:  err.Message,
			Code:     err.Code,
			Value:    nil, // core.Error doesn't have Value field
			Severity: convertCoreToAPISeverity(err.Severity),
		}
		result = append(result, apiErr)
	}
	return result
}

func convertCoreWarningsToAPI(warnings []*core.Warning) []*api.ValidationWarning {
	var result []*api.ValidationWarning
	for _, warn := range warnings {
		// Create a warning using API validation structure
		apiWarn := &api.ValidationWarning{
			Field:      warn.Error.Field,
			Message:    warn.Error.Message,
			Code:       warn.Error.Code,
			Suggestion: "", // We can take the first suggestion if available
		}
		if len(warn.Error.Suggestions) > 0 {
			apiWarn.Suggestion = warn.Error.Suggestions[0]
		}
		result = append(result, apiWarn)
	}
	return result
}
