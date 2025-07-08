// Package security - Validation engine and rule processing
// This file provides a rule-based validation engine for complex validation scenarios
package security

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"
)

// Engine provides rule-based validation capabilities
type Engine struct {
	rules                []Rule
	options              Options
	sanitizer            *Sanitizer
	constraintValidators map[string]ConstraintValidator
}

// NewEngine creates a new validation engine
func NewEngine(options Options) *Engine {
	return &Engine{
		rules:     make([]Rule, 0),
		options:   options,
		sanitizer: NewSanitizer(),
	}
}

// AddRule adds a validation rule to the engine
func (ve *Engine) AddRule(rule Rule) {
	ve.rules = append(ve.rules, rule)
}

// AddRules adds multiple validation rules to the engine
func (ve *Engine) AddRules(rules []Rule) {
	ve.rules = append(ve.rules, rules...)
}

// ValidateStruct validates a struct using the configured rules
func (ve *Engine) ValidateStruct(ctx context.Context, data interface{}) *Result {
	start := time.Now()
	result := NewResult()
	result.Metadata.ValidatorName = "Engine"
	result.Metadata.ValidatorVersion = "1.0.0"

	// Check for timeout
	if ve.options.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, ve.options.Timeout)
		defer cancel()
	}

	// Get reflection information
	val := reflect.ValueOf(data)
	typ := reflect.TypeOf(data)

	// Handle pointers
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			result.AddError("", "data cannot be nil", "NIL_DATA", nil, SeverityCritical)
			return result
		}
		val = val.Elem()
		typ = typ.Elem()
	}

	// Only handle structs
	if val.Kind() != reflect.Struct {
		result.AddError("", "data must be a struct", "INVALID_TYPE", data, SeverityCritical)
		return result
	}

	// Apply validation rules
	for _, rule := range ve.rules {
		// Check if validation was cancelled
		select {
		case <-ctx.Done():
			result.AddError("", "validation timeout", "TIMEOUT", nil, SeverityMedium)
			return result
		default:
		}

		// Skip disabled rules
		if !rule.Enabled {
			continue
		}

		// Check condition if specified
		if rule.Condition != "" && !ve.evaluateCondition(rule.Condition, data) {
			continue
		}

		// Apply rule to field
		if err := ve.applyRule(rule, val, typ, ""); err != nil {
			result.Errors = append(result.Errors, *err)
			result.Valid = false

			// Check if we should fail fast
			if ve.options.FailFast {
				break
			}

			// Check max errors limit
			if ve.options.MaxErrors > 0 && len(result.Errors) >= ve.options.MaxErrors {
				result.AddError("", "maximum error limit reached", "MAX_ERRORS", len(result.Errors), SeverityLow)
				break
			}
		}
	}

	// Finalize result
	result.Metadata.Duration = time.Since(start)
	result.Duration = result.Metadata.Duration

	// Apply sanitization if needed
	if ve.sanitizer != nil {
		ve.sanitizeResult(result)
	}

	return result
}

// applyRule applies a validation rule to a specific field
func (ve *Engine) applyRule(rule Rule, val reflect.Value, typ reflect.Type, pathPrefix string) *Error {
	// Find the field
	fieldValue, fieldType, fieldPath := ve.findField(rule.Field, val, typ, pathPrefix)
	if !fieldValue.IsValid() {
		return &Error{
			Field:    fieldPath,
			Message:  fmt.Sprintf("field '%s' not found", rule.Field),
			Code:     "FIELD_NOT_FOUND",
			Severity: SeverityLow,
			Path:     fieldPath,
		}
	}

	// Skip if field should be skipped
	if ve.shouldSkipField(fieldPath) {
		return nil
	}

	// Apply constraints to the field
	for _, constraint := range rule.Constraints {
		if err := ve.applyConstraint(constraint, fieldValue, fieldType, fieldPath); err != nil {
			err.Path = fieldPath
			return err
		}
	}

	return nil
}

// applyConstraint applies a single constraint to a field value
func (ve *Engine) applyConstraint(constraint FieldConstraint, fieldValue reflect.Value, _ reflect.Type, fieldPath string) *Error {
	fieldInterface := fieldValue.Interface()

	// Use strategy pattern to handle different constraint types
	validator := ve.getConstraintValidator(constraint.Type)
	if validator == nil {
		return &Error{
			Field:    fieldPath,
			Message:  fmt.Sprintf("unknown constraint type: %s", constraint.Type),
			Code:     "UNKNOWN_CONSTRAINT",
			Severity: SeverityLow,
		}
	}

	return validator.Validate(fieldInterface, fieldPath, constraint.Value)
}

// ConstraintValidator defines the interface for constraint validation strategies
type ConstraintValidator interface {
	Validate(value interface{}, fieldPath string, constraintValue interface{}) *Error
}

