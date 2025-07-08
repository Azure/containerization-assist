package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/common/validation-core/core"
	"github.com/Azure/container-kit/pkg/mcp/errors"
	"github.com/stretchr/testify/assert"
)

type mockValidator struct {
	name         string
	validateFunc func(ctx context.Context, input interface{}, options ValidationOptions) (*RuntimeValidationResult, error)
	shouldError  bool
	errorResult  error
}

func (m *mockValidator) Validate(ctx context.Context, input interface{}, options ValidationOptions) (*RuntimeValidationResult, error) {
	if m.shouldError {
		return nil, m.errorResult
	}
	if m.validateFunc != nil {
		return m.validateFunc(ctx, input, options)
	}

	result := &RuntimeValidationResult{
		Valid:    true,
		Score:    100,
		Errors:   []*core.Error{},
		Warnings: []*core.Warning{},
		Data:     core.RuntimeValidationData{},
		Metadata: core.ValidationMetadata{
			ValidatedAt:      time.Now(),
			ValidatorName:    m.name,
			ValidatorVersion: "1.0.0",
		},
		Timestamp: time.Now(),
	}
	return result, nil
}

func (m *mockValidator) GetName() string {
	return m.name
}

func (m *mockValidator) ValidateUnified(ctx context.Context, input interface{}, options *core.ValidationOptions) (*core.NonGenericResult, error) {

	legacyResult, err := m.Validate(ctx, input, *options)
	if err != nil {
		return nil, err
	}
	return &core.NonGenericResult{
		Valid:       legacyResult.Valid,
		Errors:      legacyResult.Errors,
		Warnings:    legacyResult.Warnings,
		Data:        legacyResult.Data,
		Score:       legacyResult.Score,
		Metadata:    legacyResult.Metadata,
		Timestamp:   legacyResult.Timestamp,
		Suggestions: legacyResult.Suggestions,
	}, nil
}

func TestNewBaseValidator(t *testing.T) {
	t.Parallel()
	validator := NewBaseValidator("test_validator", "1.0.0")

	assert.NotNil(t, validator)
	assert.Equal(t, "test_validator", validator.Name)
	assert.Equal(t, "1.0.0", validator.Version)
	assert.Equal(t, "test_validator", validator.GetName())
}

func TestBaseValidatorImpl_CreateResult(t *testing.T) {
	t.Parallel()
	validator := NewBaseValidator("test_validator", "1.0.0")
	result := validator.CreateResult()

	assert.NotNil(t, result)
	assert.True(t, result.Valid)
	assert.Equal(t, float64(100), result.Score)
	assert.Empty(t, result.Errors)
	assert.Empty(t, result.Warnings)
	assert.NotNil(t, result.Data)
	assert.Equal(t, "test_validator", result.Metadata.ValidatorName)
	assert.Equal(t, "1.0.0", result.Metadata.ValidatorVersion)
	assert.False(t, result.Metadata.ValidatedAt.IsZero())
}

func TestValidationResult_AddError(t *testing.T) {
	t.Parallel()
	validator := NewBaseValidator("test", "1.0")
	result := validator.CreateResult()
	assert.True(t, result.Valid)
	assert.Equal(t, 0, len(result.Errors))
	assert.Equal(t, 0, len(result.Warnings))
	criticalError := core.NewError(
		"CRITICAL_ERROR",
		"Critical validation error",
		core.ErrTypeValidation,
		core.SeverityCritical,
	)
	result.AddError(criticalError)

	assert.False(t, result.Valid)
	assert.Equal(t, 1, len(result.Errors))
	assert.Len(t, result.Errors, 1)
	assert.Equal(t, criticalError, result.Errors[0])
	highError := core.NewError(
		"HIGH_ERROR",
		"High severity error",
		core.ErrTypeValidation,
		core.SeverityHigh,
	)
	result.AddError(highError)

	assert.Equal(t, 2, len(result.Errors))
	mediumError := core.NewError(
		"MEDIUM_ERROR",
		"Medium severity error",
		core.ErrTypeValidation,
		core.SeverityMedium,
	)
	result.AddError(mediumError)

	assert.Equal(t, 3, len(result.Errors))

	assert.Len(t, result.Errors, 3)
}

func TestValidationResult_AddWarning(t *testing.T) {
	t.Parallel()
	validator := NewBaseValidator("test", "1.0")
	result := validator.CreateResult()

	warning := core.NewWarning(
		"STYLE_WARNING",
		"Style recommendation",
	)
	result.AddWarning(warning)

	assert.Equal(t, 1, len(result.Warnings))
	assert.Equal(t, 0, len(result.Errors))
	assert.Len(t, result.Warnings, 1)
	assert.Equal(t, warning, result.Warnings[0])
}

