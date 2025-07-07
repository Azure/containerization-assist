package api

import (
	"context"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/validation"
	"github.com/stretchr/testify/assert"
)

// MockValidator implements the Validator interface for testing
type MockValidator struct {
	name            string
	version         string
	supportedTypes  []string
	validationFunc  func(interface{}) ValidationResult
	validationError error
}

func NewMockValidator(name, version string) *MockValidator {
	return &MockValidator{
		name:           name,
		version:        version,
		supportedTypes: []string{"test", "mock"},
		validationFunc: func(_ interface{}) ValidationResult {
			return ValidationResult{
				Valid:    true,
				Errors:   []ValidationError{},
				Warnings: []ValidationWarning{},
				Duration: time.Millisecond * 10,
			}
		},
	}
}

func (m *MockValidator) Name() string {
	return m.name
}

func (m *MockValidator) Validate(_ context.Context, data interface{}) ValidationResult {
	if m.validationFunc != nil {
		return m.validationFunc(data)
	}
	return ValidationResult{Valid: true}
}

func (m *MockValidator) ValidateWithOptions(ctx context.Context, data interface{}, opts validation.Options) ValidationResult {
	result := m.Validate(ctx, data)

	// Apply options to result
	if opts.StrictMode {
		// In strict mode, warnings become errors
		for _, warning := range result.Warnings {
			result.Errors = append(result.Errors, ValidationError{
				Field:   warning.Field,
				Message: warning.Message,
				Code:    "STRICT_" + warning.Code,
			})
		}
		result.Warnings = []ValidationWarning{}
		result.Valid = len(result.Errors) == 0
	}

	return result
}

func (m *MockValidator) GetSupportedTypes() []string {
	return m.supportedTypes
}

func (m *MockValidator) GetVersion() string {
	return m.version
}

func (m *MockValidator) WithValidationFunc(fn func(interface{}) ValidationResult) *MockValidator {
	m.validationFunc = fn
	return m
}

func TestValidator_Interface(t *testing.T) {
	validator := NewMockValidator("test-validator", "1.0.0")

	// Test interface compliance
	var _ validation.Validator = validator

	assert.Equal(t, "test-validator", validator.Name())
	assert.Equal(t, "1.0.0", validator.GetVersion())
	assert.Contains(t, validator.GetSupportedTypes(), "test")
	assert.Contains(t, validator.GetSupportedTypes(), "mock")
}

func TestValidator_Validate(t *testing.T) {
	tests := []struct {
		name         string
		validator    *MockValidator
		data         interface{}
		expectValid  bool
		expectErrors int
	}{
		{
			name:         "successful_validation",
			validator:    NewMockValidator("success-validator", "1.0.0"),
			data:         "valid data",
			expectValid:  true,
			expectErrors: 0,
		},
		{
			name: "validation_with_errors",
			validator: NewMockValidator("error-validator", "1.0.0").WithValidationFunc(
				func(_ interface{}) ValidationResult {
					return ValidationResult{
						Valid: false,
						Errors: []ValidationError{
							{
								Field:   "test_field",
								Message: "validation failed",
								Code:    "VALIDATION_ERROR",
							},
						},
						Duration: time.Millisecond * 5,
					}
				},
			),
			data:         "invalid data",
			expectValid:  false,
			expectErrors: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result := tt.validator.Validate(ctx, tt.data)

			assert.Equal(t, tt.expectValid, result.Valid)
			assert.Len(t, result.Errors, tt.expectErrors)
			assert.Greater(t, result.Duration, time.Duration(0))
		})
	}
}

func TestValidator_ValidateWithOptions(t *testing.T) {
	validator := NewMockValidator("options-validator", "1.0.0").WithValidationFunc(
		func(_ interface{}) ValidationResult {
			return ValidationResult{
				Valid: true,
				Warnings: []ValidationWarning{
					{
						Field:   "warning_field",
						Message: "this is a warning",
						Code:    "WARNING_CODE",
					},
				},
				Duration: time.Millisecond * 5,
			}
		},
	)

	tests := []struct {
		name         string
		options      validation.Options
		expectValid  bool
		expectErrors int
		expectWarns  int
	}{
		{
			name: "normal_mode",
			options: validation.Options{
				StrictMode: false,
			},
			expectValid:  true,
			expectErrors: 0,
			expectWarns:  1,
		},
		{
			name: "strict_mode",
			options: validation.Options{
				StrictMode: true,
			},
			expectValid:  false,
			expectErrors: 1,
			expectWarns:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result := validator.ValidateWithOptions(ctx, "test data", tt.options)

			assert.Equal(t, tt.expectValid, result.Valid)
			assert.Len(t, result.Errors, tt.expectErrors)
			assert.Len(t, result.Warnings, tt.expectWarns)
		})
	}
}

