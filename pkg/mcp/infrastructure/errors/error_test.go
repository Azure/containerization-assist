// Package errors provides tests for structured error handling
package errors

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewError(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		component string
		message   string
	}{
		{
			name:      "basic error",
			operation: "deploy",
			component: "kubernetes",
			message:   "deployment failed",
		},
		{
			name:      "empty strings",
			operation: "",
			component: "",
			message:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewError(tt.operation, tt.component, tt.message).Build()

			assert.Equal(t, tt.operation, err.Operation)
			assert.Equal(t, tt.component, err.Component)
			assert.Equal(t, tt.message, err.Message)
			assert.NotEmpty(t, err.ID)
			assert.True(t, strings.HasPrefix(err.ID, "err_"))
			assert.Equal(t, CategoryInfrastructure, err.Category)
			assert.Equal(t, SeverityMedium, err.Severity)
			assert.NotNil(t, err.Context)
			assert.False(t, err.Timestamp.IsZero())
		})
	}
}

func TestErrorBuilder(t *testing.T) {
	cause := errors.New("underlying error")

	err := NewError("test_op", "test_component", "test message").
		WithCategory(CategoryWorkflow).
		WithSeverity(SeverityCritical).
		WithCause(cause).
		WithRecoverable(true).
		WithContext("key", "value").
		WithWorkflowID("workflow_123").
		WithSessionID("session_456").
		WithRetryAfter(time.Second * 5).
		WithStacktrace().
		Build()

	assert.Equal(t, "test_op", err.Operation)
	assert.Equal(t, "test_component", err.Component)
	assert.Equal(t, "test message", err.Message)
	assert.Equal(t, CategoryWorkflow, err.Category)
	assert.Equal(t, SeverityCritical, err.Severity)
	assert.Equal(t, cause, err.Cause)
	assert.True(t, err.Recoverable)
	assert.Equal(t, "value", err.Context["key"])
	assert.Equal(t, "workflow_123", err.WorkflowID)
	assert.Equal(t, "session_456", err.SessionID)
	assert.Equal(t, time.Second*5, *err.RetryAfter)
	assert.NotEmpty(t, err.Stacktrace)
}

func TestStructuredErrorInterface(t *testing.T) {
	cause := errors.New("root cause")
	err := NewError("test", "component", "message").
		WithCause(cause).
		Build()

	// Test error interface
	assert.Contains(t, err.Error(), "test failed in component: message")
	assert.Contains(t, err.Error(), "caused by: root cause")

	// Test unwrap interface
	assert.Equal(t, cause, err.Unwrap())

	// Test recoverable interface
	assert.False(t, err.IsRecoverable()) // Default is false

	err.Recoverable = true
	assert.True(t, err.IsRecoverable())
}

func TestConvenienceConstructors(t *testing.T) {
	t.Run("infrastructure error", func(t *testing.T) {
		cause := errors.New("network failure")
		err := NewInfrastructureError("connect", "database", "connection failed", cause)

		assert.Equal(t, CategoryInfrastructure, err.Category)
		assert.Equal(t, SeverityHigh, err.Severity)
		assert.Equal(t, cause, err.Cause)
	})

	t.Run("validation error", func(t *testing.T) {
		err := NewValidationError("email", "invalid email format")

		assert.Equal(t, CategoryValidation, err.Category)
		assert.Equal(t, SeverityMedium, err.Severity)
		assert.Equal(t, "email", err.Context["field"])
	})

	t.Run("security error", func(t *testing.T) {
		err := NewSecurityError("authenticate", "auth", "invalid credentials")

		assert.Equal(t, CategorySecurity, err.Category)
		assert.Equal(t, SeverityCritical, err.Severity)
		assert.False(t, err.Recoverable)
	})

	t.Run("workflow error", func(t *testing.T) {
		cause := errors.New("step failed")
		err := NewWorkflowError("build", "docker build failed", cause)

		assert.Equal(t, CategoryWorkflow, err.Category)
		assert.Equal(t, SeverityHigh, err.Severity)
		assert.Equal(t, "build", err.Context["step"])
		assert.True(t, err.Recoverable)
	})

	t.Run("network error", func(t *testing.T) {
		cause := errors.New("connection timeout")
		err := NewNetworkError("request", "https://api.example.com", "timeout", cause)

		assert.Equal(t, CategoryNetwork, err.Category)
		assert.Equal(t, SeverityHigh, err.Severity)
		assert.True(t, err.Recoverable)
		assert.Equal(t, time.Second*5, *err.RetryAfter)
		assert.Equal(t, "https://api.example.com", err.Context["endpoint"])
	})
}

