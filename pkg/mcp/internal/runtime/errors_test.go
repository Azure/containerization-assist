package runtime

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorType_Constants(t *testing.T) {
	assert.Equal(t, ErrorType("validation"), ErrTypeValidation)
	assert.Equal(t, ErrorType("not_found"), ErrTypeNotFound)
	assert.Equal(t, ErrorType("system"), ErrTypeSystem)
	assert.Equal(t, ErrorType("build"), ErrTypeBuild)
	assert.Equal(t, ErrorType("deployment"), ErrTypeDeployment)
	assert.Equal(t, ErrorType("security"), ErrTypeSecurity)
	assert.Equal(t, ErrorType("configuration"), ErrTypeConfig)
	assert.Equal(t, ErrorType("network"), ErrTypeNetwork)
	assert.Equal(t, ErrorType("permission"), ErrTypePermission)
}

func TestErrorSeverity_Constants(t *testing.T) {
	assert.Equal(t, ErrorSeverity("critical"), SeverityCritical)
	assert.Equal(t, ErrorSeverity("high"), SeverityHigh)
	assert.Equal(t, ErrorSeverity("medium"), SeverityMedium)
	assert.Equal(t, ErrorSeverity("low"), SeverityLow)
}

func TestToolError_Error(t *testing.T) {
	tests := []struct {
		name      string
		toolError *ToolError
		expected  string
	}{
		{
			name: "error without cause",
			toolError: &ToolError{
				Code:    "TEST_ERROR",
				Message: "test error message",
			},
			expected: "TEST_ERROR: test error message",
		},
		{
			name: "error with cause",
			toolError: &ToolError{
				Code:    "TEST_ERROR",
				Message: "test error message",
				Cause:   errors.New("underlying error"),
			},
			expected: "TEST_ERROR: test error message (caused by: underlying error)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.toolError.Error()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestToolError_Unwrap(t *testing.T) {
	underlyingErr := errors.New("underlying error")
	toolErr := &ToolError{
		Code:    "TEST_ERROR",
		Message: "test message",
		Cause:   underlyingErr,
	}

	unwrapped := toolErr.Unwrap()
	assert.Equal(t, underlyingErr, unwrapped)

	// Test with no cause
	toolErrNoCause := &ToolError{
		Code:    "TEST_ERROR",
		Message: "test message",
	}
	assert.Nil(t, toolErrNoCause.Unwrap())
}

func TestToolError_WithContext(t *testing.T) {
	toolErr := &ToolError{
		Code:    "TEST_ERROR",
		Message: "test message",
		Context: ErrorContext{},
	}

	// Add context to empty error
	result := toolErr.WithContext("key1", "value1")
	assert.Equal(t, toolErr, result) // Should return same instance
	assert.Equal(t, "value1", toolErr.Context.Fields["key1"])

	// Add more context
	toolErr.WithContext("key2", 42)
	assert.Equal(t, "value1", toolErr.Context.Fields["key1"])
	assert.Equal(t, 42, toolErr.Context.Fields["key2"])
}

func TestNewErrorBuilder(t *testing.T) {
	builder := NewErrorBuilder("TEST_CODE", "test message")

	assert.NotNil(t, builder)
	assert.NotNil(t, builder.err)
	assert.Equal(t, "TEST_CODE", builder.err.Code)
	assert.Equal(t, "test message", builder.err.Message)
	assert.Equal(t, ErrTypeSystem, builder.err.Type)
	assert.Equal(t, SeverityMedium, builder.err.Severity)
	assert.NotNil(t, builder.err.Context.Fields)
}

func TestErrorBuilder_FluentInterface(t *testing.T) {
	underlyingErr := errors.New("underlying error")

	err := NewErrorBuilder("TEST_CODE", "test message").
		WithType(ErrTypeValidation).
		WithSeverity(SeverityCritical).
		WithCause(underlyingErr).
		WithTool("test_tool").
		WithOperation("test_operation").
		WithStage("test_stage").
		WithSessionID("session123").
		WithField("custom_field", "custom_value").
		Build()

	assert.Equal(t, "TEST_CODE", err.Code)
	assert.Equal(t, "test message", err.Message)
	assert.Equal(t, ErrTypeValidation, err.Type)
	assert.Equal(t, SeverityCritical, err.Severity)
	assert.Equal(t, underlyingErr, err.Cause)
	assert.Equal(t, "test_tool", err.Context.Tool)
	assert.Equal(t, "test_operation", err.Context.Operation)
	assert.Equal(t, "test_stage", err.Context.Stage)
	assert.Equal(t, "session123", err.Context.SessionID)
	assert.Equal(t, "custom_value", err.Context.Fields["custom_field"])
}

func TestNewValidationError(t *testing.T) {
	err := NewValidationError("username", "username is required")

	assert.Equal(t, "VALIDATION_ERROR", err.Code)
	assert.Equal(t, "username is required", err.Message)
	assert.Equal(t, ErrTypeValidation, err.Type)
	assert.Equal(t, "username", err.Context.Fields["field"])
}

func TestNewNotFoundError(t *testing.T) {
	err := NewNotFoundError("user", "123")

	assert.Equal(t, "NOT_FOUND", err.Code)
	assert.Equal(t, "user not found: 123", err.Message)
	assert.Equal(t, ErrTypeNotFound, err.Type)
	assert.Equal(t, "user", err.Context.Fields["resource"])
	assert.Equal(t, "123", err.Context.Fields["identifier"])
}

func TestNewSystemError(t *testing.T) {
	cause := errors.New("disk full")
	err := NewSystemError("backup", cause)

	assert.Equal(t, "SYSTEM_ERROR", err.Code)
	assert.Equal(t, "system error during backup", err.Message)
	assert.Equal(t, ErrTypeSystem, err.Type)
	assert.Equal(t, cause, err.Cause)
	assert.Equal(t, "backup", err.Context.Operation)
}

func TestNewBuildError(t *testing.T) {
	err := NewBuildError("compile", "compilation failed")

	assert.Equal(t, "BUILD_ERROR", err.Code)
	assert.Equal(t, "compilation failed", err.Message)
	assert.Equal(t, ErrTypeBuild, err.Type)
	assert.Equal(t, "compile", err.Context.Stage)
}

func TestNewValidationErrorSet(t *testing.T) {
	errorSet := NewValidationErrorSet()

	assert.NotNil(t, errorSet)
	assert.False(t, errorSet.HasErrors())
	assert.Equal(t, 0, errorSet.Count())
	assert.Empty(t, errorSet.Errors())
	assert.Equal(t, "", errorSet.Error())
}

func TestValidationErrorSet_AddAndCount(t *testing.T) {
	errorSet := NewValidationErrorSet()

	// Add custom error
	err1 := NewValidationError("field1", "error1")
	errorSet.Add(err1)

	assert.True(t, errorSet.HasErrors())
	assert.Equal(t, 1, errorSet.Count())
	assert.Len(t, errorSet.Errors(), 1)
	assert.Equal(t, err1, errorSet.Errors()[0])

	// Add field error
	errorSet.AddField("field2", "error2")

	assert.Equal(t, 2, errorSet.Count())
	assert.Len(t, errorSet.Errors(), 2)
	assert.Equal(t, "field2", errorSet.Errors()[1].Context.Fields["field"])
}

func TestValidationErrorSet_Error(t *testing.T) {
	errorSet := NewValidationErrorSet()

	// Empty error set
	assert.Equal(t, "", errorSet.Error())

	// Single error
	errorSet.AddField("field1", "error1")
	errorMsg := errorSet.Error()
	assert.Contains(t, errorMsg, "validation failed with 1 errors")
	assert.Contains(t, errorMsg, "VALIDATION_ERROR: error1")

	// Multiple errors
	errorSet.AddField("field2", "error2")
	errorMsg = errorSet.Error()
	assert.Contains(t, errorMsg, "validation failed with 2 errors")
	assert.Contains(t, errorMsg, "VALIDATION_ERROR: error1")
	assert.Contains(t, errorMsg, "VALIDATION_ERROR: error2")
}

func TestNewErrorHandler(t *testing.T) {
	logger := "mock_logger"
	handler := NewErrorHandler(logger)

	assert.NotNil(t, handler)
	assert.Equal(t, logger, handler.logger)
}

func TestErrorHandler_Handle(t *testing.T) {
	handler := NewErrorHandler(nil)

	// Test nil error
	result := handler.Handle(nil)
	assert.Nil(t, result)

	// Test ToolError
	toolErr := &ToolError{
		Code:     "TEST_ERROR",
		Message:  "test message",
		Severity: SeverityHigh,
	}
	result = handler.Handle(toolErr)
	assert.Equal(t, toolErr, result)

	// Test unknown error
	unknownErr := errors.New("unknown error")
	result = handler.Handle(unknownErr)

	toolResult, ok := result.(*ToolError)
	require.True(t, ok)
	assert.Equal(t, "SYSTEM_ERROR", toolResult.Code)
	assert.Equal(t, unknownErr, toolResult.Cause)
}

func TestErrorHandler_IsRetryable(t *testing.T) {
	handler := NewErrorHandler(nil)

	tests := []struct {
		name      string
		error     error
		retryable bool
	}{
		{
			name:      "nil error",
			error:     nil,
			retryable: false,
		},
		{
			name: "network error",
			error: &ToolError{
				Type: ErrTypeNetwork,
			},
			retryable: true,
		},
		{
			name: "system error",
			error: &ToolError{
				Type: ErrTypeSystem,
			},
			retryable: true,
		},
		{
			name: "validation error",
			error: &ToolError{
				Type: ErrTypeValidation,
			},
			retryable: false,
		},
		{
			name: "permission error",
			error: &ToolError{
				Type: ErrTypePermission,
			},
			retryable: false,
		},
		{
			name: "retryable code - timeout",
			error: &ToolError{
				Code: "TIMEOUT",
				Type: ErrTypeBuild, // Different type, but retryable code
			},
			retryable: true,
		},
		{
			name: "retryable code - connection refused",
			error: &ToolError{
				Code: "CONNECTION_REFUSED",
				Type: ErrTypeBuild,
			},
			retryable: true,
		},
		{
			name: "retryable code - resource busy",
			error: &ToolError{
				Code: "RESOURCE_BUSY",
				Type: ErrTypeBuild,
			},
			retryable: true,
		},
		{
			name: "retryable code - rate limited",
			error: &ToolError{
				Code: "RATE_LIMITED",
				Type: ErrTypeBuild,
			},
			retryable: true,
		},
		{
			name: "non-retryable code",
			error: &ToolError{
				Code: "INVALID_INPUT",
				Type: ErrTypeBuild,
			},
			retryable: false,
		},
		{
			name:      "standard error with timeout",
			error:     errors.New("connection timeout occurred"),
			retryable: true,
		},
		{
			name:      "standard error with connection refused",
			error:     errors.New("Connection refused by server"),
			retryable: true,
		},
		{
			name:      "standard error with temporary failure",
			error:     errors.New("Temporary failure in name resolution"),
			retryable: true,
		},
		{
			name:      "standard error with resource unavailable",
			error:     errors.New("Resource temporarily unavailable"),
			retryable: true,
		},
		{
			name:      "standard error with deadlock",
			error:     errors.New("Database deadlock detected"),
			retryable: true,
		},
		{
			name:      "standard non-retryable error",
			error:     errors.New("invalid syntax"),
			retryable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.IsRetryable(tt.error)
			assert.Equal(t, tt.retryable, result)
		})
	}
}

func TestErrorHandler_isRetryableCode(t *testing.T) {
	handler := NewErrorHandler(nil)

	retryableCodes := []string{
		"TIMEOUT",
		"CONNECTION_REFUSED",
		"RESOURCE_BUSY",
		"RATE_LIMITED",
	}

	nonRetryableCodes := []string{
		"INVALID_INPUT",
		"NOT_FOUND",
		"VALIDATION_ERROR",
		"PERMISSION_DENIED",
	}

	for _, code := range retryableCodes {
		assert.True(t, handler.isRetryableCode(code), "Expected %s to be retryable", code)
	}

	for _, code := range nonRetryableCodes {
		assert.False(t, handler.isRetryableCode(code), "Expected %s to not be retryable", code)
	}
}

func TestErrorContext_Structure(t *testing.T) {
	context := ErrorContext{
		Tool:      "test_tool",
		Operation: "test_operation",
		Stage:     "test_stage",
		SessionID: "session123",
		Fields: map[string]interface{}{
			"custom": "value",
		},
	}

	assert.Equal(t, "test_tool", context.Tool)
	assert.Equal(t, "test_operation", context.Operation)
	assert.Equal(t, "test_stage", context.Stage)
	assert.Equal(t, "session123", context.SessionID)
	assert.Equal(t, "value", context.Fields["custom"])
}

func TestToolError_ComplexScenario(t *testing.T) {
	// Test a complex error scenario with chaining
	underlyingErr := errors.New("connection timeout")

	toolErr := NewErrorBuilder("NETWORK_TIMEOUT", "failed to connect to database").
		WithType(ErrTypeNetwork).
		WithSeverity(SeverityHigh).
		WithCause(underlyingErr).
		WithTool("database_connector").
		WithOperation("connect").
		WithStage("initialization").
		WithSessionID("sess_123").
		WithField("host", "db.example.com").
		WithField("port", 5432).
		Build()

	// Test error message
	errorMsg := toolErr.Error()
	assert.Contains(t, errorMsg, "NETWORK_TIMEOUT")
	assert.Contains(t, errorMsg, "failed to connect to database")
	assert.Contains(t, errorMsg, "connection timeout")

	// Test unwrapping
	assert.Equal(t, underlyingErr, toolErr.Unwrap())

	// Test context
	assert.Equal(t, "database_connector", toolErr.Context.Tool)
	assert.Equal(t, "connect", toolErr.Context.Operation)
	assert.Equal(t, "initialization", toolErr.Context.Stage)
	assert.Equal(t, "sess_123", toolErr.Context.SessionID)
	assert.Equal(t, "db.example.com", toolErr.Context.Fields["host"])
	assert.Equal(t, 5432, toolErr.Context.Fields["port"])

	// Test error handler
	handler := NewErrorHandler(nil)
	handled := handler.Handle(toolErr)
	assert.Equal(t, toolErr, handled)
	assert.True(t, handler.IsRetryable(toolErr))
}

func TestValidationErrorSet_LargeSet(t *testing.T) {
	errorSet := NewValidationErrorSet()

	// Add many errors
	for i := 0; i < 100; i++ {
		errorSet.AddField(fmt.Sprintf("field_%d", i), fmt.Sprintf("error_%d", i))
	}

	assert.Equal(t, 100, errorSet.Count())
	assert.True(t, errorSet.HasErrors())

	errorMsg := errorSet.Error()
	assert.Contains(t, errorMsg, "validation failed with 100 errors")

	// Check that all errors are included in the message
	for i := 0; i < 10; i++ { // Check first 10
		assert.Contains(t, errorMsg, fmt.Sprintf("error_%d", i))
	}
}
