package validation

import (
	"reflect"
	"strconv"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/errors"
)

// TagParser parses and interprets validation tags on struct fields
type TagParser struct {
	tagName string // Default "validate"
}

// NewTagParser creates a new tag parser
func NewTagParser() *TagParser {
	return &TagParser{
		tagName: "validate",
	}
}

// ValidationRule represents a single validation rule extracted from a tag
type ValidationRule struct {
	Name       string            `json:"name"`
	Parameters map[string]string `json:"parameters,omitempty"`
	IsRequired bool              `json:"is_required"`
}

// FieldValidationRules contains all validation rules for a single field
type FieldValidationRules struct {
	FieldName string           `json:"field_name"`
	FieldType string           `json:"field_type"`
	Rules     []ValidationRule `json:"rules"`
	IsSlice   bool             `json:"is_slice"`
	IsMap     bool             `json:"is_map"`
	IsStruct  bool             `json:"is_struct"`
}

// StructValidationRules contains validation rules for an entire struct
type StructValidationRules struct {
	TypeName string                 `json:"type_name"`
	Package  string                 `json:"package"`
	Fields   []FieldValidationRules `json:"fields"`
}

// ParseStruct extracts validation rules from all fields in a struct
func (tp *TagParser) ParseStruct(structType reflect.Type) (*StructValidationRules, error) {
	if structType.Kind() != reflect.Struct {
		return nil, errors.NewError().
			Code(errors.CodeInvalidParameter).
			Type(errors.ErrTypeValidation).
			Severity(errors.SeverityMedium).
			Message("provided type is not a struct").
			Context("type_kind", structType.Kind().String()).
			Suggestion("Provide a struct type for validation parsing").
			WithLocation().
			Build()
	}

	rules := &StructValidationRules{
		TypeName: structType.Name(),
		Package:  structType.PkgPath(),
		Fields:   make([]FieldValidationRules, 0),
	}

	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Get validation tag
		tag := field.Tag.Get(tp.tagName)
		if tag == "" {
			continue // No validation rules for this field
		}

		fieldRules, err := tp.parseFieldRules(field.Name, field.Type, tag)
		if err != nil {
			return nil, errors.NewError().
				Code(errors.CodeValidationFailed).
				Type(errors.ErrTypeValidation).
				Severity(errors.SeverityMedium).
				Messagef("failed to parse validation rules for field %s", field.Name).
				Context("field_name", field.Name).
				Context("tag_value", tag).
				Cause(err).
				WithLocation().
				Build()
		}

		rules.Fields = append(rules.Fields, *fieldRules)
	}

	return rules, nil
}

// parseFieldRules parses validation rules for a single field
func (tp *TagParser) parseFieldRules(fieldName string, fieldType reflect.Type, tag string) (*FieldValidationRules, error) {
	fieldRules := &FieldValidationRules{
		FieldName: fieldName,
		FieldType: fieldType.String(),
		Rules:     make([]ValidationRule, 0),
		IsSlice:   fieldType.Kind() == reflect.Slice || fieldType.Kind() == reflect.Array,
		IsMap:     fieldType.Kind() == reflect.Map,
		IsStruct:  fieldType.Kind() == reflect.Struct,
	}

	// Handle pointer types
	if fieldType.Kind() == reflect.Ptr {
		fieldType = fieldType.Elem()
		fieldRules.FieldType = "*" + fieldType.String()
		fieldRules.IsStruct = fieldType.Kind() == reflect.Struct
	}

	// Parse individual rules from tag
	rules := strings.Split(tag, ",")
	for _, rule := range rules {
		rule = strings.TrimSpace(rule)
		if rule == "" {
			continue
		}

		validationRule, err := tp.parseRule(rule)
		if err != nil {
			return nil, errors.NewError().
				Code(errors.CodeValidationFailed).
				Type(errors.ErrTypeValidation).
				Severity(errors.SeverityMedium).
				Messagef("failed to parse validation rule: %s", rule).
				Context("rule", rule).
				Context("field_name", fieldName).
				Cause(err).
				WithLocation().
				Build()
		}

		fieldRules.Rules = append(fieldRules.Rules, *validationRule)
	}

	return fieldRules, nil
}

// parseRule parses a single validation rule string
func (tp *TagParser) parseRule(rule string) (*ValidationRule, error) {
	validationRule := &ValidationRule{
		Parameters: make(map[string]string),
		IsRequired: false,
	}

	// Check for parameters (e.g., "min=5", "max=100")
	if strings.Contains(rule, "=") {
		parts := strings.SplitN(rule, "=", 2)
		if len(parts) != 2 {
			return nil, errors.NewError().
				Code(errors.CodeValidationFailed).
				Type(errors.ErrTypeValidation).
				Severity(errors.SeverityMedium).
				Messagef("invalid rule format: %s", rule).
				Context("rule", rule).
				Suggestion("Use format 'rule=value' for parameterized rules").
				WithLocation().
				Build()
		}

		validationRule.Name = strings.TrimSpace(parts[0])
		paramValue := strings.TrimSpace(parts[1])

		// Handle different parameter formats
		if err := tp.parseParameters(validationRule, paramValue); err != nil {
			return nil, err
		}
	} else {
		// Simple rule without parameters (e.g., "required", "email")
		validationRule.Name = rule
	}

	// Mark certain rules as required
	validationRule.IsRequired = tp.isRequiredRule(validationRule.Name)

	return validationRule, nil
}

