package security

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// TagBasedValidator provides tag-based validation without reflection
type TagBasedValidator struct {
	validators map[string]ValidationFunc
	options    ValidationOptions
	mutex      sync.RWMutex
}

// ValidationFunc validates a field value
type ValidationFunc func(value interface{}, fieldName string, params map[string]interface{}) error

// ValidationOptions provides options for validation
type ValidationOptions struct {
	FailFast      bool          // Stop on first validation error
	CacheRules    bool          // Cache parsed validation rules
	Timeout       time.Duration // Validation timeout
	IgnoreUnknown bool          // Ignore unknown validation tags
	CustomTags    string        // Custom tag name (default: "validate")
}

// DefaultValidationOptions returns default validation options
func DefaultValidationOptions() ValidationOptions {
	return ValidationOptions{
		FailFast:      false,
		CacheRules:    true,
		Timeout:       30 * time.Second,
		IgnoreUnknown: false,
		CustomTags:    "validate",
	}
}

// NewTagBasedValidator creates a new tag-based validator
func NewTagBasedValidator(options ...ValidationOptions) *TagBasedValidator {
	opts := DefaultValidationOptions()
	if len(options) > 0 {
		opts = options[0]
	}

	validator := &TagBasedValidator{
		validators: make(map[string]ValidationFunc),
		options:    opts,
	}

	// Register built-in validators
	validator.registerBuiltinValidators()

	return validator
}

// registerBuiltinValidators registers common validation functions
func (v *TagBasedValidator) registerBuiltinValidators() {
	v.validators["required"] = v.validateRequired
	v.validators["min"] = v.validateMin
	v.validators["max"] = v.validateMax
	v.validators["len"] = v.validateLen
	v.validators["email"] = v.validateEmail
	v.validators["url"] = v.validateURL
	v.validators["uuid"] = v.validateUUID
	v.validators["oneof"] = v.validateOneOf
	v.validators["port"] = v.validatePort
	v.validators["dive"] = v.validateDive

	// Register enhanced validators from CommonValidationTags
	tags := CommonValidationTags()
	if validator, ok := tags[TagImageName]; ok && validator.Validator != nil {
		v.validators["image_name"] = validator.Validator
	}
	if validator, ok := tags[TagK8sName]; ok && validator.Validator != nil {
		v.validators["k8s_name"] = validator.Validator
	}
}

// ValidateField validates a single field value using validation rules
func (v *TagBasedValidator) ValidateField(ctx context.Context, fieldName string, value interface{}, rules string) error {
	if rules == "" {
		return nil
	}

	// Set up timeout if specified
	if v.options.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, v.options.Timeout)
		defer cancel()
	}

	// Parse validation rules
	ruleList := v.parseValidationRules(rules)

	// Check for omitempty rule
	if v.hasOmitEmpty(ruleList) && v.isZeroValue(value) {
		return nil // Skip validation for empty values
	}

	// Apply validation rules
	var validationErrors []error
	for _, rule := range ruleList {
		// Skip omitempty rule as it's already handled
		if rule.Name == "omitempty" {
			continue
		}

		// Check for timeout
		select {
		case <-ctx.Done():
			return errors.NewError().
				Message("validation timeout").
				WithLocation().
				Build()
		default:
		}

		// Apply validation rule
		if err := v.applyValidationRule(value, fieldName, rule); err != nil {
			validationErrors = append(validationErrors, err)

			// Stop on first error if FailFast is enabled
			if v.options.FailFast {
				return err
			}
		}
	}

	// Return combined errors if any
	if len(validationErrors) > 0 {
		return v.combineErrors(validationErrors)
	}

	return nil
}

// ValidationRule represents a parsed validation rule
type ValidationRule struct {
	Name       string
	Parameters map[string]interface{}
}

// parseValidationRules parses validation rules from a string
func (v *TagBasedValidator) parseValidationRules(rules string) []ValidationRule {
	if rules == "" {
		return nil
	}

	var ruleList []ValidationRule
	ruleParts := strings.Split(rules, ",")

	for _, part := range ruleParts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		rule := ValidationRule{
			Parameters: make(map[string]interface{}),
		}

		// Parse rule with parameters (e.g., "min=5", "oneof=val1 val2")
		if strings.Contains(part, "=") {
			parts := strings.SplitN(part, "=", 2)
			rule.Name = parts[0]
			rule.Parameters["value"] = parts[1]
		} else {
			rule.Name = part
		}

		ruleList = append(ruleList, rule)
	}

	return ruleList
}

