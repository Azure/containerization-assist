package validation

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// TagParser parses struct tags and generates validation rules
type TagParser struct {
	tagName string
}

// NewTagParser creates a new tag parser
func NewTagParser() *TagParser {
	return &TagParser{
		tagName: "validate",
	}
}

// ParseStruct parses validation tags from a struct and returns validation rules
func (tp *TagParser) ParseStruct(structType reflect.Type) ([]ValidationRule, error) {
	if structType.Kind() == reflect.Ptr {
		structType = structType.Elem()
	}

	if structType.Kind() != reflect.Struct {
		return nil, errors.Validationf("tag_parser", "expected struct type, got %s", structType.Kind())
	}

	var rules []ValidationRule

	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Get validation tag
		tag := field.Tag.Get(tp.tagName)
		if tag == "" {
			continue
		}

		// Parse validation rules for this field
		fieldRules, err := tp.parseFieldTag(field.Name, field.Type, tag)
		if err != nil {
			return nil, errors.Wrapf(err, "tag_parser", "failed to parse tag for field %s", field.Name)
		}

		rules = append(rules, fieldRules...)
	}

	return rules, nil
}

// ValidationRule represents a single validation rule parsed from a tag
type ValidationRule struct {
	FieldName   string
	FieldType   reflect.Type
	RuleName    string
	Parameters  map[string]interface{}
	Required    bool
	Optional    bool
	Conditional *ConditionalRule
}

// ConditionalRule represents conditional validation (required_if, etc.)
type ConditionalRule struct {
	Type      string // "required_if", "required_unless", etc.
	Field     string
	Value     interface{}
	Operation string // "eq", "ne", "gt", "lt", etc.
}

// parseFieldTag parses a single field's validation tag
func (tp *TagParser) parseFieldTag(fieldName string, fieldType reflect.Type, tag string) ([]ValidationRule, error) {
	// Split tag by commas, but handle commas within parameters
	rules := tp.splitTag(tag)
	var validationRules []ValidationRule

	required := false
	optional := false

	for _, rule := range rules {
		rule = strings.TrimSpace(rule)
		if rule == "" {
			continue
		}

		// Handle special cases
		switch rule {
		case "required":
			required = true
			validationRules = append(validationRules, ValidationRule{
				FieldName:  fieldName,
				FieldType:  fieldType,
				RuleName:   "required",
				Parameters: make(map[string]interface{}),
				Required:   true,
			})
			continue
		case "omitempty":
			optional = true
			continue
		case "dive":
			// Handle array/slice validation
			validationRules = append(validationRules, ValidationRule{
				FieldName:  fieldName,
				FieldType:  fieldType,
				RuleName:   "dive",
				Parameters: make(map[string]interface{}),
			})
			continue
		}

		// Parse rule with parameters
		ruleName, params, err := tp.parseRuleWithParams(rule)
		if err != nil {
			return nil, errors.Wrapf(err, "tag_parser", "failed to parse rule: %s", rule)
		}

		// Handle conditional rules
		if tp.isConditionalRule(ruleName) {
			conditionalRule, err := tp.parseConditionalRule(ruleName, params)
			if err != nil {
				return nil, err
			}

			validationRules = append(validationRules, ValidationRule{
				FieldName:   fieldName,
				FieldType:   fieldType,
				RuleName:    ruleName,
				Parameters:  params,
				Conditional: conditionalRule,
			})
			continue
		}

		validationRules = append(validationRules, ValidationRule{
			FieldName:  fieldName,
			FieldType:  fieldType,
			RuleName:   ruleName,
			Parameters: params,
			Required:   required,
			Optional:   optional,
		})
	}

	return validationRules, nil
}