func TestClassifyError(t *testing.T) {
	tests := []struct {
		name             string
		err              error
		expectedCategory ErrorCategory
		expectedSeverity ErrorSeverity
	}{
		{
			name:             "nil error",
			err:              nil,
			expectedCategory: CategoryInfrastructure,
			expectedSeverity: SeverityInfo,
		},
		{
			name:             "structured error",
			err:              NewSecurityError("test", "component", "security violation"),
			expectedCategory: CategorySecurity,
			expectedSeverity: SeverityCritical,
		},
		{
			name:             "image not found",
			err:              errors.New("No such image: nginx:latest"),
			expectedCategory: CategoryDocker,
			expectedSeverity: SeverityMedium,
		},
		{
			name:             "network timeout",
			err:              errors.New("connection timeout"),
			expectedCategory: CategoryNetwork,
			expectedSeverity: SeverityHigh,
		},
		{
			name:             "validation failed",
			err:              errors.New("validation failed: invalid format"),
			expectedCategory: CategoryValidation,
			expectedSeverity: SeverityMedium,
		},
		{
			name:             "permission denied",
			err:              errors.New("permission denied"),
			expectedCategory: CategorySecurity,
			expectedSeverity: SeverityHigh,
		},
		{
			name:             "unknown error",
			err:              errors.New("something went wrong"),
			expectedCategory: CategoryInfrastructure,
			expectedSeverity: SeverityMedium,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			category, severity := ClassifyError(tt.err)
			assert.Equal(t, tt.expectedCategory, category)
			assert.Equal(t, tt.expectedSeverity, severity)
		})
	}
}

func TestErrorClassificationFunctions(t *testing.T) {
	t.Run("IsImageNotFound", func(t *testing.T) {
		assert.True(t, IsImageNotFound(errors.New("No such image: nginx")))
		assert.True(t, IsImageNotFound(errors.New("image not found")))
		assert.False(t, IsImageNotFound(errors.New("other error")))
		assert.False(t, IsImageNotFound(nil))
	})

	t.Run("IsResourceNotFound", func(t *testing.T) {
		assert.True(t, IsResourceNotFound(errors.New("pod not found")))
		assert.True(t, IsResourceNotFound(errors.New("NotFound")))
		assert.False(t, IsResourceNotFound(errors.New("other error")))
	})

	t.Run("IsPermissionDenied", func(t *testing.T) {
		assert.True(t, IsPermissionDenied(errors.New("permission denied")))
		assert.True(t, IsPermissionDenied(errors.New("access forbidden")))
		assert.False(t, IsPermissionDenied(errors.New("other error")))
	})

	t.Run("IsNetworkError", func(t *testing.T) {
		assert.True(t, IsNetworkError(errors.New("connection refused")))
		assert.True(t, IsNetworkError(errors.New("network unreachable")))
		assert.False(t, IsNetworkError(errors.New("other error")))
	})
}

func TestIsRecoverableError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: true,
		},
		{
			name:     "structured recoverable",
			err:      NewNetworkError("request", "endpoint", "timeout", nil),
			expected: true,
		},
		{
			name:     "structured non-recoverable",
			err:      NewSecurityError("auth", "component", "security violation"),
			expected: false,
		},
		{
			name:     "network error",
			err:      errors.New("connection timeout"),
			expected: true,
		},
		{
			name:     "security error",
			err:      errors.New("security violation"),
			expected: false,
		},
		{
			name:     "validation error",
			err:      errors.New("validation failed"),
			expected: false,
		},
		{
			name:     "unknown error",
			err:      errors.New("unknown error"),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRecoverableError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetRetryDelay(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		attempt  int
		minDelay time.Duration
		maxDelay time.Duration
	}{
		{
			name:     "nil error",
			err:      nil,
			attempt:  1,
			minDelay: 0,
			maxDelay: 0,
		},
		{
			name:     "structured error with retry delay",
			err:      NewNetworkError("request", "endpoint", "timeout", nil),
			attempt:  1,
			minDelay: time.Second * 5,
			maxDelay: time.Second * 5,
		},
		{
			name:     "network error with exponential backoff",
			err:      errors.New("connection timeout"),
			attempt:  2,
			minDelay: time.Second * 2,
			maxDelay: time.Second * 10,
		},
		{
			name:     "ai error with longer delay",
			err:      NewAIError("sample", "model", "rate limited", nil),
			attempt:  1,
			minDelay: time.Second * 10,
			maxDelay: time.Second * 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delay := GetRetryDelay(tt.err, tt.attempt)
			if tt.minDelay == tt.maxDelay {
				assert.Equal(t, tt.minDelay, delay)
			} else {
				assert.GreaterOrEqual(t, delay, tt.minDelay)
				assert.LessOrEqual(t, delay, tt.maxDelay)
			}
		})
	}
}