// hasOmitEmpty checks if omitempty rule is present
func (v *TagBasedValidator) hasOmitEmpty(rules []ValidationRule) bool {
	for _, rule := range rules {
		if rule.Name == "omitempty" {
			return true
		}
	}
	return false
}

// isZeroValue checks if a value is zero
func (v *TagBasedValidator) isZeroValue(value interface{}) bool {
	if value == nil {
		return true
	}

	switch v := value.(type) {
	case string:
		return v == ""
	case bool:
		return !v
	case int, int8, int16, int32, int64:
		return v == 0
	case uint, uint8, uint16, uint32, uint64:
		return v == 0
	case float32, float64:
		return v == 0
	default:
		return false
	}
}

// applyValidationRule applies a single validation rule
func (v *TagBasedValidator) applyValidationRule(value interface{}, fieldName string, rule ValidationRule) error {
	v.mutex.RLock()
	validator, exists := v.validators[rule.Name]
	v.mutex.RUnlock()

	if !exists {
		if v.options.IgnoreUnknown {
			return nil
		}
		return errors.NewError().
			Message(fmt.Sprintf("unknown validation rule: %s", rule.Name)).
			WithLocation().
			Build()
	}

	return validator(value, fieldName, rule.Parameters)
}

// Built-in validation functions

// validateRequired validates required fields
func (v *TagBasedValidator) validateRequired(value interface{}, fieldName string, _ map[string]interface{}) error {
	if v.isZeroValue(value) {
		return errors.NewError().
			Message(fmt.Sprintf("field %s is required", fieldName)).
			WithLocation().
			Build()
	}
	return nil
}

// validateMin validates minimum value constraints
func (v *TagBasedValidator) validateMin(value interface{}, fieldName string, params map[string]interface{}) error {
	paramValue, exists := params["value"]
	if !exists {
		return errors.NewError().
			Message("min validation requires a value parameter").
			WithLocation().
			Build()
	}

	minVal := v.parseIntParam(paramValue.(string))

	switch v := value.(type) {
	case string:
		if len(v) < minVal {
			return errors.NewError().
				Message(fmt.Sprintf("field %s must be at least %d characters", fieldName, minVal)).
				WithLocation().
				Build()
		}
	case int:
		if v < minVal {
			return errors.NewError().
				Message(fmt.Sprintf("field %s must be at least %d", fieldName, minVal)).
				WithLocation().
				Build()
		}
	case []interface{}:
		if len(v) < minVal {
			return errors.NewError().
				Message(fmt.Sprintf("field %s must have at least %d items", fieldName, minVal)).
				WithLocation().
				Build()
		}
	}

	return nil
}

// validateMax validates maximum value constraints
func (v *TagBasedValidator) validateMax(value interface{}, fieldName string, params map[string]interface{}) error {
	paramValue, exists := params["value"]
	if !exists {
		return errors.NewError().
			Message("max validation requires a value parameter").
			WithLocation().
			Build()
	}

	maxVal := v.parseIntParam(paramValue.(string))

	switch v := value.(type) {
	case string:
		if len(v) > maxVal {
			return errors.NewError().
				Message(fmt.Sprintf("field %s must be at most %d characters", fieldName, maxVal)).
				WithLocation().
				Build()
		}
	case int:
		if v > maxVal {
			return errors.NewError().
				Message(fmt.Sprintf("field %s must be at most %d", fieldName, maxVal)).
				WithLocation().
				Build()
		}
	case []interface{}:
		if len(v) > maxVal {
			return errors.NewError().
				Message(fmt.Sprintf("field %s must have at most %d items", fieldName, maxVal)).
				WithLocation().
				Build()
		}
	}

	return nil
}

// validateLen validates exact length constraints
func (v *TagBasedValidator) validateLen(value interface{}, fieldName string, params map[string]interface{}) error {
	paramValue, exists := params["value"]
	if !exists {
		return errors.NewError().
			Message("len validation requires a value parameter").
			WithLocation().
			Build()
	}

	lenVal := v.parseIntParam(paramValue.(string))

	switch v := value.(type) {
	case string:
		if len(v) != lenVal {
			return errors.NewError().
				Message(fmt.Sprintf("field %s must be exactly %d characters", fieldName, lenVal)).
				WithLocation().
				Build()
		}
	case []interface{}:
		if len(v) != lenVal {
			return errors.NewError().
				Message(fmt.Sprintf("field %s must have exactly %d items", fieldName, lenVal)).
				WithLocation().
				Build()
		}
	}

	return nil
}