func TestValidationResult_ScoreCalculation(t *testing.T) {
	t.Parallel()

	validator := NewBaseValidator("test", "1.0")

	t.Run("no issues", func(t *testing.T) {
		t.Parallel()
		result := validator.CreateResult()
		assert.Equal(t, float64(100), result.Score)
		assert.True(t, result.Valid)
	})

	t.Run("with errors", func(t *testing.T) {
		t.Parallel()
		result := validator.CreateResult()
		criticalError := core.NewError(
			"CRITICAL", "Critical error", core.ErrTypeValidation, core.SeverityCritical,
		)
		result.AddError(criticalError)

		assert.False(t, result.Valid)
		assert.Len(t, result.Errors, 1)
	})

	t.Run("with warnings", func(t *testing.T) {
		t.Parallel()
		result := validator.CreateResult()

		warning := core.NewWarning("WARNING", "Test warning")
		result.AddWarning(warning)

		assert.True(t, result.Valid)
		assert.Len(t, result.Warnings, 1)
	})
}

func TestValidationResult_ManualMerge(t *testing.T) {
	t.Parallel()

	result1 := &RuntimeValidationResult{
		Valid: true,
		Score: 100,
		Errors: []*core.Error{
			core.NewError("E1", "Error 1", core.ErrTypeValidation, core.SeverityCritical),
		},
		Warnings: []*core.Warning{
			core.NewWarning("W1", "Warning 1"),
		},
		Data: core.RuntimeValidationData{},
		Metadata: core.ValidationMetadata{
			ValidatedAt:      time.Now(),
			ValidatorName:    "test1",
			ValidatorVersion: "1.0.0",
		},
		Timestamp: time.Now(),
	}

	result2 := &RuntimeValidationResult{
		Valid: false,
		Score: 80,
		Errors: []*core.Error{
			core.NewError("E2", "Error 2", core.ErrTypeValidation, core.SeverityHigh),
		},
		Warnings: []*core.Warning{
			core.NewWarning("W2", "Warning 2"),
			core.NewWarning("W3", "Warning 3"),
		},
		Data: core.RuntimeValidationData{},
		Metadata: core.ValidationMetadata{
			ValidatedAt:      time.Now(),
			ValidatorName:    "test2",
			ValidatorVersion: "1.0.0",
		},
		Timestamp: time.Now(),
	}
	result1.Valid = result1.Valid && result2.Valid
	result1.Errors = append(result1.Errors, result2.Errors...)
	result1.Warnings = append(result1.Warnings, result2.Warnings...)
	if result2.Score < result1.Score {
		result1.Score = result2.Score
	}

	assert.False(t, result1.Valid)
	assert.Len(t, result1.Errors, 2)
	assert.Len(t, result1.Warnings, 3)
	assert.Equal(t, float64(80), result1.Score)
}

func TestValidationResult_MergeNil(t *testing.T) {
	t.Parallel()
	validator := NewBaseValidator("test", "1.0")
	result := validator.CreateResult()

	originalValid := result.Valid
	originalErrors := len(result.Errors)
	originalWarnings := len(result.Warnings)

	assert.Equal(t, originalValid, result.Valid)
	assert.Equal(t, originalErrors, len(result.Errors))
	assert.Equal(t, originalWarnings, len(result.Warnings))
}

func TestValidationResult_SeverityFiltering(t *testing.T) {
	t.Parallel()

	validator := NewBaseValidator("test", "1.0")
	result := validator.CreateResult()
	criticalError := core.NewError("E1", "Critical", core.ErrTypeValidation, core.SeverityCritical)
	highError := core.NewError("E2", "High", core.ErrTypeValidation, core.SeverityHigh)
	mediumError := core.NewError("E3", "Medium", core.ErrTypeValidation, core.SeverityMedium)
	lowError := core.NewError("E4", "Low", core.ErrTypeValidation, core.SeverityLow)

	result.AddError(criticalError)
	result.AddError(highError)
	result.AddError(mediumError)
	result.AddError(lowError)
	warning1 := core.NewWarning("W1", "Warning 1")
	warning2 := core.NewWarning("W2", "Warning 2")
	result.AddWarning(warning1)
	result.AddWarning(warning2)

	assert.Len(t, result.Errors, 4)
	assert.Len(t, result.Warnings, 2)
	assert.Equal(t, core.SeverityCritical, result.Errors[0].Severity)
	assert.Equal(t, core.SeverityHigh, result.Errors[1].Severity)
	assert.Equal(t, core.SeverityMedium, result.Errors[2].Severity)
	assert.Equal(t, core.SeverityLow, result.Errors[3].Severity)
}

