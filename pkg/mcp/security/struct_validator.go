package security

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/common/validation-core/core"
	"github.com/Azure/container-kit/pkg/mcp/errors"
)

// StructValidator provides tag-based validation for structs
// Deprecated: This uses reflection-based validation. Use the type-safe validators
// from pkg/common/validation-core/generic_validator.go instead for better performance
// and type safety.
type StructValidator struct {
	parser     *TagParser
	validators map[reflect.Type]StructValidatorFunc
	options    StructValidationOptions
}

// UnifiedStructValidator implements the unified validation framework for struct validation
type UnifiedStructValidator struct {
	core.Validator
	structValidator *StructValidator
}

// StructValidationData represents the data structure for struct validation
type StructValidationData struct {
	StructValue interface{}            `json:"struct_value"`
	Rules       []string               `json:"rules,omitempty"`
	Options     map[string]interface{} `json:"options,omitempty"`
}

// StructValidatorFunc is a function that validates a struct value
type StructValidatorFunc func(reflect.Value) error

// StructValidationOptions provides options for struct validation
type StructValidationOptions struct {
	FailFast      bool          // Stop on first validation error
	CacheRules    bool          // Cache parsed validation rules
	Timeout       time.Duration // Validation timeout
	IgnoreUnknown bool          // Ignore unknown validation tags
	CustomTags    string        // Custom tag name (default: "validate")
}

// DefaultStructValidationOptions returns default validation options
func DefaultStructValidationOptions() StructValidationOptions {
	return StructValidationOptions{
		FailFast:      false,
		CacheRules:    true,
		Timeout:       30 * time.Second,
		IgnoreUnknown: false,
		CustomTags:    "validate",
	}
}

// NewStructValidator creates a new struct validator
func NewStructValidator(options ...StructValidationOptions) *StructValidator {
	opts := DefaultStructValidationOptions()
	if len(options) > 0 {
		opts = options[0]
	}

	return &StructValidator{
		parser:     NewTagParser(),
		validators: make(map[reflect.Type]StructValidatorFunc),
		options:    opts,
	}
}

// ValidateStruct validates a struct using validation tags
func (sv *StructValidator) ValidateStruct(ctx context.Context, structPtr interface{}) error {
	if structPtr == nil {
		return errors.Validation("struct_validator", "struct cannot be nil")
	}

	// Set up timeout if specified
	if sv.options.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, sv.options.Timeout)
		defer cancel()
	}

	// Get reflection info
	structValue := reflect.ValueOf(structPtr)
	structType := reflect.TypeOf(structPtr)

	// Handle pointers
	if structType.Kind() == reflect.Ptr {
		if structValue.IsNil() {
			return errors.Validation("struct_validator", "struct pointer cannot be nil")
		}
		structValue = structValue.Elem()
		structType = structType.Elem()
	}

	// Ensure we have a struct
	if structType.Kind() != reflect.Struct {
		return errors.Validationf("struct_validator", "expected struct, got %s", structType.Kind())
	}

	// Check for timeout
	select {
	case <-ctx.Done():
		return errors.Validation("struct_validator", "validation timeout")
	default:
	}

	// Get or create validator function for this struct type
	validator, err := sv.getValidatorFunc(structType)
	if err != nil {
		return errors.Wrapf(err, "struct_validator", "failed to get validator for %s", structType.Name())
	}

	// Apply validation
	return validator(structValue)
}

// getValidatorFunc gets or creates a validator function for the given struct type
func (sv *StructValidator) getValidatorFunc(structType reflect.Type) (StructValidatorFunc, error) {
	// Check cache if enabled
	if sv.options.CacheRules {
		if validator, exists := sv.validators[structType]; exists {
			return validator, nil
		}
	}

	// Parse validation rules from struct tags
	rules, err := sv.parser.ParseStruct(structType)
	if err != nil {
		return nil, errors.Wrapf(err, "struct_validator", "failed to parse validation tags")
	}

	// Create validator function
	validator := sv.createValidatorFunc(rules)

	// Cache if enabled
	if sv.options.CacheRules {
		sv.validators[structType] = validator
	}

	return validator, nil
}

