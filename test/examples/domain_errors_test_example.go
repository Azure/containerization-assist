package examples

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainerrros "github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// Example test for domain/errors package
// This demonstrates comprehensive testing of the RichError system

func TestRichError_BasicConstruction(t *testing.T) {
	// Test basic error construction
	err := domainerrros.NewError().
		Code(domainerrros.CodeValidationFailed).
		Type(domainerrros.ErrTypeValidation).
		Severity(domainerrros.SeverityMedium).
		Message("validation failed").
		Build()

	assert.Equal(t, domainerrros.CodeValidationFailed, err.Code())
	assert.Equal(t, domainerrros.ErrTypeValidation, err.Type())
	assert.Equal(t, domainerrros.SeverityMedium, err.Severity())
	assert.Equal(t, "validation failed", err.Error())
}

func TestRichError_WithContext(t *testing.T) {
	// Test error with context information
	err := domainerrros.NewError().
		Code(domainerrros.CodeValidationFailed).
		Message("field validation failed").
		Context("field", "username").
		Context("value", "invalid@value").
		Context("rule", "alphanumeric_only").
		Build()

	context := err.Context()
	assert.Equal(t, "username", context["field"])
	assert.Equal(t, "invalid@value", context["value"])
	assert.Equal(t, "alphanumeric_only", context["rule"])
}

func TestRichError_WithSuggestion(t *testing.T) {
	// Test error with helpful suggestions
	err := domainerrros.NewError().
		Code(domainerrros.CodeConfigurationInvalid).
		Message("invalid configuration").
		Suggestion("Check the configuration file format").
		Suggestion("Ensure all required fields are present").
		Build()

	suggestions := err.Suggestions()
	assert.Len(t, suggestions, 2)
	assert.Contains(t, suggestions, "Check the configuration file format")
	assert.Contains(t, suggestions, "Ensure all required fields are present")
}

func TestRichError_WithLocation(t *testing.T) {
	// Test error with location information
	err := domainerrros.NewError().
		Code(domainerrros.CodeInternalError).
		Message("unexpected error").
		WithLocation().
		Build()

	location := err.Location()
	assert.NotNil(t, location)
	assert.Contains(t, location.Function, "TestRichError_WithLocation")
	assert.Contains(t, location.File, "domain_errors_test_example.go")
	assert.Greater(t, location.Line, 0)
}

func TestRichError_ErrorWrapping(t *testing.T) {
	// Test wrapping of standard errors
	originalErr := errors.New("original error")

	wrappedErr := domainerrros.NewError().
		Code(domainerrros.CodeExternalServiceError).
		Message("external service failed").
		Wrap(originalErr).
		Build()

	assert.Equal(t, originalErr, wrappedErr.Unwrap())
	assert.True(t, errors.Is(wrappedErr, originalErr))
	assert.Contains(t, wrappedErr.Error(), "external service failed")
	assert.Contains(t, wrappedErr.Error(), "original error")
}

func TestRichError_ChainedWrapping(t *testing.T) {
	// Test multiple levels of error wrapping
	level1 := errors.New("database connection failed")

	level2 := domainerrros.NewError().
		Code(domainerrros.CodeExternalServiceError).
		Message("storage layer error").
		Wrap(level1).
		Build()

	level3 := domainerrros.NewError().
		Code(domainerrros.CodeOperationFailed).
		Message("user operation failed").
		Context("operation", "create_user").
		Context("user_id", "12345").
		Wrap(level2).
		Build()

	// Test the error chain
	assert.True(t, errors.Is(level3, level1))
	assert.True(t, errors.Is(level3, level2))

	// Test unwrapping
	assert.Equal(t, level2, level3.Unwrap())
	assert.Equal(t, level1, level2.Unwrap())
}

func TestRichError_JSONSerialization(t *testing.T) {
	// Test JSON serialization for API responses
	err := domainerrros.NewError().
		Code(domainerrros.CodeValidationFailed).
		Type(domainerrros.ErrTypeValidation).
		Severity(domainerrros.SeverityHigh).
		Message("validation failed").
		Context("field", "email").
		Context("value", "invalid-email").
		Suggestion("Provide a valid email address").
		Build()

	// Test JSON marshaling
	jsonData, marshalErr := err.MarshalJSON()
	require.NoError(t, marshalErr)
	assert.Contains(t, string(jsonData), "validation failed")
	assert.Contains(t, string(jsonData), "VALIDATION_FAILED")
	assert.Contains(t, string(jsonData), "high")

	// Test JSON unmarshaling
	var unmarshaledErr domainerrros.RichError
	unmarshalErr := unmarshaledErr.UnmarshalJSON(jsonData)
	require.NoError(t, unmarshalErr)

	assert.Equal(t, err.Code(), unmarshaledErr.Code())
	assert.Equal(t, err.Message(), unmarshaledErr.Error())
	assert.Equal(t, err.Severity(), unmarshaledErr.Severity())
}