func TestSeverityConstants(t *testing.T) {
	t.Parallel()

	assert.NotEmpty(t, core.SeverityCritical)
	assert.NotEmpty(t, core.SeverityHigh)
	assert.NotEmpty(t, core.SeverityMedium)
	assert.NotEmpty(t, core.SeverityLow)
	severities := []core.ErrorSeverity{
		core.SeverityCritical,
		core.SeverityHigh,
		core.SeverityMedium,
		core.SeverityLow,
	}

	for i, sev1 := range severities {
		for j, sev2 := range severities {
			if i != j {
				assert.NotEqual(t, sev1, sev2)
			}
		}
	}
}

func TestNewValidationContext(t *testing.T) {
	t.Parallel()
	options := core.NewValidationOptions()
	options.StrictMode = true

	ctx := NewValidationContext("session123", "/tmp/work", *options)

	assert.NotNil(t, ctx)
	assert.Equal(t, "session123", ctx.SessionID)
	assert.Equal(t, "/tmp/work", ctx.WorkingDir)
	assert.Equal(t, *options, ctx.Options)
	assert.False(t, ctx.StartTime.IsZero())
	assert.NotNil(t, ctx.Custom)
}

func TestValidationContext_Duration(t *testing.T) {
	t.Parallel()
	options := core.NewValidationOptions()
	ctx := NewValidationContext("test", "/tmp", *options)
	time.Sleep(10 * time.Millisecond)
	duration := ctx.Duration()

	assert.True(t, duration >= 10*time.Millisecond)
	assert.True(t, duration < 100*time.Millisecond)
}

func TestNewValidatorChain(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	validator1 := &mockValidator{
		name: "validator1",
		validateFunc: func(ctx context.Context, input interface{}, options ValidationOptions) (*RuntimeValidationResult, error) {
			result := &RuntimeValidationResult{
				Valid: true,
				Errors: []*core.Error{
					core.NewError("E1", "Error 1", core.ErrTypeValidation, core.SeverityMedium),
				},
				Warnings: []*core.Warning{
					core.NewWarning("W1", "Warning 1"),
				},
				Data: core.RuntimeValidationData{
					ToolName: "validator1",
				},
				Metadata: core.ValidationMetadata{
					ValidatedAt:      time.Now(),
					ValidatorName:    "validator1",
					ValidatorVersion: "1.0.0",
				},
				Timestamp: time.Now(),
			}
			return result, nil
		},
	}

	validator2 := &mockValidator{
		name: "validator2",
		validateFunc: func(ctx context.Context, input interface{}, options ValidationOptions) (*RuntimeValidationResult, error) {
			result := &RuntimeValidationResult{
				Valid: false,
				Errors: []*core.Error{
					core.NewError("E2", "Error 2", core.ErrTypeValidation, core.SeverityHigh),
				},
				Warnings: []*core.Warning{},
				Data: core.RuntimeValidationData{
					ToolName: "validator2",
				},
				Metadata: core.ValidationMetadata{
					ValidatedAt:      time.Now(),
					ValidatorName:    "validator2",
					ValidatorVersion: "1.0.0",
				},
				Timestamp: time.Now(),
			}
			return result, nil
		},
	}

	chain := NewValidatorChain(validator1, validator2)

	result, err := chain.Validate(context.Background(), "test input", *core.NewValidationOptions())

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.Valid)
	assert.Len(t, result.Errors, 2)
	assert.Len(t, result.Warnings, 1)
	assert.Equal(t, "", result.Data.ToolName)
	assert.NotNil(t, result.Data)
	assert.True(t, result.Score >= 0 && result.Score <= 100)
}

func TestValidatorChain_ValidateError(t *testing.T) {
	t.Parallel()
	validator1 := &mockValidator{name: "validator1"}
	validator2 := &mockValidator{
		name:        "validator2",
		shouldError: true,
		errorResult: errors.Validation("test", "validation failed"),
	}

	chain := NewValidatorChain(validator1, validator2)

	result, err := chain.Validate(context.Background(), "test input", *core.NewValidationOptions())

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "validator validator2 failed")
}

func TestValidatorChain_EmptyChain(t *testing.T) {
	t.Parallel()
	chain := NewValidatorChain()

	result, err := chain.Validate(context.Background(), "test input", *core.NewValidationOptions())

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
	assert.Empty(t, result.Warnings)
	assert.Equal(t, float64(0), result.Score)
}

func TestValidationOptions_Structure(t *testing.T) {
	t.Parallel()
	options := core.NewValidationOptions()
	options.StrictMode = true
	options.Context = map[string]interface{}{
		"param1": "value1",
		"param2": 42,
	}

	assert.True(t, options.StrictMode)
	assert.Equal(t, "value1", options.Context["param1"])
	assert.Equal(t, 42, options.Context["param2"])
	assert.NotNil(t, options.Context)
}