// createValidatorFunc creates a validator function from parsed rules
func (sv *StructValidator) createValidatorFunc(rules []ValidationRule) StructValidatorFunc {
	return func(structValue reflect.Value) error {
		var validationErrors []error

		for _, rule := range rules {
			if err := sv.parser.applyValidationRule(structValue, rule); err != nil {
				validationErrors = append(validationErrors, err)

				// Stop on first error if FailFast is enabled
				if sv.options.FailFast {
					return err
				}
			}
		}

		// Return combined errors if any
		if len(validationErrors) > 0 {
			return sv.combineErrors(validationErrors)
		}

		return nil
	}
}

// combineErrors combines multiple validation errors into a single error
func (sv *StructValidator) combineErrors(validationErrors []error) error {
	if len(validationErrors) == 1 {
		return validationErrors[0]
	}

	var errorMessages []string
	for _, err := range validationErrors {
		errorMessages = append(errorMessages, err.Error())
	}

	return errors.Validationf("struct_validator", "multiple validation errors: %v", errorMessages)
}

// ValidateField validates a single field value using a validation tag
func (sv *StructValidator) ValidateField(fieldValue interface{}, tag string) error {
	if tag == "" {
		return nil // No validation required
	}

	// Create a temporary struct to hold the field
	tempStructType := reflect.StructOf([]reflect.StructField{
		{
			Name: "TempField",
			Type: reflect.TypeOf(fieldValue),
			Tag:  reflect.StructTag(sv.options.CustomTags + `:"` + tag + `"`),
		},
	})

	// Create instance and set field value
	tempStruct := reflect.New(tempStructType).Elem()
	tempStruct.Field(0).Set(reflect.ValueOf(fieldValue))

	// Parse and validate
	rules, err := sv.parser.ParseStruct(tempStructType)
	if err != nil {
		return errors.Wrapf(err, "struct_validator", "failed to parse field validation tag")
	}

	validator := sv.createValidatorFunc(rules)
	return validator(tempStruct)
}

// RegisterCustomValidator registers a custom validation function
func (sv *StructValidator) RegisterCustomValidator(name string, validator CustomValidatorFunc) {
	// This would extend the tag parser to support custom validators
	// Implementation depends on how we want to handle custom validators
}

// CustomValidatorFunc is a custom validation function
type CustomValidatorFunc func(fieldValue interface{}, params map[string]interface{}) error

// ValidationExample demonstrates usage of the struct validator
type ValidationExample struct {
	Name        string   `validate:"required,min=3,max=50"`
	Email       string   `validate:"required,email"`
	Port        int      `validate:"required,min=1,max=65535"`
	Image       string   `validate:"required,image_name"`
	Namespace   string   `validate:"k8s_name"`
	Tags        []string `validate:"required,min=1,dive,required"`
	Environment string   `validate:"required,oneof=dev staging prod"`
}

// Validate provides a convenience method for the example struct
func (ve *ValidationExample) Validate() error {
	validator := NewStructValidator()
	return validator.ValidateStruct(context.Background(), ve)
}

// BatchValidator allows validating multiple structs with the same rules
type BatchValidator struct {
	structValidator *StructValidator
	options         BatchValidationOptions
}

// BatchValidationOptions provides options for batch validation
type BatchValidationOptions struct {
	StopOnError   bool // Stop batch validation on first error
	MaxConcurrent int  // Maximum concurrent validations (0 = no limit)
}

// NewBatchValidator creates a new batch validator
func NewBatchValidator(options ...BatchValidationOptions) *BatchValidator {
	opts := BatchValidationOptions{
		StopOnError:   false,
		MaxConcurrent: 0,
	}
	if len(options) > 0 {
		opts = options[0]
	}

	return &BatchValidator{
		structValidator: NewStructValidator(),
		options:         opts,
	}
}

