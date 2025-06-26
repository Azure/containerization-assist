package runtime

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Mock validator for testing
type mockValidator struct {
	name           string
	validateFunc   func(ctx context.Context, input interface{}, options ValidationOptions) (*ValidationResult, error)
	shouldError    bool
	errorResult    error
}

func (m *mockValidator) Validate(ctx context.Context, input interface{}, options ValidationOptions) (*ValidationResult, error) {
	if m.shouldError {
		return nil, m.errorResult
	}
	if m.validateFunc != nil {
		return m.validateFunc(ctx, input, options)
	}
	// Default successful validation
	result := &ValidationResult{
		IsValid:  true,
		Score:    100,
		Errors:   []ValidationError{},
		Warnings: []ValidationWarning{},
		Context:  make(map[string]interface{}),
	}
	return result, nil
}

func (m *mockValidator) GetName() string {
	return m.name
}

func TestNewBaseValidator(t *testing.T) {
	validator := NewBaseValidator("test_validator", "1.0.0")
	
	assert.NotNil(t, validator)
	assert.Equal(t, "test_validator", validator.Name)
	assert.Equal(t, "1.0.0", validator.Version)
	assert.Equal(t, "test_validator", validator.GetName())
}

func TestBaseValidatorImpl_CreateResult(t *testing.T) {
	validator := NewBaseValidator("test_validator", "1.0.0")
	result := validator.CreateResult()
	
	assert.NotNil(t, result)
	assert.True(t, result.IsValid)
	assert.Equal(t, 100, result.Score)
	assert.Empty(t, result.Errors)
	assert.Empty(t, result.Warnings)
	assert.NotNil(t, result.Context)
	assert.Equal(t, "test_validator", result.Metadata.ValidatorName)
	assert.Equal(t, "1.0.0", result.Metadata.ValidatorVersion)
	assert.False(t, result.Metadata.Timestamp.IsZero())
	assert.NotNil(t, result.Metadata.Parameters)
}

func TestValidationResult_AddError(t *testing.T) {
	validator := NewBaseValidator("test", "1.0")
	result := validator.CreateResult()
	
	// Initially valid
	assert.True(t, result.IsValid)
	assert.Equal(t, 0, result.TotalIssues)
	assert.Equal(t, 0, result.CriticalIssues)
	
	// Add critical error
	criticalError := ValidationError{
		Code:     "CRITICAL_ERROR",
		Message:  "Critical validation error",
		Severity: "critical",
	}
	result.AddError(criticalError)
	
	assert.False(t, result.IsValid)
	assert.Equal(t, 1, result.TotalIssues)
	assert.Equal(t, 1, result.CriticalIssues)
	assert.Len(t, result.Errors, 1)
	assert.Equal(t, criticalError, result.Errors[0])
	
	// Add high severity error
	highError := ValidationError{
		Code:     "HIGH_ERROR",
		Message:  "High severity error",
		Severity: "high",
	}
	result.AddError(highError)
	
	assert.Equal(t, 2, result.TotalIssues)
	assert.Equal(t, 2, result.CriticalIssues)
	
	// Add medium severity error
	mediumError := ValidationError{
		Code:     "MEDIUM_ERROR",
		Message:  "Medium severity error",
		Severity: "medium",
	}
	result.AddError(mediumError)
	
	assert.Equal(t, 3, result.TotalIssues)
	assert.Equal(t, 2, result.CriticalIssues) // Only critical and high count
}

func TestValidationResult_AddWarning(t *testing.T) {
	validator := NewBaseValidator("test", "1.0")
	result := validator.CreateResult()
	
	warning := ValidationWarning{
		Code:    "STYLE_WARNING",
		Message: "Style recommendation",
		Impact:  "maintainability",
	}
	result.AddWarning(warning)
	
	assert.Equal(t, 1, result.TotalIssues)
	assert.Equal(t, 0, result.CriticalIssues) // Warnings don't count as critical
	assert.Len(t, result.Warnings, 1)
	assert.Equal(t, warning, result.Warnings[0])
}