func TestValidationResult_Structure(t *testing.T) {
	result := ValidationResult{
		Valid: true,
		Errors: []ValidationError{
			{
				Field:   "test_field",
				Message: "test error",
				Code:    "TEST_ERROR",
			},
		},
		Warnings: []ValidationWarning{
			{
				Field:   "warning_field",
				Message: "test warning",
				Code:    "TEST_WARNING",
			},
		},
		Duration: time.Second,
	}

	assert.True(t, result.Valid)
	assert.Len(t, result.Errors, 1)
	assert.Len(t, result.Warnings, 1)
	assert.Equal(t, "test_field", result.Errors[0].Field)
	assert.Equal(t, "test error", result.Errors[0].Message)
	assert.Equal(t, "TEST_ERROR", result.Errors[0].Code)

	assert.Equal(t, "warning_field", result.Warnings[0].Field)
	assert.Equal(t, "test warning", result.Warnings[0].Message)
	assert.Equal(t, "TEST_WARNING", result.Warnings[0].Code)

	assert.Equal(t, time.Second, result.Duration)
}

func TestValidationError_Structure(t *testing.T) {
	validationError := ValidationError{
		Field:   "username",
		Message: "username is required",
		Code:    "REQUIRED_FIELD",
	}

	assert.Equal(t, "username", validationError.Field)
	assert.Equal(t, "username is required", validationError.Message)
	assert.Equal(t, "REQUIRED_FIELD", validationError.Code)
}

func TestValidationWarning_Structure(t *testing.T) {
	warning := ValidationWarning{
		Field:   "email",
		Message: "email format could be improved",
		Code:    "FORMAT_SUGGESTION",
	}

	assert.Equal(t, "email", warning.Field)
	assert.Equal(t, "email format could be improved", warning.Message)
	assert.Equal(t, "FORMAT_SUGGESTION", warning.Code)
}

func TestValidationOptions_Structure(t *testing.T) {
	options := validation.Options{
		StrictMode: true,
		MaxErrors:  10,
		Timeout:    time.Minute * 5,
		Context:    map[string]string{"environment": "test"},
	}

	assert.True(t, options.StrictMode)
	assert.Equal(t, 10, options.MaxErrors)
	assert.Equal(t, time.Minute*5, options.Timeout)
	assert.Equal(t, "test", options.Context["environment"])
}

func TestValidator_ContextHandling(t *testing.T) {
	validator := NewMockValidator("context-validator", "1.0.0")

	// Test with canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := validator.Validate(ctx, "test data")

	// Should still return a result even with canceled context
	assert.NotNil(t, result)
}

func TestValidator_ErrorHandling(t *testing.T) {
	validator := NewMockValidator("error-validator", "1.0.0").WithValidationFunc(
		func(_ interface{}) ValidationResult {
			return ValidationResult{
				Valid: false,
				Errors: []ValidationError{
					{
						Field:   "critical_field",
						Message: "critical validation error",
						Code:    "CRITICAL_ERROR",
					},
					{
						Field:   "minor_field",
						Message: "minor validation issue",
						Code:    "MINOR_ERROR",
					},
				},
			}
		},
	)

	ctx := context.Background()
	result := validator.Validate(ctx, "invalid data")

	assert.False(t, result.Valid)
	assert.Len(t, result.Errors, 2)

	// Should have the expected number of errors
	assert.Len(t, result.Errors, 2)
}

// BenchmarkValidator_Validate benchmarks the validation process
func BenchmarkValidator_Validate(b *testing.B) {
	validator := NewMockValidator("benchmark-validator", "1.0.0")
	ctx := context.Background()
	data := "benchmark test data"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := validator.Validate(ctx, data)
		_ = result
	}
}

// BenchmarkValidator_ValidateWithOptions benchmarks validation with options
func BenchmarkValidator_ValidateWithOptions(b *testing.B) {
	validator := NewMockValidator("benchmark-options-validator", "1.0.0")
	ctx := context.Background()
	data := "benchmark test data"
	options := validation.Options{
		StrictMode: true,
		MaxErrors:  100,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := validator.ValidateWithOptions(ctx, data, options)
		_ = result
	}
}