// ValidateBatch validates multiple structs
func (bv *BatchValidator) ValidateBatch(ctx context.Context, structs []interface{}) []error {
	var validationErrors []error

	for i, structPtr := range structs {
		if err := bv.structValidator.ValidateStruct(ctx, structPtr); err != nil {
			validationErrors = append(validationErrors,
				errors.Wrapf(err, "batch_validator", "validation failed for item %d", i))

			if bv.options.StopOnError {
				break
			}
		}
	}

	return validationErrors
}

// ValidationMiddleware provides middleware for HTTP handlers or similar
type ValidationMiddleware struct {
	validator *StructValidator
}

// NewValidationMiddleware creates validation middleware
func NewValidationMiddleware() *ValidationMiddleware {
	return &ValidationMiddleware{
		validator: NewStructValidator(),
	}
}

// ValidateRequest validates a request struct
func (vm *ValidationMiddleware) ValidateRequest(ctx context.Context, request interface{}) error {
	return vm.validator.ValidateStruct(ctx, request)
}

// GenerateValidator can be used with go:generate to create optimized validators
func GenerateValidator(structType reflect.Type) (string, error) {
	parser := NewTagParser()
	rules, err := parser.ParseStruct(structType)
	if err != nil {
		return "", err
	}

	// Generate optimized Go code for validation
	// This would be implemented in a separate code generation tool
	_ = rules
	return "// Generated validation code would go here", nil
}

// --- Unified Validation Framework Implementation ---

// NewUnifiedStructValidator creates a new unified struct validator
func NewUnifiedStructValidator(options ...StructValidationOptions) core.Validator {
	structValidator := NewStructValidator(options...)
	return &UnifiedStructValidator{
		structValidator: structValidator,
	}
}

// Validate implements the core.Validator interface
func (v *UnifiedStructValidator) Validate(ctx context.Context, data interface{}, options *core.ValidationOptions) *core.NonGenericResult {
	result := core.NewNonGenericResult("unified_struct_validator", "1.0.0")

	// Convert input data
	var structData *StructValidationData
	if mapped, ok := data.(map[string]interface{}); ok {
		// Handle map input - extract struct_value
		structValue, exists := mapped["struct_value"]
		if !exists {
			result.AddError(&core.Error{
				Code:     "STRUCT_VALIDATOR_001",
				Message:  "struct_value field is required",
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityHigh,
				Field:    "struct_value",
			})
			return result
		}
		structData = &StructValidationData{
			StructValue: structValue,
		}
		if rules, ok := mapped["rules"].([]string); ok {
			structData.Rules = rules
		}
		if opts, ok := mapped["options"].(map[string]interface{}); ok {
			structData.Options = opts
		}
	} else if typed, ok := data.(*StructValidationData); ok {
		structData = typed
	} else {
		// Assume the data itself is the struct to validate
		structData = &StructValidationData{
			StructValue: data,
		}
	}

	// Perform struct validation
	err := v.structValidator.ValidateStruct(ctx, structData.StructValue)

	if err != nil {
		// Convert domain error to unified error
		result.AddError(&core.Error{
			Code:     "STRUCT_VALIDATOR_002",
			Message:  err.Error(),
			Type:     core.ErrTypeValidation,
			Severity: core.SeverityHigh,
			Context: map[string]interface{}{
				"original_error": err.Error(),
			},
		})
	} else {
		// Validation succeeded - result.Valid is already true by default
		result.AddSuggestion("Struct validation passed successfully")
	}

	return result
}

// GetName returns the validator name
func (v *UnifiedStructValidator) GetName() string {
	return "unified_struct_validator"
}

// GetVersion returns the validator version
func (v *UnifiedStructValidator) GetVersion() string {
	return "1.0.0"
}

// GetSupportedTypes returns the data types this validator can handle
func (v *UnifiedStructValidator) GetSupportedTypes() []string {
	return []string{"struct", "map[string]interface{}", "*StructValidationData"}
}

// ValidateStructUnified provides a convenience method for unified struct validation
func ValidateStructUnified(ctx context.Context, structPtr interface{}, options *core.ValidationOptions) *core.NonGenericResult {
	validator := NewUnifiedStructValidator()
	return validator.Validate(ctx, structPtr, options)
}

