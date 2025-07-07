package validation

import (
	"context"
	"testing"
	"time"
)

// BenchmarkValidationFramework benchmarks the validation framework performance
func BenchmarkValidationFramework(b *testing.B) {
	ctx := context.Background()

	// Test data
	testInput := map[string]interface{}{
		"field1": "valid_value",
		"field2": 42,
		"field3": true,
	}

	// Create a simple test validator
	validator := &testValidator{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := validator.Validate(ctx, testInput)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkValidationWithRules tests performance with multiple validation rules
func BenchmarkValidationWithRules(b *testing.B) {
	ctx := context.Background()

	testInput := map[string]interface{}{
		"field1": "valid_value",
		"field2": 42,
		"field3": true,
		"field4": "another_value",
		"field5": 123.45,
	}

	validator := &complexTestValidator{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := validator.Validate(ctx, testInput)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkValidationError tests performance when validation fails
func BenchmarkValidationError(b *testing.B) {
	ctx := context.Background()

	testInput := map[string]interface{}{
		"field1": "", // Invalid - empty string
		"field2": -1, // Invalid - negative number
	}

	validator := &testValidator{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.Validate(ctx, testInput)
		// We expect this to return an error, which is normal
	}
}

// testValidator implements a simple validator for benchmarking
type testValidator struct{}

func (v *testValidator) Validate(ctx context.Context, input interface{}) error {
	data, ok := input.(map[string]interface{})
	if !ok {
		return &ValidationError{
			Field:   "input",
			Code:    "INVALID_TYPE",
			Message: "Input must be a map",
		}
	}

	// Simple validations
	if field1, exists := data["field1"]; exists {
		if str, ok := field1.(string); ok && str == "" {
			return &ValidationError{
				Field:   "field1",
				Code:    "REQUIRED_FIELD",
				Message: "Field1 cannot be empty",
			}
		}
	}

	if field2, exists := data["field2"]; exists {
		if num, ok := field2.(int); ok && num < 0 {
			return &ValidationError{
				Field:   "field2",
				Code:    "INVALID_VALUE",
				Message: "Field2 must be non-negative",
			}
		}
	}

	return nil
}

// complexTestValidator implements a more complex validator for performance testing
type complexTestValidator struct{}

func (v *complexTestValidator) Validate(ctx context.Context, input interface{}) error {
	data, ok := input.(map[string]interface{})
	if !ok {
		return &ValidationError{
			Field:   "input",
			Code:    "INVALID_TYPE",
			Message: "Input must be a map",
		}
	}

	// More complex validations with multiple rules
	rules := []ValidationRule{
		{
			Name:        "field1_validation",
			Description: "Field1 string validation",
			Severity:    "medium",
			Category:    "input",
			Enabled:     true,
			Config: ValidationRuleConfig{
				Required:  true,
				MinLength: 1,
			},
		},
		{
			Name:        "field2_validation",
			Description: "Field2 integer validation",
			Severity:    "medium",
			Category:    "input",
			Enabled:     true,
			Config: ValidationRuleConfig{
				Required: true,
			},
		},
		{
			Name:        "field3_validation",
			Description: "Field3 boolean validation",
			Severity:    "low",
			Category:    "input",
			Enabled:     true,
			Config: ValidationRuleConfig{
				Required: false,
			},
		},
		{
			Name:        "field4_validation",
			Description: "Field4 string length validation",
			Severity:    "medium",
			Category:    "input",
			Enabled:     true,
			Config: ValidationRuleConfig{
				Required:  false,
				MaxLength: 100,
			},
		},
		{
			Name:        "field5_validation",
			Description: "Field5 float validation",
			Severity:    "medium",
			Category:    "input",
			Enabled:     true,
			Config: ValidationRuleConfig{
				Required: false,
			},
		},
	}

	return v.validateWithRules(data, rules)
}

func (v *complexTestValidator) validateWithRules(data map[string]interface{}, rules []ValidationRule) error {
	// Simple validation simulation using field names from rules
	fieldMap := map[string]ValidationRuleConfig{
		"field1_validation": rules[0].Config,
		"field2_validation": rules[1].Config,
		"field3_validation": rules[2].Config,
		"field4_validation": rules[3].Config,
		"field5_validation": rules[4].Config,
	}

	// Check field1
	if config, exists := fieldMap["field1_validation"]; exists && config.Required {
		if value, ok := data["field1"]; !ok || value == "" {
			return &ValidationError{
				Field:   "field1",
				Code:    "REQUIRED_FIELD",
				Message: "Field is required",
			}
		}
	}

	// Check field2
	if config, exists := fieldMap["field2_validation"]; exists && config.Required {
		if value, ok := data["field2"]; !ok {
			return &ValidationError{
				Field:   "field2",
				Code:    "REQUIRED_FIELD",
				Message: "Field is required",
			}
		} else if num, ok := value.(int); ok && num < 0 {
			return &ValidationError{
				Field:   "field2",
				Code:    "MIN_VALUE",
				Message: "Value must be non-negative",
			}
		}
	}

	// Check field4 for max length
	if config, exists := fieldMap["field4_validation"]; exists && config.MaxLength > 0 {
		if value, ok := data["field4"]; ok {
			if str, ok := value.(string); ok && len(str) > config.MaxLength {
				return &ValidationError{
					Field:   "field4",
					Code:    "MAX_LENGTH",
					Message: "String too long",
				}
			}
		}
	}

	return nil
}

// BenchmarkValidationWithTimeout tests validation with context timeout
func BenchmarkValidationWithTimeout(b *testing.B) {
	// Create context with short timeout to test cancellation handling
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Microsecond)
	defer cancel()

	testInput := map[string]interface{}{
		"field1": "valid_value",
		"field2": 42,
	}

	validator := &timeoutTestValidator{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.Validate(ctx, testInput)
		// We expect this might timeout, which is normal for this benchmark
	}
}

// timeoutTestValidator tests context cancellation handling
type timeoutTestValidator struct{}

func (v *timeoutTestValidator) Validate(ctx context.Context, input interface{}) error {
	// Check context cancellation before processing
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Continue with validation
	}

	// Simulate some validation work
	time.Sleep(2 * time.Microsecond) // This will trigger timeout in benchmark

	return nil
}