// getConstraintValidator returns the appropriate validator for a constraint type
func (ve *Engine) getConstraintValidator(constraintType string) ConstraintValidator {
	// Initialize registry if not already done
	if ve.constraintValidators == nil {
		ve.initializeConstraintValidators()
	}

	return ve.constraintValidators[constraintType]
}

// initializeConstraintValidators sets up the constraint validator registry
func (ve *Engine) initializeConstraintValidators() {
	ve.constraintValidators = map[string]ConstraintValidator{
		"required":      &RequiredValidator{engine: ve},
		"min_length":    &MinLengthValidator{engine: ve},
		"max_length":    &MaxLengthValidator{engine: ve},
		"pattern":       &PatternValidator{engine: ve},
		"min":           &MinValidator{engine: ve},
		"max":           &MaxValidator{engine: ve},
		"enum":          &EnumValidator{engine: ve},
		"url":           &URLValidator{engine: ve},
		"ip":            &IPValidator{engine: ve},
		"port":          &PortValidator{engine: ve},
		"duration":      &DurationValidator{engine: ve},
		"image":         &ImageValidator{engine: ve},
		"session_id":    &SessionIDValidator{engine: ve},
		"namespace":     &NamespaceValidator{engine: ve},
		"resource_name": &ResourceNameValidator{engine: ve},
		"no_sensitive":  &NoSensitiveValidator{engine: ve},
	}
}

// Constraint validator implementations

// RequiredValidator validates required constraints
type RequiredValidator struct {
	engine *Engine
}

func (v *RequiredValidator) Validate(value interface{}, fieldPath string, _ interface{}) *Error {
	return v.engine.validateRequired(value, fieldPath)
}

// MinLengthValidator validates minimum length constraints
type MinLengthValidator struct {
	engine *Engine
}

func (v *MinLengthValidator) Validate(value interface{}, fieldPath string, constraintValue interface{}) *Error {
	if length, ok := constraintValue.(int); ok {
		return v.engine.validateMinLength(value, fieldPath, length)
	}
	return nil
}

// MaxLengthValidator validates maximum length constraints
type MaxLengthValidator struct {
	engine *Engine
}

func (v *MaxLengthValidator) Validate(value interface{}, fieldPath string, constraintValue interface{}) *Error {
	if length, ok := constraintValue.(int); ok {
		return v.engine.validateMaxLength(value, fieldPath, length)
	}
	return nil
}

// PatternValidator validates pattern constraints
type PatternValidator struct {
	engine *Engine
}

func (v *PatternValidator) Validate(value interface{}, fieldPath string, constraintValue interface{}) *Error {
	if pattern, ok := constraintValue.(string); ok {
		return v.engine.validatePatternString(value, fieldPath, pattern)
	}
	return nil
}

// MinValidator validates minimum value constraints
type MinValidator struct {
	engine *Engine
}

func (v *MinValidator) Validate(value interface{}, fieldPath string, constraintValue interface{}) *Error {
	if minVal, ok := constraintValue.(float64); ok {
		return v.engine.validateMin(value, fieldPath, minVal)
	}
	return nil
}

// MaxValidator validates maximum value constraints
type MaxValidator struct {
	engine *Engine
}

func (v *MaxValidator) Validate(value interface{}, fieldPath string, constraintValue interface{}) *Error {
	if maxVal, ok := constraintValue.(float64); ok {
		return v.engine.validateMax(value, fieldPath, maxVal)
	}
	return nil
}

// EnumValidator validates enum constraints
type EnumValidator struct {
	engine *Engine
}

func (v *EnumValidator) Validate(value interface{}, fieldPath string, constraintValue interface{}) *Error {
	if choices, ok := constraintValue.([]string); ok {
		return v.engine.validateEnumValue(value, fieldPath, choices)
	}
	return nil
}

// URLValidator validates URL constraints
type URLValidator struct {
	engine *Engine
}

func (v *URLValidator) Validate(value interface{}, fieldPath string, _ interface{}) *Error {
	return v.engine.validateURLValue(value, fieldPath)
}

// IPValidator validates IP constraints
type IPValidator struct {
	engine *Engine
}

func (v *IPValidator) Validate(value interface{}, fieldPath string, _ interface{}) *Error {
	return v.engine.validateIPValue(value, fieldPath)
}

// PortValidator validates port constraints
type PortValidator struct {
	engine *Engine
}

func (v *PortValidator) Validate(value interface{}, fieldPath string, _ interface{}) *Error {
	return v.engine.validatePortValue(value, fieldPath)
}

// DurationValidator validates duration constraints
type DurationValidator struct {
	engine *Engine
}

func (v *DurationValidator) Validate(value interface{}, fieldPath string, _ interface{}) *Error {
	return v.engine.validateDurationValue(value, fieldPath)
}