func TestValidationResult_CalculateScore(t *testing.T) {
	tests := []struct {
		name           string
		errors         []ValidationError
		warnings       []ValidationWarning
		expectedScore  int
	}{
		{
			name:          "no issues",
			errors:        []ValidationError{},
			warnings:      []ValidationWarning{},
			expectedScore: 100,
		},
		{
			name: "critical error",
			errors: []ValidationError{
				{Severity: "critical"},
			},
			warnings:      []ValidationWarning{},
			expectedScore: 80, // 100 - 20
		},
		{
			name: "high error",
			errors: []ValidationError{
				{Severity: "high"},
			},
			warnings:      []ValidationWarning{},
			expectedScore: 85, // 100 - 15
		},
		{
			name: "medium error",
			errors: []ValidationError{
				{Severity: "medium"},
			},
			warnings:      []ValidationWarning{},
			expectedScore: 90, // 100 - 10
		},
		{
			name: "low error",
			errors: []ValidationError{
				{Severity: "low"},
			},
			warnings:      []ValidationWarning{},
			expectedScore: 95, // 100 - 5
		},
		{
			name: "unknown severity",
			errors: []ValidationError{
				{Severity: "unknown"},
			},
			warnings:      []ValidationWarning{},
			expectedScore: 100, // No deduction for unknown severity
		},
		{
			name:   "warnings only",
			errors: []ValidationError{},
			warnings: []ValidationWarning{
				{Code: "W1"},
				{Code: "W2"},
				{Code: "W3"},
			},
			expectedScore: 94, // 100 - (3 * 2)
		},
		{
			name: "mixed issues",
			errors: []ValidationError{
				{Severity: "critical"}, // -20
				{Severity: "medium"},   // -10
			},
			warnings: []ValidationWarning{
				{Code: "W1"}, // -2
				{Code: "W2"}, // -2
			},
			expectedScore: 66, // 100 - 20 - 10 - 2 - 2
		},
		{
			name: "score below zero",
			errors: []ValidationError{
				{Severity: "critical"}, // -20
				{Severity: "critical"}, // -20
				{Severity: "critical"}, // -20
				{Severity: "critical"}, // -20
				{Severity: "critical"}, // -20
				{Severity: "critical"}, // -20 (total -120)
			},
			warnings:      []ValidationWarning{},
			expectedScore: 0, // Clamped to 0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &ValidationResult{
				Errors:   tt.errors,
				Warnings: tt.warnings,
			}
			
			result.CalculateScore()
			assert.Equal(t, tt.expectedScore, result.Score)
		})
	}
}

func TestValidationResult_Merge(t *testing.T) {
	result1 := &ValidationResult{
		IsValid: true,
		Errors: []ValidationError{
			{Code: "E1", Severity: "critical"},
		},
		Warnings: []ValidationWarning{
			{Code: "W1"},
		},
		TotalIssues:    2,
		CriticalIssues: 1,
		Context: map[string]interface{}{
			"key1": "value1",
		},
	}
	
	result2 := &ValidationResult{
		IsValid: false,
		Errors: []ValidationError{
			{Code: "E2", Severity: "high"},
		},
		Warnings: []ValidationWarning{
			{Code: "W2"},
			{Code: "W3"},
		},
		TotalIssues:    3,
		CriticalIssues: 1,
		Context: map[string]interface{}{
			"key2": "value2",
		},
	}
	
	result1.Merge(result2)
	
	assert.False(t, result1.IsValid) // Should be false if any merged result is false
	assert.Len(t, result1.Errors, 2)
	assert.Len(t, result1.Warnings, 3)
	assert.Equal(t, 5, result1.TotalIssues)
	assert.Equal(t, 2, result1.CriticalIssues)
	assert.Equal(t, "value1", result1.Context["key1"])
	assert.Equal(t, "value2", result1.Context["key2"])
}

func TestValidationResult_MergeNil(t *testing.T) {
	result := &ValidationResult{
		IsValid:     true,
		TotalIssues: 1,
	}
	
	result.Merge(nil)
	
	// Should be unchanged
	assert.True(t, result.IsValid)
	assert.Equal(t, 1, result.TotalIssues)
}

func TestValidationResult_FilterBySeverity(t *testing.T) {
	result := &ValidationResult{
		Errors: []ValidationError{
			{Code: "E1", Severity: "critical"},
			{Code: "E2", Severity: "high"},
			{Code: "E3", Severity: "medium"},
			{Code: "E4", Severity: "low"},
		},
		Warnings: []ValidationWarning{
			{Code: "W1"},
			{Code: "W2"},
		},
		TotalIssues:    6,
		CriticalIssues: 2,
	}
	
	// Filter to high and above
	result.FilterBySeverity("high")
	
	assert.Len(t, result.Errors, 2) // critical and high only
	assert.Len(t, result.Warnings, 2) // warnings unchanged
	assert.Equal(t, 4, result.TotalIssues) // 2 errors + 2 warnings
	assert.Equal(t, 2, result.CriticalIssues) // critical and high
	
	// Check that correct errors remain
	assert.Equal(t, "E1", result.Errors[0].Code)
	assert.Equal(t, "E2", result.Errors[1].Code)
}