// splitTag splits a validation tag by commas, handling quoted values
func (tp *TagParser) splitTag(tag string) []string {
	var rules []string
	var current strings.Builder
	inQuotes := false
	escapeNext := false

	for _, char := range tag {
		if escapeNext {
			current.WriteRune(char)
			escapeNext = false
			continue
		}

		switch char {
		case '\\':
			escapeNext = true
			current.WriteRune(char)
		case '"', '\'':
			inQuotes = !inQuotes
			current.WriteRune(char)
		case ',':
			if inQuotes {
				current.WriteRune(char)
			} else {
				rules = append(rules, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(char)
		}
	}

	if current.Len() > 0 {
		rules = append(rules, current.String())
	}

	return rules
}

// parseRuleWithParams parses a rule that may have parameters
func (tp *TagParser) parseRuleWithParams(rule string) (string, map[string]interface{}, error) {
	params := make(map[string]interface{})

	// Check if rule has parameters (contains =)
	if !strings.Contains(rule, "=") {
		return rule, params, nil
	}

	parts := strings.SplitN(rule, "=", 2)
	if len(parts) != 2 {
		return rule, params, nil
	}

	ruleName := strings.TrimSpace(parts[0])
	paramValue := strings.TrimSpace(parts[1])

	// Parse the parameter value based on the rule type
	switch ruleName {
	case "min", "max":
		// Numeric parameters
		if val, err := strconv.ParseFloat(paramValue, 64); err == nil {
			params["value"] = val
		} else {
			return "", nil, errors.Validationf("tag_parser", "invalid numeric value for %s: %s", ruleName, paramValue)
		}

	case "len":
		// Length parameter
		if val, err := strconv.Atoi(paramValue); err == nil {
			params["length"] = val
		} else {
			return "", nil, errors.Validationf("tag_parser", "invalid length value: %s", paramValue)
		}

	case "oneof":
		// Multiple choice parameter
		choices := strings.Fields(paramValue)
		params["choices"] = choices

	case "regex":
		// Regex pattern parameter
		if _, err := regexp.Compile(paramValue); err != nil {
			return "", nil, errors.Wrapf(err, "tag_parser", "invalid regex pattern: %s", paramValue)
		}
		params["pattern"] = paramValue

	case "required_if", "required_unless":
		// Conditional parameters: field value
		condParts := strings.Fields(paramValue)
		if len(condParts) != 2 {
			return "", nil, errors.Validationf("tag_parser", "invalid conditional format: %s", paramValue)
		}
		params["field"] = condParts[0]
		params["value"] = condParts[1]

	case "eqfield", "nefield", "gtfield", "ltfield", "gtefield", "ltefield":
		// Field comparison parameters
		params["field"] = paramValue

	default:
		// Generic string parameter
		params["value"] = paramValue
	}

	return ruleName, params, nil
}

// isConditionalRule checks if a rule is conditional
func (tp *TagParser) isConditionalRule(ruleName string) bool {
	conditionalRules := []string{
		"required_if", "required_unless", "required_with", "required_without",
		"eqfield", "nefield", "gtfield", "ltfield", "gtefield", "ltefield",
	}

	for _, cond := range conditionalRules {
		if cond == ruleName {
			return true
		}
	}
	return false
}

// parseConditionalRule parses conditional rule parameters
func (tp *TagParser) parseConditionalRule(ruleName string, params map[string]interface{}) (*ConditionalRule, error) {
	switch ruleName {
	case "required_if", "required_unless":
		field, ok := params["field"].(string)
		if !ok {
			return nil, errors.Validationf("tag_parser", "missing field for conditional rule %s", ruleName)
		}
		value := params["value"]

		return &ConditionalRule{
			Type:      ruleName,
			Field:     field,
			Value:     value,
			Operation: "eq",
		}, nil

	case "eqfield", "nefield":
		field, ok := params["field"].(string)
		if !ok {
			return nil, errors.Validationf("tag_parser", "missing field for conditional rule %s", ruleName)
		}

		operation := "eq"
		if ruleName == "nefield" {
			operation = "ne"
		}

		return &ConditionalRule{
			Type:      "field_comparison",
			Field:     field,
			Operation: operation,
		}, nil

	case "gtfield", "ltfield", "gtefield", "ltefield":
		field, ok := params["field"].(string)
		if !ok {
			return nil, errors.Validationf("tag_parser", "missing field for conditional rule %s", ruleName)
		}

		var operation string
		switch ruleName {
		case "gtfield":
			operation = "gt"
		case "ltfield":
			operation = "lt"
		case "gtefield":
			operation = "gte"
		case "ltefield":
			operation = "lte"
		}

		return &ConditionalRule{
			Type:      "field_comparison",
			Field:     field,
			Operation: operation,
		}, nil

	default:
		return nil, errors.Validationf("tag_parser", "unknown conditional rule: %s", ruleName)
	}
}

// ValidatorFromRules creates a validator function from parsed rules
func (tp *TagParser) ValidatorFromRules(rules []ValidationRule) StructValidatorFunc {
	return func(structValue reflect.Value) error {
		for _, rule := range rules {
			if err := tp.applyValidationRule(structValue, rule); err != nil {
				return err
			}
		}
		return nil
	}
}

// applyValidationRule applies a single validation rule to a struct value
func (tp *TagParser) applyValidationRule(structValue reflect.Value, rule ValidationRule) error {
	// Get the field value
	fieldValue := structValue.FieldByName(rule.FieldName)
	if !fieldValue.IsValid() {
		return errors.Validationf("tag_parser", "field %s not found", rule.FieldName)
	}

	// Handle conditional rules
	if rule.Conditional != nil {
		shouldValidate, err := tp.evaluateConditional(structValue, rule.Conditional)
		if err != nil {
			return err
		}
		if !shouldValidate {
			return nil // Skip validation
		}
	}

	// Handle optional fields
	if rule.Optional && tp.isZeroValue(fieldValue) {
		return nil // Skip validation for zero values when optional
	}

	// Apply the specific validation rule
	return tp.applySpecificRule(fieldValue, rule)
}

// evaluateConditional evaluates a conditional rule
func (tp *TagParser) evaluateConditional(structValue reflect.Value, cond *ConditionalRule) (bool, error) {
	switch cond.Type {
	case "required_if":
		fieldValue := structValue.FieldByName(cond.Field)
		if !fieldValue.IsValid() {
			return false, errors.Validationf("tag_parser", "conditional field %s not found", cond.Field)
		}
		return tp.compareValues(fieldValue.Interface(), cond.Value, "eq"), nil

	case "required_unless":
		fieldValue := structValue.FieldByName(cond.Field)
		if !fieldValue.IsValid() {
			return false, errors.Validationf("tag_parser", "conditional field %s not found", cond.Field)
		}
		return !tp.compareValues(fieldValue.Interface(), cond.Value, "eq"), nil

	case "field_comparison":
		fieldValue := structValue.FieldByName(cond.Field)
		if !fieldValue.IsValid() {
			return false, errors.Validationf("tag_parser", "comparison field %s not found", cond.Field)
		}
		// For field comparisons, we always validate (the comparison is the validation)
		return true, nil

	default:
		return false, errors.Validationf("tag_parser", "unknown conditional type: %s", cond.Type)
	}
}

// compareValues compares two values using the specified operation
func (tp *TagParser) compareValues(a, b interface{}, operation string) bool {
	switch operation {
	case "eq":
		return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
	case "ne":
		return fmt.Sprintf("%v", a) != fmt.Sprintf("%v", b)
	// Add more operations as needed
	default:
		return false
	}
}

// isZeroValue checks if a reflect.Value is the zero value for its type
func (tp *TagParser) isZeroValue(v reflect.Value) bool {
	return v.IsZero()
}

// applySpecificRule applies a specific validation rule
func (tp *TagParser) applySpecificRule(fieldValue reflect.Value, rule ValidationRule) error {
	fieldInterface := fieldValue.Interface()
	fieldName := rule.FieldName

	switch rule.RuleName {
	case "required":
		if tp.isZeroValue(fieldValue) {
			return errors.Validationf("validation", "%s is required", fieldName)
		}

	case "min":
		if minVal, ok := rule.Parameters["value"].(float64); ok {
			return tp.validateMinValue(fieldInterface, fieldName, minVal)
		}

	case "max":
		if maxVal, ok := rule.Parameters["value"].(float64); ok {
			return tp.validateMaxValue(fieldInterface, fieldName, maxVal)
		}

	case "len":
		if length, ok := rule.Parameters["length"].(int); ok {
			return tp.validateLength(fieldInterface, fieldName, length)
		}

	case "oneof":
		if choices, ok := rule.Parameters["choices"].([]string); ok {
			return tp.validateOneOf(fieldInterface, fieldName, choices)
		}

	case "email":
		return tp.validateEmail(fieldInterface, fieldName)

	case "url":
		return tp.validateURL(fieldInterface, fieldName)

	case "regex":
		if pattern, ok := rule.Parameters["pattern"].(string); ok {
			return tp.validateRegex(fieldInterface, fieldName, pattern)
		}

	case "image_name":
		return tp.validateImageName(fieldInterface, fieldName)

	case "tag_format":
		return tp.validateTagFormat(fieldInterface, fieldName)

	case "k8s_name":
		return tp.validateK8sName(fieldInterface, fieldName)

	case "port":
		return tp.validatePort(fieldInterface, fieldName)

	case "dive":
		return tp.validateDive(fieldValue, rule)

	default:
		return errors.Validationf("validation", "unknown validation rule: %s", rule.RuleName)
	}

	return nil
}

// Validation helper methods

func (tp *TagParser) validateMinValue(value interface{}, fieldName string, minVal float64) error {
	switch v := value.(type) {
	case int:
		if float64(v) < minVal {
			return errors.Validationf("validation", "%s must be at least %g", fieldName, minVal)
		}
	case float64:
		if v < minVal {
			return errors.Validationf("validation", "%s must be at least %g", fieldName, minVal)
		}
	case string:
		if float64(len(v)) < minVal {
			return errors.Validationf("validation", "%s must be at least %g characters", fieldName, minVal)
		}
	}
	return nil
}

func (tp *TagParser) validateMaxValue(value interface{}, fieldName string, maxVal float64) error {
	switch v := value.(type) {
	case int:
		if float64(v) > maxVal {
			return errors.Validationf("validation", "%s must be at most %g", fieldName, maxVal)
		}
	case float64:
		if v > maxVal {
			return errors.Validationf("validation", "%s must be at most %g", fieldName, maxVal)
		}
	case string:
		if float64(len(v)) > maxVal {
			return errors.Validationf("validation", "%s must be at most %g characters", fieldName, maxVal)
		}
	}
	return nil
}

func (tp *TagParser) validateLength(value interface{}, fieldName string, expectedLen int) error {
	if str, ok := value.(string); ok {
		if len(str) != expectedLen {
			return errors.Validationf("validation", "%s must be exactly %d characters", fieldName, expectedLen)
		}
	}
	return nil
}

func (tp *TagParser) validateOneOf(value interface{}, fieldName string, choices []string) error {
	if str, ok := value.(string); ok {
		for _, choice := range choices {
			if str == choice {
				return nil
			}
		}
		return errors.Validationf("validation", "%s must be one of: %s", fieldName, strings.Join(choices, ", "))
	}
	return nil
}

func (tp *TagParser) validateEmail(value interface{}, fieldName string) error {
	if str, ok := value.(string); ok {
		emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
		if !emailRegex.MatchString(str) {
			return errors.Validationf("validation", "%s must be a valid email address", fieldName)
		}
	}
	return nil
}

func (tp *TagParser) validateURL(value interface{}, fieldName string) error {
	if str, ok := value.(string); ok {
		if !strings.HasPrefix(str, "http://") && !strings.HasPrefix(str, "https://") {
			return errors.Validationf("validation", "%s must be a valid URL", fieldName)
		}
	}
	return nil
}

func (tp *TagParser) validateRegex(value interface{}, fieldName, pattern string) error {
	if str, ok := value.(string); ok {
		regex, err := regexp.Compile(pattern)
		if err != nil {
			return errors.Wrapf(err, "validation", "invalid regex pattern for %s", fieldName)
		}
		if !regex.MatchString(str) {
			return errors.Validationf("validation", "%s must match pattern %s", fieldName, pattern)
		}
	}
	return nil
}

func (tp *TagParser) validateImageName(value interface{}, fieldName string) error {
	if str, ok := value.(string); ok {
		// Docker image name validation
		imageRegex := regexp.MustCompile(`^([a-zA-Z0-9._-]+/)?[a-zA-Z0-9._-]+(/[a-zA-Z0-9._-]+)*(:([a-zA-Z0-9._-]+))?$`)
		if !imageRegex.MatchString(str) {
			return errors.Validationf("validation", "%s must be a valid Docker image name", fieldName)
		}
	}
	return nil
}

func (tp *TagParser) validateTagFormat(value interface{}, fieldName string) error {
	if str, ok := value.(string); ok {
		// Docker tag validation
		tagRegex := regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
		if !tagRegex.MatchString(str) {
			return errors.Validationf("validation", "%s must be a valid Docker tag", fieldName)
		}
	}
	return nil
}

func (tp *TagParser) validateK8sName(value interface{}, fieldName string) error {
	if str, ok := value.(string); ok {
		// Kubernetes name validation (RFC 1123)
		k8sRegex := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
		if !k8sRegex.MatchString(str) || len(str) > 63 {
			return errors.Validationf("validation", "%s must be a valid Kubernetes name", fieldName)
		}
	}
	return nil
}

func (tp *TagParser) validatePort(value interface{}, fieldName string) error {
	if port, ok := value.(int); ok {
		if port < 1 || port > 65535 {
			return errors.Validationf("validation", "%s must be a valid port number (1-65535)", fieldName)
		}
	}
	return nil
}

func (tp *TagParser) validateDive(fieldValue reflect.Value, rule ValidationRule) error {
	// Handle array/slice validation by applying validation to each element
	if fieldValue.Kind() == reflect.Slice || fieldValue.Kind() == reflect.Array {
		for i := 0; i < fieldValue.Len(); i++ {
			elemValue := fieldValue.Index(i)
			// For dive, we would need to apply nested validation rules
			// This is a simplified implementation
			if tp.isZeroValue(elemValue) {
				return errors.Validationf("validation", "%s[%d] is required", rule.FieldName, i)
			}
		}
	}
	return nil
}