// ImageValidator validates image constraints
type ImageValidator struct {
	engine *Engine
}

func (v *ImageValidator) Validate(value interface{}, fieldPath string, _ interface{}) *Error {
	return v.engine.validateImageValue(value, fieldPath)
}

// SessionIDValidator validates session ID constraints
type SessionIDValidator struct {
	engine *Engine
}

func (v *SessionIDValidator) Validate(value interface{}, fieldPath string, _ interface{}) *Error {
	return v.engine.validateSessionIDValue(value, fieldPath)
}

// NamespaceValidator validates namespace constraints
type NamespaceValidator struct {
	engine *Engine
}

func (v *NamespaceValidator) Validate(value interface{}, fieldPath string, _ interface{}) *Error {
	return v.engine.validateNamespaceValue(value, fieldPath)
}

// ResourceNameValidator validates resource name constraints
type ResourceNameValidator struct {
	engine *Engine
}

func (v *ResourceNameValidator) Validate(value interface{}, fieldPath string, _ interface{}) *Error {
	return v.engine.validateResourceNameValue(value, fieldPath)
}

// NoSensitiveValidator validates no sensitive data constraints
type NoSensitiveValidator struct {
	engine *Engine
}

func (v *NoSensitiveValidator) Validate(value interface{}, fieldPath string, _ interface{}) *Error {
	return v.engine.validateNoSensitiveValue(value, fieldPath)
}

// Constraint validation helper functions

func (ve *Engine) validateRequired(value interface{}, fieldPath string) *Error {
	if str, ok := value.(string); ok {
		return ValidateRequired(str, fieldPath)
	}
	if value == nil || (reflect.ValueOf(value).Kind() == reflect.Ptr && reflect.ValueOf(value).IsNil()) {
		return &Error{
			Field:    fieldPath,
			Message:  fmt.Sprintf("%s is required", fieldPath),
			Code:     "FIELD_REQUIRED",
			Severity: SeverityHigh,
		}
	}
	return nil
}

func (ve *Engine) validateMinLength(value interface{}, fieldPath string, minLength int) *Error {
	if str, ok := value.(string); ok {
		return ValidateLength(str, fieldPath, minLength, 0)
	}
	return nil
}

func (ve *Engine) validateMaxLength(value interface{}, fieldPath string, maxLength int) *Error {
	if str, ok := value.(string); ok {
		return ValidateLength(str, fieldPath, 0, maxLength)
	}
	return nil
}

func (ve *Engine) validatePatternString(value interface{}, fieldPath, pattern string) *Error {
	if str, ok := value.(string); ok {
		if compiledPattern, err := regexp.Compile(pattern); err == nil {
			return ValidatePattern(str, fieldPath, compiledPattern, "must match pattern")
		}
	}
	return nil
}

func (ve *Engine) validateMin(value interface{}, fieldPath string, minVal float64) *Error {
	if num := ve.toFloat64(value); num != nil {
		return ValidateRange(*num, fieldPath, minVal, float64(1<<63-1))
	}
	return nil
}

func (ve *Engine) validateMax(value interface{}, fieldPath string, maxVal float64) *Error {
	if num := ve.toFloat64(value); num != nil {
		return ValidateRange(*num, fieldPath, float64(-1<<63), maxVal)
	}
	return nil
}

func (ve *Engine) validateEnumValue(value interface{}, fieldPath string, choices []string) *Error {
	if str, ok := value.(string); ok {
		return ValidateEnum(str, fieldPath, choices)
	}
	return nil
}

func (ve *Engine) validateURLValue(value interface{}, fieldPath string) *Error {
	if str, ok := value.(string); ok {
		return ValidateURL(str, fieldPath)
	}
	return nil
}

func (ve *Engine) validateIPValue(value interface{}, fieldPath string) *Error {
	if str, ok := value.(string); ok {
		return ValidateIPAddress(str, fieldPath)
	}
	return nil
}

func (ve *Engine) validatePortValue(value interface{}, fieldPath string) *Error {
	if port, ok := value.(int); ok {
		return ValidatePort(port, fieldPath)
	}
	return nil
}

func (ve *Engine) validateDurationValue(value interface{}, fieldPath string) *Error {
	if str, ok := value.(string); ok {
		return ValidateDuration(str, fieldPath)
	}
	return nil
}

func (ve *Engine) validateImageValue(value interface{}, fieldPath string) *Error {
	if str, ok := value.(string); ok {
		return ValidateImageReference(str, fieldPath)
	}
	return nil
}

func (ve *Engine) validateSessionIDValue(value interface{}, fieldPath string) *Error {
	if str, ok := value.(string); ok {
		return ValidateSessionID(str, fieldPath)
	}
	return nil
}

func (ve *Engine) validateNamespaceValue(value interface{}, fieldPath string) *Error {
	if str, ok := value.(string); ok {
		return ValidateNamespace(str, fieldPath)
	}
	return nil
}