func TestValidationError_Structure(t *testing.T) {
	t.Parallel()

	err := core.NewError(
		"INVALID_FORMAT",
		"Invalid format detected",
		core.ErrTypeValidation,
		core.SeverityHigh,
	).WithField("$.field.subfield").WithLine(42).WithColumn(10)
	err.WithSuggestion("Use correct format")

	assert.Equal(t, "INVALID_FORMAT", err.Code)
	assert.Equal(t, core.ErrTypeValidation, err.Type)
	assert.Equal(t, "Invalid format detected", err.Message)
	assert.Equal(t, core.SeverityHigh, err.Severity)
	assert.Equal(t, "$.field.subfield", err.Field)
	assert.Equal(t, 42, err.Line)
	assert.Equal(t, 10, err.Column)
	assert.Contains(t, err.Suggestions, "Use correct format")
}

func TestValidationWarning_Structure(t *testing.T) {
	t.Parallel()

	warning := core.NewWarning(
		"PERFORMANCE_HINT",
		"Consider optimizing this operation",
	)
	warning.Error.WithField("$.field").WithLine(24)
	warning.Error.WithSuggestion("Use a more efficient algorithm")

	assert.Equal(t, "PERFORMANCE_HINT", warning.Error.Code)
	assert.Equal(t, core.ErrTypeValidation, warning.Error.Type)
	assert.Equal(t, "Consider optimizing this operation", warning.Error.Message)
	assert.Contains(t, warning.Error.Suggestions, "Use a more efficient algorithm")
	assert.Equal(t, "$.field", warning.Error.Field)
	assert.Equal(t, 24, warning.Error.Line)
}

func TestValidationMetadata_Structure(t *testing.T) {
	t.Parallel()

	timestamp := time.Now()
	context := map[string]interface{}{
		"strict": true,
		"level":  "high",
	}

	metadata := core.ValidationMetadata{
		ValidatedAt:      timestamp,
		ValidatorName:    "test_validator",
		ValidatorVersion: "2.1.0",
		Context:          context,
	}

	assert.Equal(t, "test_validator", metadata.ValidatorName)
	assert.Equal(t, "2.1.0", metadata.ValidatorVersion)
	assert.Equal(t, timestamp, metadata.ValidatedAt)
	assert.Equal(t, context, metadata.Context)
}

func TestValidationResult_ComplexScenario(t *testing.T) {
	t.Parallel()

	validator := NewBaseValidator("complex_validator", "1.0.0")
	result := validator.CreateResult()
	result.AddError(core.NewError("E1", "Critical error", core.ErrTypeValidation, core.SeverityCritical))
	result.AddError(core.NewError("E2", "High error", core.ErrTypeValidation, core.SeverityHigh))
	result.AddError(core.NewError("E3", "Medium error", core.ErrTypeValidation, core.SeverityMedium))
	result.AddError(core.NewError("E4", "Low error", core.ErrTypeValidation, core.SeverityLow))

	result.AddWarning(core.NewWarning("W1", "Warning 1"))
	result.AddWarning(core.NewWarning("W2", "Warning 2"))
	assert.False(t, result.Valid)
	assert.Len(t, result.Errors, 4)
	assert.Len(t, result.Warnings, 2)
	other := &RuntimeValidationResult{
		Valid: true,
		Errors: []*core.Error{
			core.NewError("E5", "Another error", core.ErrTypeValidation, core.SeverityMedium),
		},
		Warnings: []*core.Warning{
			core.NewWarning("W3", "Another warning"),
		},
		Data: core.RuntimeValidationData{
			ToolName: "merged-validator",
		},
		Metadata: core.ValidationMetadata{
			ValidatedAt:      time.Now(),
			ValidatorName:    "merged",
			ValidatorVersion: "1.0.0",
		},
		Timestamp: time.Now(),
	}
	result.Valid = result.Valid && other.Valid
	result.Errors = append(result.Errors, other.Errors...)
	result.Warnings = append(result.Warnings, other.Warnings...)

	assert.Len(t, result.Errors, 5)
	assert.Len(t, result.Warnings, 3)
	assert.False(t, result.Valid)
	severityCount := make(map[core.ErrorSeverity]int)
	for _, err := range result.Errors {
		severityCount[err.Severity]++
	}
	assert.Equal(t, 1, severityCount[core.SeverityCritical])
	assert.Equal(t, 1, severityCount[core.SeverityHigh])
	assert.Equal(t, 2, severityCount[core.SeverityMedium])
	assert.Equal(t, 1, severityCount[core.SeverityLow])
}