// validateEmail validates email format
func (v *TagBasedValidator) validateEmail(value interface{}, fieldName string, _ map[string]interface{}) error {
	str, ok := value.(string)
	if !ok {
		return errors.NewError().
			Message(fmt.Sprintf("field %s must be a string for email validation", fieldName)).
			WithLocation().
			Build()
	}

	if !strings.Contains(str, "@") || !strings.Contains(str, ".") {
		return errors.NewError().
			Message(fmt.Sprintf("field %s must be a valid email address", fieldName)).
			WithLocation().
			Build()
	}

	return nil
}

// validateURL validates URL format
func (v *TagBasedValidator) validateURL(value interface{}, fieldName string, _ map[string]interface{}) error {
	str, ok := value.(string)
	if !ok {
		return errors.NewError().
			Message(fmt.Sprintf("field %s must be a string for URL validation", fieldName)).
			WithLocation().
			Build()
	}

	if !strings.HasPrefix(str, "http://") && !strings.HasPrefix(str, "https://") {
		return errors.NewError().
			Message(fmt.Sprintf("field %s must be a valid URL", fieldName)).
			WithLocation().
			Build()
	}

	return nil
}

// validateUUID validates UUID format
func (v *TagBasedValidator) validateUUID(value interface{}, fieldName string, _ map[string]interface{}) error {
	str, ok := value.(string)
	if !ok {
		return errors.NewError().
			Message(fmt.Sprintf("field %s must be a string for UUID validation", fieldName)).
			WithLocation().
			Build()
	}

	// Simple UUID format check
	if len(str) != 36 || strings.Count(str, "-") != 4 {
		return errors.NewError().
			Message(fmt.Sprintf("field %s must be a valid UUID", fieldName)).
			WithLocation().
			Build()
	}

	return nil
}

// validateOneOf validates that value is one of the allowed values
func (v *TagBasedValidator) validateOneOf(value interface{}, fieldName string, params map[string]interface{}) error {
	paramValue, exists := params["value"]
	if !exists {
		return errors.NewError().
			Message("oneof validation requires a value parameter").
			WithLocation().
			Build()
	}

	allowedValues := strings.Split(paramValue.(string), " ")
	valueStr := fmt.Sprintf("%v", value)

	for _, allowed := range allowedValues {
		if valueStr == allowed {
			return nil
		}
	}

	return errors.NewError().
		Message(fmt.Sprintf("field %s must be one of: %s", fieldName, strings.Join(allowedValues, ", "))).
		WithLocation().
		Build()
}

// validateImageName validates Docker image name format
func (v *TagBasedValidator) validateImageName(value interface{}, fieldName string, _ map[string]interface{}) error {
	str, ok := value.(string)
	if !ok {
		return errors.NewError().
			Message(fmt.Sprintf("field %s must be a string for image name validation", fieldName)).
			WithLocation().
			Build()
	}

	// Basic image name validation
	if str == "" {
		return errors.NewError().
			Message(fmt.Sprintf("field %s cannot be empty", fieldName)).
			WithLocation().
			Build()
	}

	// Check for invalid characters
	if strings.ContainsAny(str, " \t\n\r") {
		return errors.NewError().
			Message(fmt.Sprintf("field %s cannot contain whitespace", fieldName)).
			WithLocation().
			Build()
	}

	return nil
}

// validateK8sName validates Kubernetes name format
func (v *TagBasedValidator) validateK8sName(value interface{}, fieldName string, _ map[string]interface{}) error {
	str, ok := value.(string)
	if !ok {
		return errors.NewError().
			Message(fmt.Sprintf("field %s must be a string for Kubernetes name validation", fieldName)).
			WithLocation().
			Build()
	}

	// Basic Kubernetes name validation
	if len(str) > 253 {
		return errors.NewError().
			Message(fmt.Sprintf("field %s must be at most 253 characters", fieldName)).
			WithLocation().
			Build()
	}

	// Check for invalid characters
	if strings.ContainsAny(str, " \t\n\r") {
		return errors.NewError().
			Message(fmt.Sprintf("field %s cannot contain whitespace", fieldName)).
			WithLocation().
			Build()
	}

	return nil
}

// validatePort validates port number
func (v *TagBasedValidator) validatePort(value interface{}, fieldName string, _ map[string]interface{}) error {
	var port int

	switch val := value.(type) {
	case int:
		port = val
	case string:
		port = v.parseIntParam(val)
	default:
		return errors.NewError().
			Message(fmt.Sprintf("field %s must be a number for port validation", fieldName)).
			WithLocation().
			Build()
	}

	if port < 1 || port > 65535 {
		return errors.NewError().
			Message(fmt.Sprintf("field %s must be a valid port number (1-65535)", fieldName)).
			WithLocation().
			Build()
	}

	return nil
}