func TestGetSeverityLevel(t *testing.T) {
	tests := []struct {
		severity string
		expected int
	}{
		{"critical", 4},
		{"high", 3},
		{"medium", 2},
		{"low", 1},
		{"unknown", 0},
		{"", 0},
	}
	
	for _, tt := range tests {
		t.Run(tt.severity, func(t *testing.T) {
			result := GetSeverityLevel(tt.severity)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewValidationContext(t *testing.T) {
	options := ValidationOptions{
		Severity:    "high",
		StrictMode:  true,
		IgnoreRules: []string{"rule1", "rule2"},
	}
	
	ctx := NewValidationContext("session123", "/tmp/work", options)
	
	assert.NotNil(t, ctx)
	assert.Equal(t, "session123", ctx.SessionID)
	assert.Equal(t, "/tmp/work", ctx.WorkingDir)
	assert.Equal(t, options, ctx.Options)
	assert.False(t, ctx.StartTime.IsZero())
	assert.NotNil(t, ctx.Custom)
}

func TestValidationContext_Duration(t *testing.T) {
	ctx := NewValidationContext("test", "/tmp", ValidationOptions{})
	
	// Wait a bit and check duration
	time.Sleep(10 * time.Millisecond)
	duration := ctx.Duration()
	
	assert.True(t, duration >= 10*time.Millisecond)
	assert.True(t, duration < 100*time.Millisecond) // Should be reasonable
}

func TestNewValidatorChain(t *testing.T) {
	validator1 := &mockValidator{name: "validator1"}
	validator2 := &mockValidator{name: "validator2"}
	
	chain := NewValidatorChain(validator1, validator2)
	
	assert.NotNil(t, chain)
	assert.Len(t, chain.validators, 2)
	assert.Equal(t, validator1, chain.validators[0])
	assert.Equal(t, validator2, chain.validators[1])
	assert.Equal(t, "ValidatorChain", chain.GetName())
}

func TestValidatorChain_ValidateSuccess(t *testing.T) {
	validator1 := &mockValidator{
		name: "validator1",
		validateFunc: func(ctx context.Context, input interface{}, options ValidationOptions) (*ValidationResult, error) {
			result := &ValidationResult{
				IsValid: true,
				Errors: []ValidationError{
					{Code: "E1", Severity: "medium"},
				},
				Warnings: []ValidationWarning{
					{Code: "W1"},
				},
				Context: map[string]interface{}{
					"validator1": "result",
				},
			}
			return result, nil
		},
	}
	
	validator2 := &mockValidator{
		name: "validator2",
		validateFunc: func(ctx context.Context, input interface{}, options ValidationOptions) (*ValidationResult, error) {
			result := &ValidationResult{
				IsValid: false,
				Errors: []ValidationError{
					{Code: "E2", Severity: "high"},
				},
				Warnings: []ValidationWarning{},
				Context: map[string]interface{}{
					"validator2": "result",
				},
			}
			return result, nil
		},
	}
	
	chain := NewValidatorChain(validator1, validator2)
	
	result, err := chain.Validate(context.Background(), "test input", ValidationOptions{})
	
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsValid) // Should be false if any validator returns false
	assert.Len(t, result.Errors, 2)
	assert.Len(t, result.Warnings, 1)
	assert.Equal(t, "result", result.Context["validator1"])
	assert.Equal(t, "result", result.Context["validator2"])
	
	// Score should be calculated
	assert.True(t, result.Score >= 0 && result.Score <= 100)
}

func TestValidatorChain_ValidateError(t *testing.T) {
	validator1 := &mockValidator{name: "validator1"} // Success
	validator2 := &mockValidator{
		name:        "validator2",
		shouldError: true,
		errorResult: errors.New("validation failed"),
	}
	
	chain := NewValidatorChain(validator1, validator2)
	
	result, err := chain.Validate(context.Background(), "test input", ValidationOptions{})
	
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "validator validator2 failed")
	assert.Contains(t, err.Error(), "validation failed")
}

func TestValidatorChain_EmptyChain(t *testing.T) {
	chain := NewValidatorChain()
	
	result, err := chain.Validate(context.Background(), "test input", ValidationOptions{})
	
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.IsValid)
	assert.Empty(t, result.Errors)
	assert.Empty(t, result.Warnings)
	assert.Equal(t, 100, result.Score) // Should calculate score even for empty chain
}

func TestValidationOptions_Structure(t *testing.T) {
	options := ValidationOptions{
		Severity:     "high",
		IgnoreRules:  []string{"rule1", "rule2"},
		StrictMode:   true,
		CustomParams: map[string]interface{}{
			"param1": "value1",
			"param2": 42,
		},
	}
	
	assert.Equal(t, "high", options.Severity)
	assert.Equal(t, []string{"rule1", "rule2"}, options.IgnoreRules)
	assert.True(t, options.StrictMode)
	assert.Equal(t, "value1", options.CustomParams["param1"])
	assert.Equal(t, 42, options.CustomParams["param2"])
}

func TestValidationError_Structure(t *testing.T) {
	location := ErrorLocation{
		File:   "test.go",
		Line:   42,
		Column: 10,
		Path:   "$.field.subfield",
	}
	
	err := ValidationError{
		Code:          "INVALID_FORMAT",
		Type:          "format",
		Message:       "Invalid format detected",
		Severity:      "high",
		Location:      location,
		Fix:           "Use correct format",
		Documentation: "https://docs.example.com/format",
	}
	
	assert.Equal(t, "INVALID_FORMAT", err.Code)
	assert.Equal(t, "format", err.Type)
	assert.Equal(t, "Invalid format detected", err.Message)
	assert.Equal(t, "high", err.Severity)
	assert.Equal(t, location, err.Location)
	assert.Equal(t, "Use correct format", err.Fix)
	assert.Equal(t, "https://docs.example.com/format", err.Documentation)
}

func TestValidationWarning_Structure(t *testing.T) {
	location := WarningLocation{
		File: "test.go",
		Line: 24,
		Path: "$.field",
	}
	
	warning := ValidationWarning{
		Code:       "PERFORMANCE_HINT",
		Type:       "performance",
		Message:    "Consider optimizing this operation",
		Suggestion: "Use a more efficient algorithm",
		Impact:     "performance",
		Location:   location,
	}
	
	assert.Equal(t, "PERFORMANCE_HINT", warning.Code)
	assert.Equal(t, "performance", warning.Type)
	assert.Equal(t, "Consider optimizing this operation", warning.Message)
	assert.Equal(t, "Use a more efficient algorithm", warning.Suggestion)
	assert.Equal(t, "performance", warning.Impact)
	assert.Equal(t, location, warning.Location)
}

func TestValidationMetadata_Structure(t *testing.T) {
	duration := time.Minute
	timestamp := time.Now()
	params := map[string]interface{}{
		"strict": true,
		"level":  "high",
	}
	
	metadata := ValidationMetadata{
		ValidatorName:    "test_validator",
		ValidatorVersion: "2.1.0",
		Duration:         duration,
		Timestamp:        timestamp,
		Parameters:       params,
	}
	
	assert.Equal(t, "test_validator", metadata.ValidatorName)
	assert.Equal(t, "2.1.0", metadata.ValidatorVersion)
	assert.Equal(t, duration, metadata.Duration)
	assert.Equal(t, timestamp, metadata.Timestamp)
	assert.Equal(t, params, metadata.Parameters)
}

func TestValidationResult_ComplexScenario(t *testing.T) {
	// Test a complex validation scenario with multiple operations
	validator := NewBaseValidator("complex_validator", "1.0.0")
	result := validator.CreateResult()
	
	// Add various types of errors and warnings
	result.AddError(ValidationError{Code: "E1", Severity: "critical"})
	result.AddError(ValidationError{Code: "E2", Severity: "high"})
	result.AddError(ValidationError{Code: "E3", Severity: "medium"})
	result.AddError(ValidationError{Code: "E4", Severity: "low"})
	
	result.AddWarning(ValidationWarning{Code: "W1"})
	result.AddWarning(ValidationWarning{Code: "W2"})
	
	// Calculate score
	result.CalculateScore()
	
	// Expected: 100 - 20 (critical) - 15 (high) - 10 (medium) - 5 (low) - 4 (2 warnings * 2) = 46
	assert.Equal(t, 46, result.Score)
	assert.False(t, result.IsValid)
	assert.Equal(t, 6, result.TotalIssues)
	assert.Equal(t, 2, result.CriticalIssues) // critical + high
	
	// Test merging with another result
	other := &ValidationResult{
		IsValid: true,
		Errors: []ValidationError{
			{Code: "E5", Severity: "medium"},
		},
		Warnings: []ValidationWarning{
			{Code: "W3"},
		},
		TotalIssues:    2,
		CriticalIssues: 0,
		Context: map[string]interface{}{
			"merged": true,
		},
	}
	
	result.Merge(other)
	
	assert.Len(t, result.Errors, 5)
	assert.Len(t, result.Warnings, 3)
	assert.Equal(t, 8, result.TotalIssues)
	assert.Equal(t, 2, result.CriticalIssues)
	assert.True(t, result.Context["merged"].(bool))
	
	// Test filtering
	result.FilterBySeverity("high")
	assert.Len(t, result.Errors, 2) // Only critical and high remain
	assert.Len(t, result.Warnings, 3) // Warnings unchanged
}