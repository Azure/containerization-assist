package common

import (
	"fmt"
	"strings"
	"time"
)

type StandardizedValidationMixin struct {
	validationRules []ValidationRule
	errors          []ValidationError
}

type ValidationRule struct {
	Name        string
	Description string
	Validator   func(interface{}) error
}

func NewStandardizedValidationMixin() *StandardizedValidationMixin {
	return &StandardizedValidationMixin{
		validationRules: make([]ValidationRule, 0),
		errors:          make([]ValidationError, 0),
	}
}

func (v *StandardizedValidationMixin) AddRule(name, description string, validator func(interface{}) error) {
	rule := ValidationRule{
		Name:        name,
		Description: description,
		Validator:   validator,
	}
	v.validationRules = append(v.validationRules, rule)
}

func (v *StandardizedValidationMixin) Validate(data interface{}) ValidationResult[interface{}] {
	start := time.Now()
	v.errors = make([]ValidationError, 0)

	for _, rule := range v.validationRules {
		if err := rule.Validator(data); err != nil {
			v.errors = append(v.errors, ValidationError{
				Code:    rule.Name,
				Message: err.Error(),
				Field:   rule.Description,
			})
		}
	}

	return ValidationResult[interface{}]{
		Valid:    len(v.errors) == 0,
		Data:     data,
		Errors:   v.errors,
		Duration: time.Since(start),
	}
}

func (v *StandardizedValidationMixin) buildMessage() string {
	if len(v.errors) == 0 {
		return "Validation passed"
	}

	var messages []string
	for _, err := range v.errors {
		messages = append(messages, fmt.Sprintf("%s: %s", err.Code, err.Message))
	}
	return fmt.Sprintf("Validation failed with %d errors: %s", len(v.errors), strings.Join(messages, "; "))
}

func (v *StandardizedValidationMixin) GetRules() []ValidationRule {
	return v.validationRules
}

func (v *StandardizedValidationMixin) ClearRules() {
	v.validationRules = make([]ValidationRule, 0)
	v.errors = make([]ValidationError, 0)
}

func (v *StandardizedValidationMixin) HasErrors() bool {
	return len(v.errors) > 0
}

func (v *StandardizedValidationMixin) GetErrors() []ValidationError {
	return v.errors
}