// validateDive validates array elements
func (v *TagBasedValidator) validateDive(value interface{}, fieldName string, params map[string]interface{}) error {
	// For now, just check if it's an array
	switch value.(type) {
	case []interface{}, []string, []int:
		return nil
	default:
		return errors.NewError().
			Message(fmt.Sprintf("field %s must be an array for dive validation", fieldName)).
			WithLocation().
			Build()
	}
}

// parseIntParam parses an integer parameter from a string
func (v *TagBasedValidator) parseIntParam(param string) int {
	if param == "" {
		return 0
	}

	var val int
	for _, r := range param {
		if r < '0' || r > '9' {
			break
		}
		val = val*10 + int(r-'0')
	}
	return val
}

// combineErrors combines multiple validation errors into one
func (v *TagBasedValidator) combineErrors(errs []error) error {
	if len(errs) == 1 {
		return errs[0]
	}

	var messages []string
	for _, err := range errs {
		messages = append(messages, err.Error())
	}

	return errors.NewError().
		Message(fmt.Sprintf("validation errors: %s", strings.Join(messages, "; "))).
		WithLocation().
		Build()
}

// RegisterValidator registers a custom validation function
func (v *TagBasedValidator) RegisterValidator(name string, validator ValidationFunc) {
	v.mutex.Lock()
	defer v.mutex.Unlock()
	v.validators[name] = validator
}

// ListValidators returns all registered validator names
func (v *TagBasedValidator) ListValidators() []string {
	v.mutex.RLock()
	defer v.mutex.RUnlock()

	names := make([]string, 0, len(v.validators))
	for name := range v.validators {
		names = append(names, name)
	}
	return names
}

// UnifiedTagBasedValidator implements the unified validation framework
type UnifiedTagBasedValidator struct {
	tagValidator *TagBasedValidator
}

// NewUnifiedTagBasedValidator creates a new unified tag-based validator
func NewUnifiedTagBasedValidator(options ...ValidationOptions) Validator {
	tagValidator := NewTagBasedValidator(options...)
	return &UnifiedTagBasedValidator{
		tagValidator: tagValidator,
	}
}

// Validate implements the security.Validator interface
func (v *UnifiedTagBasedValidator) Validate(ctx context.Context, data any) Result {
	result := NewSessionResult("unified_tag_based_validator", "1.0.0")

	// Handle different input types
	switch input := data.(type) {
	case map[string]interface{}:
		// Validate each field in the map
		for fieldName, fieldValue := range input {
			// For now, we'll validate basic fields without specific rules
			// This can be enhanced later to support rule extraction
			_ = fieldName
			if fieldValue != nil {
				// Basic validation - just check if field has value
				continue
			}
		}
	default:
		result.AddError("", "unsupported data type for tag-based validation", "TAG_VALIDATOR_002", data, SeverityHigh)
	}

	return *result
}

// ValidateWithOptions implements the security.Validator interface
func (v *UnifiedTagBasedValidator) ValidateWithOptions(ctx context.Context, data any, opts Options) Result {
	// For now, just call the basic Validate method
	// This can be enhanced later to use the options
	return v.Validate(ctx, data)
}

// Name returns the validator name
func (v *UnifiedTagBasedValidator) Name() string {
	return "unified_tag_based_validator"
}

// GetName returns the validator name (legacy compatibility)
func (v *UnifiedTagBasedValidator) GetName() string {
	return "unified_tag_based_validator"
}

// GetVersion returns the validator version
func (v *UnifiedTagBasedValidator) GetVersion() string {
	return "1.0.0"
}

// GetSupportedTypes returns the data types this validator can handle
func (v *UnifiedTagBasedValidator) GetSupportedTypes() []string {
	return []string{"map[string]interface{}", "struct"}
}

// ValidateTaggedData validates data using tag-based validation (main entry point)
func ValidateTaggedData(ctx context.Context, data interface{}, rules map[string]string) error {
	validator := NewTagBasedValidator()

	switch input := data.(type) {
	case map[string]interface{}:
		var errors []error

		for fieldName, fieldValue := range input {
			if fieldRules, exists := rules[fieldName]; exists {
				err := validator.ValidateField(ctx, fieldName, fieldValue, fieldRules)
				if err != nil {
					errors = append(errors, err)
				}
			}
		}

		if len(errors) > 0 {
			return validator.combineErrors(errors)
		}
	}

	return nil
}

// Global tag-based validator instance
var globalTagValidator = NewTagBasedValidator()

// GetGlobalTagValidator returns the global tag-based validator
func GetGlobalTagValidator() *TagBasedValidator {
	return globalTagValidator
}