func TestWrap(t *testing.T) {
	t.Run("wrap nil error", func(t *testing.T) {
		wrapped := Wrap(nil, "op", "comp", "msg")
		assert.Nil(t, wrapped)
	})

	t.Run("wrap standard error", func(t *testing.T) {
		original := errors.New("original error")
		wrapped := Wrap(original, "test_op", "test_comp", "wrapped message")

		assert.Equal(t, "test_op", wrapped.Operation)
		assert.Equal(t, "test_comp", wrapped.Component)
		assert.Equal(t, "wrapped message", wrapped.Message)
		assert.Equal(t, original, wrapped.Cause)
	})

	t.Run("wrap structured error", func(t *testing.T) {
		original := NewWorkflowError("build", "build failed", nil)
		wrapped := Wrap(original, "deploy", "kubernetes", "deployment failed")

		assert.Equal(t, "deploy", wrapped.Operation)
		assert.Equal(t, "kubernetes", wrapped.Component)
		assert.Equal(t, "deployment failed", wrapped.Message)
		assert.Equal(t, original, wrapped.Cause)
		assert.Equal(t, CategoryWorkflow, wrapped.Category) // Preserved from original
	})
}

func TestWrapContext(t *testing.T) {
	original := errors.New("original error")
	context := map[string]interface{}{
		"key1": "value1",
		"key2": 42,
	}

	wrapped := WrapContext(original, "test_op", "test_comp", "wrapped message", context)

	assert.Equal(t, "test_op", wrapped.Operation)
	assert.Equal(t, "value1", wrapped.Context["key1"])
	assert.Equal(t, 42, wrapped.Context["key2"])
}

func TestErrorAggregator(t *testing.T) {
	aggregator := NewErrorAggregator(time.Hour)

	// Add some test errors
	err1 := NewWorkflowError("build", "build failed", nil)
	err2 := NewSecurityError("auth", "component", "security violation")
	err3 := NewValidationError("field", "invalid value")

	aggregator.Add(err1)
	aggregator.Add(err2)
	aggregator.Add(err3)

	// Test report generation
	report := aggregator.GetReport()

	assert.Equal(t, int64(3), report.TotalErrors)
	assert.Len(t, report.Categories, 3)
	assert.Len(t, report.Severities, 3)
	assert.NotEmpty(t, report.TopErrors)

	// Test category stats
	workflowStats, exists := report.Categories[CategoryWorkflow]
	assert.True(t, exists)
	assert.Equal(t, int64(1), workflowStats.Count)

	securityStats, exists := report.Categories[CategorySecurity]
	assert.True(t, exists)
	assert.Equal(t, int64(1), securityStats.Count)

	// Test critical errors
	criticalErrors := aggregator.GetCriticalErrors()
	assert.Len(t, criticalErrors, 1)
	assert.Equal(t, CategorySecurity, criticalErrors[0].Category)

	// Test recoverable errors
	recoverableErrors := aggregator.GetRecoverableErrors()
	assert.Len(t, recoverableErrors, 1) // Only workflow error is recoverable
	assert.Equal(t, CategoryWorkflow, recoverableErrors[0].Category)
}

func TestErrorAggregatorFromError(t *testing.T) {
	aggregator := NewErrorAggregator(time.Hour)

	// Add regular error
	err := errors.New("regular error")
	aggregator.AddFromError(err, "test_op", "test_comp")

	report := aggregator.GetReport()
	assert.Equal(t, int64(1), report.TotalErrors)

	// Verify the error was properly wrapped
	assert.Len(t, report.TopErrors, 1)
	structErr := report.TopErrors[0]
	assert.Equal(t, "test_op", structErr.Operation)
	assert.Equal(t, "test_comp", structErr.Component)
	assert.Equal(t, err, structErr.Cause)
}

func TestSummarizeError(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		attempt int
	}{
		{
			name:    "nil error",
			err:     nil,
			attempt: 1,
		},
		{
			name:    "network error",
			err:     errors.New("connection timeout"),
			attempt: 2,
		},
		{
			name:    "validation error",
			err:     errors.New("validation failed"),
			attempt: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := SummarizeError(tt.err, tt.attempt)
			require.NotNil(t, summary)
			assert.NotEmpty(t, summary.Category)
			assert.NotEmpty(t, summary.Severity)
		})
	}
}

func TestFormat(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: "no error",
		},
		{
			name:     "structured error",
			err:      NewValidationError("field", "invalid"),
			expected: "[validation] invalid",
		},
		{
			name:     "standard error",
			err:      errors.New("standard error"),
			expected: "[infrastructure] standard error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Format(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