// ValidateTaggedStruct validates a struct using our tag-based validation system
// This is the main function that tools should use to validate their argument structs
func ValidateTaggedStruct(structPtr interface{}) error {
	if structPtr == nil {
		return errors.Validation("validate_tagged_struct", "struct cannot be nil")
	}

	// Get reflection info
	structValue := reflect.ValueOf(structPtr)
	structType := reflect.TypeOf(structPtr)

	// Handle pointers
	if structType.Kind() == reflect.Ptr {
		if structValue.IsNil() {
			return errors.Validation("validate_tagged_struct", "struct pointer cannot be nil")
		}
		structValue = structValue.Elem()
		structType = structType.Elem()
	}

	// Ensure we have a struct
	if structType.Kind() != reflect.Struct {
		return errors.Validationf("validate_tagged_struct", "expected struct, got %s", structType.Kind())
	}

	// Get common validation tags
	tagDefs := CommonValidationTags()

	// Validate each field
	var validationErrors []error
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		fieldValue := structValue.Field(i)

		// Skip unexported fields
		if !fieldValue.CanInterface() {
			continue
		}

		// Get validation tags
		validateTag := field.Tag.Get("validate")
		if validateTag == "" {
			continue
		}

		// Parse validation rules
		rules := parseValidationRules(validateTag)

		// Check if this is a dive validation (for slices/arrays)
		diveIndex := -1
		for i, rule := range rules {
			if rule == "dive" {
				diveIndex = i
				break
			}
		}

		if diveIndex >= 0 {
			// Check if field should be skipped due to omitempty
			shouldSkip := false
			for _, rule := range rules {
				if rule == "omitempty" && isZeroValue(fieldValue.Interface()) {
					shouldSkip = true
					break
				}
			}

			if !shouldSkip {
				// Apply pre-dive rules to the slice itself
				for i := 0; i < diveIndex; i++ {
					// Skip the omitempty rule itself as it's already been processed
					if rules[i] == "omitempty" {
						continue
					}
					if err := applyValidationRule(fieldValue.Interface(), field.Name, rules[i], tagDefs); err != nil {
						validationErrors = append(validationErrors, err)
					}
				}
			}

			// Apply post-dive rules to each element (only if not skipped)
			if !shouldSkip && diveIndex+1 < len(rules) && fieldValue.Kind() == reflect.Slice {
				elementRules := rules[diveIndex+1:]
				for i := 0; i < fieldValue.Len(); i++ {
					elem := fieldValue.Index(i)
					for _, rule := range elementRules {
						if err := applyValidationRule(elem.Interface(), fmt.Sprintf("%s[%d]", field.Name, i), rule, tagDefs); err != nil {
							validationErrors = append(validationErrors, err)
						}
					}
				}
			}
		} else {
			// No dive, apply all rules to the field itself
			// Check if field should be skipped due to omitempty
			shouldSkip := false
			for _, rule := range rules {
				if rule == "omitempty" && isZeroValue(fieldValue.Interface()) {
					shouldSkip = true
					break
				}
			}

			if !shouldSkip {
				for _, rule := range rules {
					// Skip the omitempty rule itself as it's already been processed
					if rule == "omitempty" {
						continue
					}
					if err := applyValidationRule(fieldValue.Interface(), field.Name, rule, tagDefs); err != nil {
						validationErrors = append(validationErrors, err)
					}
				}
			}
		}
	}

	// Return combined errors
	if len(validationErrors) > 0 {
		return combineValidationErrors(validationErrors)
	}

	return nil
}

// parseValidationRules parses a validation tag string into individual rules
func parseValidationRules(tag string) []string {
	if tag == "" {
		return nil
	}
	return strings.Split(tag, ",")
}