func TestRichError_ErrorCodes(t *testing.T) {
	// Test all error codes are properly defined
	testCases := []struct {
		code        domainerrros.ErrorCode
		description string
	}{
		{domainerrros.CodeValidationFailed, "validation failed"},
		{domainerrros.CodeConfigurationInvalid, "configuration invalid"},
		{domainerrros.CodeInternalError, "internal error"},
		{domainerrros.CodeExternalServiceError, "external service error"},
		{domainerrros.CodeOperationFailed, "operation failed"},
		{domainerrros.CodeResourceNotFound, "resource not found"},
		{domainerrros.CodePermissionDenied, "permission denied"},
		{domainerrros.CodeTimeoutError, "timeout error"},
	}

	for _, tc := range testCases {
		t.Run(string(tc.code), func(t *testing.T) {
			err := domainerrros.NewError().
				Code(tc.code).
				Message(tc.description).
				Build()

			assert.Equal(t, tc.code, err.Code())
			assert.NotEmpty(t, err.Error())
		})
	}
}

func TestRichError_ErrorTypes(t *testing.T) {
	// Test all error types are properly defined
	testCases := []struct {
		errorType domainerrros.ErrorType
		code      domainerrros.ErrorCode
	}{
		{domainerrros.ErrTypeValidation, domainerrros.CodeValidationFailed},
		{domainerrros.ErrTypeConfiguration, domainerrros.CodeConfigurationInvalid},
		{domainerrros.ErrTypeInternal, domainerrros.CodeInternalError},
		{domainerrros.ErrTypeExternal, domainerrros.CodeExternalServiceError},
		{domainerrros.ErrTypeOperation, domainerrros.CodeOperationFailed},
		{domainerrros.ErrTypeResource, domainerrros.CodeResourceNotFound},
		{domainerrros.ErrTypeSecurity, domainerrros.CodePermissionDenied},
		{domainerrros.ErrTypeTimeout, domainerrros.CodeTimeoutError},
	}

	for _, tc := range testCases {
		t.Run(string(tc.errorType), func(t *testing.T) {
			err := domainerrros.NewError().
				Type(tc.errorType).
				Code(tc.code).
				Message("test error").
				Build()

			assert.Equal(t, tc.errorType, err.Type())
			assert.Equal(t, tc.code, err.Code())
		})
	}
}

func TestRichError_SeverityLevels(t *testing.T) {
	// Test all severity levels
	severities := []domainerrros.ErrorSeverity{
		domainerrros.SeverityLow,
		domainerrros.SeverityMedium,
		domainerrros.SeverityHigh,
		domainerrros.SeverityCritical,
	}

	for _, severity := range severities {
		t.Run(string(severity), func(t *testing.T) {
			err := domainerrros.NewError().
				Severity(severity).
				Code(domainerrros.CodeInternalError).
				Message("test error").
				Build()

			assert.Equal(t, severity, err.Severity())
		})
	}
}

func TestRichError_ErrorFormatting(t *testing.T) {
	// Test error message formatting
	err := domainerrros.NewError().
		Code(domainerrros.CodeValidationFailed).
		Message("validation failed for user %s", "john_doe").
		Context("user_id", "12345").
		Build()

	assert.Contains(t, err.Error(), "validation failed for user john_doe")
	assert.Equal(t, "12345", err.Context()["user_id"])
}

// Benchmark tests
func BenchmarkRichError_Creation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = domainerrros.NewError().
			Code(domainerrros.CodeValidationFailed).
			Type(domainerrros.ErrTypeValidation).
			Severity(domainerrros.SeverityMedium).
			Message("validation failed").
			Context("field", "test").
			Build()
	}
}

func BenchmarkRichError_WithLocation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = domainerrros.NewError().
			Code(domainerrros.CodeInternalError).
			Message("error with location").
			WithLocation().
			Build()
	}
}

// Property-based test example
func TestRichError_Properties(t *testing.T) {
	// Test that RichError maintains its properties correctly

	// Property: Error code should always be preserved
	t.Run("code_preservation", func(t *testing.T) {
		codes := []domainerrros.ErrorCode{
			domainerrros.CodeValidationFailed,
			domainerrros.CodeConfigurationInvalid,
			domainerrros.CodeInternalError,
		}

		for _, code := range codes {
			err := domainerrros.NewError().Code(code).Build()
			assert.Equal(t, code, err.Code())
		}
	})

	// Property: Context should be additive
	t.Run("context_additive", func(t *testing.T) {
		err := domainerrros.NewError().
			Context("key1", "value1").
			Context("key2", "value2").
			Context("key3", "value3").
			Build()

		context := err.Context()
		assert.Len(t, context, 3)
		assert.Equal(t, "value1", context["key1"])
		assert.Equal(t, "value2", context["key2"])
		assert.Equal(t, "value3", context["key3"])
	})

	// Property: Suggestions should be accumulated
	t.Run("suggestions_accumulative", func(t *testing.T) {
		err := domainerrros.NewError().
			Suggestion("suggestion 1").
			Suggestion("suggestion 2").
			Suggestion("suggestion 3").
			Build()

		suggestions := err.Suggestions()
		assert.Len(t, suggestions, 3)
		assert.Contains(t, suggestions, "suggestion 1")
		assert.Contains(t, suggestions, "suggestion 2")
		assert.Contains(t, suggestions, "suggestion 3")
	})
}

// Integration test with context
func TestRichError_WithContext_Integration(t *testing.T) {
	// Test RichError behavior with Go context
	ctx := context.Background()
	ctx = context.WithValue(ctx, "request_id", "req-12345")

	err := domainerrros.NewError().
		Code(domainerrros.CodeValidationFailed).
		Message("validation failed").
		Context("request_id", ctx.Value("request_id")).
		Build()

	assert.Equal(t, "req-12345", err.Context()["request_id"])
}