// parseParameters parses parameter values for validation rules
func (tp *TagParser) parseParameters(rule *ValidationRule, paramValue string) error {
	switch rule.Name {
	case "min", "max":
		// Numeric parameters
		if _, err := strconv.ParseFloat(paramValue, 64); err != nil {
			return errors.NewError().
				Code(errors.CodeInvalidParameter).
				Type(errors.ErrTypeValidation).
				Severity(errors.SeverityMedium).
				Messagef("invalid numeric parameter for %s: %s", rule.Name, paramValue).
				Context("rule_name", rule.Name).
				Context("parameter_value", paramValue).
				Suggestion("Provide a valid numeric value").
				WithLocation().
				Build()
		}
		rule.Parameters["value"] = paramValue

	case "len":
		// Length parameter
		if _, err := strconv.Atoi(paramValue); err != nil {
			return errors.NewError().
				Code(errors.CodeInvalidParameter).
				Type(errors.ErrTypeValidation).
				Severity(errors.SeverityMedium).
				Messagef("invalid length parameter: %s", paramValue).
				Context("parameter_value", paramValue).
				Suggestion("Provide a valid integer for length").
				WithLocation().
				Build()
		}
		rule.Parameters["length"] = paramValue

	case "regex":
		// Regular expression parameter
		rule.Parameters["pattern"] = paramValue

	case "oneof":
		// One-of values (comma or space separated)
		values := strings.Fields(strings.ReplaceAll(paramValue, ",", " "))
		rule.Parameters["values"] = strings.Join(values, ",")

	case "required_if", "required_unless":
		// Conditional requirements
		parts := strings.Fields(paramValue)
		if len(parts) != 2 {
			return errors.NewError().
				Code(errors.CodeInvalidParameter).
				Type(errors.ErrTypeValidation).
				Severity(errors.SeverityMedium).
				Messagef("invalid conditional parameter format: %s", paramValue).
				Context("parameter_value", paramValue).
				Suggestion("Use format 'field_name value' for conditional rules").
				WithLocation().
				Build()
		}
		rule.Parameters["field"] = parts[0]
		rule.Parameters["value"] = parts[1]

	case "gtfield", "ltfield", "eqfield", "nefield":
		// Field comparison rules
		rule.Parameters["field"] = paramValue

	default:
		// Generic parameter
		rule.Parameters["value"] = paramValue
	}

	return nil
}

// isRequiredRule determines if a rule makes a field required
func (tp *TagParser) isRequiredRule(ruleName string) bool {
	requiredRules := map[string]bool{
		"required":      true,
		"required_if":   true,
		"required_with": true,
	}
	return requiredRules[ruleName]
}

// GetValidationRules is a convenience method to get validation rules for a struct type
func GetValidationRules(structType reflect.Type) (*StructValidationRules, error) {
	parser := NewTagParser()
	return parser.ParseStruct(structType)
}

// HasValidationTags checks if a struct type has any validation tags
func HasValidationTags(structType reflect.Type) bool {
	if structType.Kind() != reflect.Struct {
		return false
	}

	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		if field.IsExported() && field.Tag.Get("validate") != "" {
			return true
		}
	}
	return false
}

// GetFieldValidationRules gets validation rules for a specific field
func GetFieldValidationRules(structType reflect.Type, fieldName string) (*FieldValidationRules, error) {
	field, found := structType.FieldByName(fieldName)
	if !found {
		return nil, errors.NewError().
			Code(errors.CodeNotFound).
			Type(errors.ErrTypeValidation).
			Severity(errors.SeverityMedium).
			Messagef("field %s not found in struct %s", fieldName, structType.Name()).
			Context("field_name", fieldName).
			Context("struct_type", structType.Name()).
			WithLocation().
			Build()
	}

	tag := field.Tag.Get("validate")
	if tag == "" {
		return &FieldValidationRules{
			FieldName: fieldName,
			FieldType: field.Type.String(),
			Rules:     []ValidationRule{},
		}, nil
	}

	parser := NewTagParser()
	return parser.parseFieldRules(fieldName, field.Type, tag)
}

// ValidateTagSyntax validates that a validation tag has correct syntax
func ValidateTagSyntax(tag string) error {
	if tag == "" {
		return nil
	}

	parser := NewTagParser()
	rules := strings.Split(tag, ",")

	for _, rule := range rules {
		rule = strings.TrimSpace(rule)
		if rule == "" {
			continue
		}

		_, err := parser.parseRule(rule)
		if err != nil {
			return err
		}
	}

	return nil
}
