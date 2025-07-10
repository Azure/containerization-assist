package security

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// StructValidator provides validation for structs using tags
type StructValidator struct {
	tagValidator *TagBasedValidator
}

// NewStructValidator creates a new struct validator
func NewStructValidator() *StructValidator {
	return &StructValidator{
		tagValidator: GetGlobalTagValidator(),
	}
}

// ValidateField validates a single field value with validation rules
func (sv *StructValidator) ValidateField(ctx context.Context, fieldName string, value interface{}, rules string) error {
	return sv.tagValidator.ValidateField(ctx, fieldName, value, rules)
}

// Validate validates a struct based on its tags
func (sv *StructValidator) Validate(data interface{}) error {
	if data == nil {
		return nil
	}

	v := reflect.ValueOf(data)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return errors.NewError().
			Code(errors.CodeValidationFailed).
			Message("expected struct type").
			Build()
	}

	return sv.ValidateStruct(context.Background(), v)
}

// ValidateStruct validates a struct recursively
func (sv *StructValidator) ValidateStruct(ctx context.Context, v reflect.Value) error {
	t := v.Type()
	var validationErrors []error

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)

		// Skip unexported fields
		if !fieldValue.CanInterface() {
			continue
		}

		// Get validation tag
		tag := field.Tag.Get("validate")
		if tag == "" || tag == "-" {
			continue
		}

		// Validate the field
		if err := sv.tagValidator.ValidateField(ctx, field.Name, fieldValue.Interface(), tag); err != nil {
			validationErrors = append(validationErrors, err)
		}
	}

	if len(validationErrors) > 0 {
		return sv.combineErrors(validationErrors)
	}

	return nil
}

// combineErrors combines multiple validation errors
func (sv *StructValidator) combineErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	if len(errs) == 1 {
		return errs[0]
	}

	var messages []string
	for _, err := range errs {
		messages = append(messages, err.Error())
	}

	return errors.NewError().
		Code(errors.CodeValidationFailed).
		Message(fmt.Sprintf("validation failed: %s", strings.Join(messages, "; "))).
		Build()
}

// ValidateTaggedStruct is a convenience function for validating structs with tags
func ValidateTaggedStruct(data interface{}) error {
	validator := NewStructValidator()
	return validator.Validate(data)
}