// applyValidationRule applies a single validation rule to a field value
func applyValidationRule(value interface{}, fieldName string, rule string, tagDefs map[string]ValidationTagDefinition) error {
	// Handle rule parameters (e.g., "min=5", "max=10")
	parts := strings.SplitN(rule, "=", 2)
	ruleName := parts[0]
	var ruleParam string
	if len(parts) > 1 {
		ruleParam = parts[1]
	}

	// Handle special rules
	switch ruleName {
	case "required":
		if isZeroValue(value) {
			return errors.Validationf(fieldName, "field %s is required", fieldName)
		}
		return nil
	case "omitempty":
		// omitempty means skip validation if field is empty
		if isZeroValue(value) {
			return nil
		}
		return nil
	case "min":
		return validateMin(value, fieldName, ruleParam)
	case "max":
		return validateMax(value, fieldName, ruleParam)
	case "dive":
		// dive means validate slice/array elements
		return validateDive(value, fieldName, tagDefs)
	case "keys", "endkeys", "values", "endvalues":
		// Map validation - skip for now
		return nil
	}

	// Check if it's a custom validation tag
	if tagDef, exists := tagDefs[ruleName]; exists {
		if tagDef.Validator != nil {
			params := make(map[string]interface{})
			if ruleParam != "" {
				params["param"] = ruleParam
			}
			return tagDef.Validator(value, fieldName, params)
		}
	}

	// Unknown validation rule - ignore for now
	return nil
}

// isZeroValue checks if a value is the zero value for its type
func isZeroValue(value interface{}) bool {
	if value == nil {
		return true
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.String:
		return v.String() == ""
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Slice, reflect.Map, reflect.Array:
		return v.Len() == 0
	case reflect.Ptr, reflect.Interface:
		return v.IsNil()
	}
	return false
}

// validateMin validates minimum value constraint
func validateMin(value interface{}, fieldName string, param string) error {
	v := reflect.ValueOf(value)
	minVal, err := parseIntParam(param)
	if err != nil {
		return errors.Validationf(fieldName, "invalid min parameter: %s", param)
	}

	switch v.Kind() {
	case reflect.String:
		if len(v.String()) < minVal {
			return errors.Validationf(fieldName, "field %s must be at least %d characters", fieldName, minVal)
		}
	case reflect.Slice, reflect.Array, reflect.Map:
		if v.Len() < minVal {
			return errors.Validationf(fieldName, "field %s must have at least %d items", fieldName, minVal)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if v.Int() < int64(minVal) {
			return errors.Validationf(fieldName, "field %s must be at least %d", fieldName, minVal)
		}
	}
	return nil
}

// validateMax validates maximum value constraint
func validateMax(value interface{}, fieldName string, param string) error {
	v := reflect.ValueOf(value)
	maxVal, err := parseIntParam(param)
	if err != nil {
		return errors.Validationf(fieldName, "invalid max parameter: %s", param)
	}

	switch v.Kind() {
	case reflect.String:
		if len(v.String()) > maxVal {
			return errors.Validationf(fieldName, "field %s must be at most %d characters", fieldName, maxVal)
		}
	case reflect.Slice, reflect.Array, reflect.Map:
		if v.Len() > maxVal {
			return errors.Validationf(fieldName, "field %s must have at most %d items", fieldName, maxVal)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if v.Int() > int64(maxVal) {
			return errors.Validationf(fieldName, "field %s must be at most %d", fieldName, maxVal)
		}
	}
	return nil
}

// validateDive validates slice/array elements
func validateDive(value interface{}, fieldName string, tagDefs map[string]ValidationTagDefinition) error {
	v := reflect.ValueOf(value)
	if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
		return nil
	}

	// For now, we'll skip dive validation
	// Full implementation would parse remaining tags after "dive" and apply them to elements
	return nil
}

// parseIntParam parses an integer parameter from a string
func parseIntParam(param string) (int, error) {
	if param == "" {
		return 0, errors.Validation("parse_int_param", "empty parameter")
	}
	var val int
	for _, r := range param {
		if r < '0' || r > '9' {
			return 0, errors.Validation("parse_int_param", "invalid integer")
		}
		val = val*10 + int(r-'0')
	}
	return val, nil
}

// combineValidationErrors combines multiple validation errors into one
func combineValidationErrors(errs []error) error {
	if len(errs) == 1 {
		return errs[0]
	}
	var messages []string
	for _, err := range errs {
		messages = append(messages, err.Error())
	}
	return errors.Validationf("validation", "multiple validation errors: %s", strings.Join(messages, "; "))
}