func (ve *Engine) validateResourceNameValue(value interface{}, fieldPath string) *Error {
	if str, ok := value.(string); ok {
		return ValidateResourceName(str, fieldPath)
	}
	return nil
}

func (ve *Engine) validateNoSensitiveValue(value interface{}, fieldPath string) *Error {
	if str, ok := value.(string); ok {
		return ValidateNoSensitiveData(str, fieldPath)
	}
	return nil
}

// Helper functions

func (ve *Engine) findField(fieldPath string, val reflect.Value, typ reflect.Type, pathPrefix string) (reflect.Value, reflect.Type, string) {
	parts := strings.Split(fieldPath, ".")
	currentVal := val
	currentType := typ
	fullPath := pathPrefix

	for _, part := range parts {
		if fullPath != "" {
			fullPath += "."
		}
		fullPath += part

		// Handle struct fields
		if currentVal.Kind() == reflect.Struct {
			field := currentVal.FieldByName(part)
			if !field.IsValid() {
				return reflect.Value{}, nil, fullPath
			}

			fieldType, _ := currentType.FieldByName(part)
			currentVal = field
			currentType = fieldType.Type
		} else {
			return reflect.Value{}, nil, fullPath
		}
	}

	return currentVal, currentType, fullPath
}

func (ve *Engine) shouldSkipField(fieldPath string) bool {
	for _, skipField := range ve.options.SkipFields {
		if fieldPath == skipField || strings.HasPrefix(fieldPath, skipField+".") {
			return true
		}
	}
	return false
}

func (ve *Engine) evaluateCondition(_ string, _ interface{}) bool {
	// Simple condition evaluation - can be extended
	// For now, just return true to apply all rules
	return true
}

func (ve *Engine) toFloat64(value interface{}) *float64 {
	switch v := value.(type) {
	case float64:
		return &v
	case float32:
		f := float64(v)
		return &f
	case int:
		f := float64(v)
		return &f
	case int32:
		f := float64(v)
		return &f
	case int64:
		f := float64(v)
		return &f
	}
	return nil
}

func (ve *Engine) sanitizeResult(result *Result) {
	// Sanitize error messages
	for i := range result.Errors {
		result.Errors[i].Message = ve.sanitizer.SanitizeString(result.Errors[i].Message)
		if result.Errors[i].Value != nil {
			if str, ok := result.Errors[i].Value.(string); ok {
				result.Errors[i].Value = ve.sanitizer.SanitizeString(str)
			}
		}
	}

	// Sanitize warning messages
	for i := range result.Warnings {
		result.Warnings[i].Message = ve.sanitizer.SanitizeString(result.Warnings[i].Message)
		if result.Warnings[i].Value != nil {
			if str, ok := result.Warnings[i].Value.(string); ok {
				result.Warnings[i].Value = ve.sanitizer.SanitizeString(str)
			}
		}
	}
}

// ChainValidator allows chaining multiple validators
type ChainValidator struct {
	validators []Validator
	options    Options
}

// NewChainValidator creates a new chain validator
func NewChainValidator(options Options) *ChainValidator {
	return &ChainValidator{
		validators: make([]Validator, 0),
		options:    options,
	}
}

// Add adds a validator to the chain
func (cv *ChainValidator) Add(validator Validator) {
	cv.validators = append(cv.validators, validator)
}

// Name returns the validator name
func (cv *ChainValidator) Name() string {
	return "ChainValidator"
}

// Validate validates data using all validators in the chain
func (cv *ChainValidator) Validate(ctx context.Context, data any) Result {
	result := *NewResult()
	result.Metadata.ValidatorName = cv.Name()

	for _, validator := range cv.validators {
		validatorResult := validator.Validate(ctx, data)

		// Merge results
		result.Errors = append(result.Errors, validatorResult.Errors...)
		result.Warnings = append(result.Warnings, validatorResult.Warnings...)

		if !validatorResult.Valid {
			result.Valid = false
		}

		// Check if we should stop on first failure
		if cv.options.FailFast && !validatorResult.Valid {
			break
		}
	}

	return result
}

// ValidateWithOptions validates with options
func (cv *ChainValidator) ValidateWithOptions(ctx context.Context, data any, opts Options) Result {
	// Temporarily override options
	originalOptions := cv.options
	cv.options = opts
	defer func() { cv.options = originalOptions }()

	return cv.Validate(ctx, data)
}

// GetSupportedTypes returns supported types
func (cv *ChainValidator) GetSupportedTypes() []string {
	types := make([]string, 0)
	for _, validator := range cv.validators {
		types = append(types, validator.GetSupportedTypes()...)
	}
	return types
}

// GetVersion returns the validator version
func (cv *ChainValidator) GetVersion() string {
	return "1.0.0"
}
