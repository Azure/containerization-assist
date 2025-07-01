package errors

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCoreError_Creation(t *testing.T) {
	tests := []struct {
		name     string
		createFn func() *CoreError
		expected struct {
			module   string
			category ErrorCategory
			severity Severity
		}
	}{
		{
			name: "validation_error",
			createFn: func() *CoreError {
				return Validation("test", "invalid input provided")
			},
			expected: struct {
				module   string
				category ErrorCategory
				severity Severity
			}{
				module:   "test",
				category: CategoryValidation,
				severity: SeverityMedium,
			},
		},
		{
			name: "network_error",
			createFn: func() *CoreError {
				return Network("network", "connection timeout")
			},
			expected: struct {
				module   string
				category ErrorCategory
				severity Severity
			}{
				module:   "network",
				category: CategoryNetwork,
				severity: SeverityMedium,
			},
		},
		{
			name: "internal_error",
			createFn: func() *CoreError {
				return Internal("system", "unexpected panic")
			},
			expected: struct {
				module   string
				category ErrorCategory
				severity Severity
			}{
				module:   "system",
				category: CategoryInternal,
				severity: SeverityMedium,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.createFn()

			assert.Equal(t, tt.expected.module, err.Module)
			assert.Equal(t, tt.expected.category, err.Category)
			assert.Equal(t, tt.expected.severity, err.Severity)
			assert.NotZero(t, err.Timestamp)
			assert.NotNil(t, err.Context)
		})
	}
}

func TestCoreError_ErrorInterface(t *testing.T) {
	err := New("test", "test error message", CategoryValidation)

	// Test Error() method
	errorStr := err.Error()
	assert.Contains(t, errorStr, "test error message")
	assert.Contains(t, errorStr, "mcp/test")

	// Test with no module
	err.Module = ""
	errorStr = err.Error()
	assert.Contains(t, errorStr, "test error message")
	assert.Contains(t, errorStr, "mcp:")
}

func TestCoreError_WithContext(t *testing.T) {
	err := New("test", "test error", CategoryValidation)

	err.WithContext("user_id", "user123")
	err.WithContext("operation", "validate_input")

	assert.Equal(t, "user123", err.Context["user_id"])
	assert.Equal(t, "validate_input", err.Context["operation"])
}

func TestCoreError_WithSession(t *testing.T) {
	err := New("test", "test error", CategoryValidation)

	err.WithSession("session123", "analyze", "validation", "input_validator")

	assert.Equal(t, "session123", err.SessionID)
	assert.Equal(t, "analyze", err.Tool)
	assert.Equal(t, "validation", err.Stage)
	assert.Equal(t, "input_validator", err.Component)
}

func TestCoreError_WithDiagnostics(t *testing.T) {
	err := New("test", "test error", CategoryValidation)

	diagnostics := &ErrorDiagnostics{
		RootCause:    "Invalid input format",
		ErrorPattern: "VALIDATION_FAILED",
		Symptoms:     []string{"malformed JSON", "missing required field"},
		Checks: []DiagnosticCheck{
			{
				Name:    "JSON validation",
				Status:  "fail",
				Details: "Syntax error at line 5",
			},
		},
	}

	err.WithDiagnostics(diagnostics)

	assert.NotNil(t, err.Diagnostics)
	assert.Equal(t, "Invalid input format", err.Diagnostics.RootCause)
	assert.Equal(t, "VALIDATION_FAILED", err.Diagnostics.ErrorPattern)
	assert.Len(t, err.Diagnostics.Symptoms, 2)
	assert.Len(t, err.Diagnostics.Checks, 1)
}

func TestCoreError_WithResolution(t *testing.T) {
	err := New("test", "test error", CategoryValidation)

	resolution := &ErrorResolution{
		ImmediateSteps: []ResolutionStep{
			{
				Step:        1,
				Action:      "Fix JSON syntax",
				Description: "Correct the malformed JSON",
				Expected:    "Valid JSON structure",
			},
		},
		Alternatives: []Alternative{
			{
				Approach:    "Use schema validation",
				Description: "Implement JSON schema validation",
				Effort:      "medium",
				Risk:        "low",
			},
		},
		Prevention: []string{
			"Add JSON validation before processing",
			"Use strict parsing mode",
		},
	}

	err.WithResolution(resolution)

	assert.NotNil(t, err.Resolution)
	assert.Len(t, err.Resolution.ImmediateSteps, 1)
	assert.Len(t, err.Resolution.Alternatives, 1)
	assert.Len(t, err.Resolution.Prevention, 2)
}

func TestCoreError_WithSystemState(t *testing.T) {
	err := New("test", "test error", CategoryResource)

	state := &SystemState{
		DockerAvailable: true,
		K8sConnected:    false,
		DiskSpaceMB:     1024,
		MemoryMB:        8192,
		LoadAverage:     1.5,
	}

	err.WithSystemState(state)

	assert.NotNil(t, err.SystemState)
	assert.True(t, err.SystemState.DockerAvailable)
	assert.False(t, err.SystemState.K8sConnected)
	assert.Equal(t, int64(1024), err.SystemState.DiskSpaceMB)
}

func TestCoreError_WithResourceUsage(t *testing.T) {
	err := New("test", "test error", CategoryResource)

	usage := &ResourceUsage{
		CPUPercent:     85.5,
		MemoryMB:       2048,
		DiskUsageMB:    512,
		NetworkBytesTx: 1024000,
		NetworkBytesRx: 2048000,
	}

	err.WithResourceUsage(usage)

	assert.NotNil(t, err.ResourceUsage)
	assert.Equal(t, 85.5, err.ResourceUsage.CPUPercent)
	assert.Equal(t, int64(2048), err.ResourceUsage.MemoryMB)
}

func TestCoreError_Wrap(t *testing.T) {
	originalErr := fmt.Errorf("original error")

	wrappedErr := Wrap(originalErr, "wrapper", "failed to process")

	assert.NotNil(t, wrappedErr)
	assert.Equal(t, "wrapper", wrappedErr.Module)
	assert.Equal(t, "failed to process", wrappedErr.Message)
	assert.Equal(t, originalErr, wrappedErr.Cause)
	assert.Equal(t, "original error", wrappedErr.CauseStr)

	// Test Unwrap
	assert.Equal(t, originalErr, wrappedErr.Unwrap())
}

func TestCoreError_WrapCoreError(t *testing.T) {
	originalErr := New("original", "original error", CategoryValidation)
	originalErr.SetRetryable(true).SetFatal(true)

	wrappedErr := Wrap(originalErr, "wrapper", "wrapped error")

	assert.NotNil(t, wrappedErr)
	assert.Equal(t, "wrapper", wrappedErr.Module)
	assert.Equal(t, "wrapped error", wrappedErr.Message)
	assert.Equal(t, CategoryValidation, wrappedErr.Category) // Preserved from original
	assert.True(t, wrappedErr.Retryable)                     // Preserved from original
	assert.True(t, wrappedErr.Fatal)                         // Preserved from original
	assert.Len(t, wrappedErr.Wrapped, 1)
	assert.Equal(t, "original error", wrappedErr.Wrapped[0])
}

func TestCoreError_JSONSerialization(t *testing.T) {
	err := New("test", "test error", CategoryValidation)
	err.WithContext("test_key", "test_value")
	err.SetRetryable(true)

	jsonData, jsonErr := err.ToJSON()
	require.NoError(t, jsonErr)
	assert.NotEmpty(t, jsonData)

	// Verify it's valid JSON by parsing back
	assert.Contains(t, string(jsonData), "test error")
	assert.Contains(t, string(jsonData), "validation")
	assert.Contains(t, string(jsonData), "test_key")
}

func TestCoreError_Is(t *testing.T) {
	err1 := New("test", "error 1", CategoryValidation)
	err1.Code = "TEST_001"

	err2 := New("test", "error 2", CategoryValidation)
	err2.Code = "TEST_001"

	err3 := New("test", "error 3", CategoryNetwork)
	err3.Code = "TEST_001"

	// Same category, module, and code should match
	assert.True(t, err1.Is(err2))

	// Different category should not match
	assert.False(t, err1.Is(err3))

	// Test with wrapped error
	originalErr := fmt.Errorf("original")
	wrappedErr := Wrap(originalErr, "test", "wrapped")
	assert.True(t, wrappedErr.Is(originalErr))
}

func TestCoreError_SetFlags(t *testing.T) {
	err := New("test", "test error", CategoryValidation)

	// Test default values
	assert.False(t, err.Retryable)
	assert.True(t, err.Recoverable)
	assert.False(t, err.Fatal)

	// Test setting flags
	err.SetRetryable(true)
	err.SetRecoverable(false)
	err.SetFatal(true)

	assert.True(t, err.Retryable)
	assert.False(t, err.Recoverable)
	assert.True(t, err.Fatal)
}

func TestCoreError_ConstructorFunctions(t *testing.T) {
	tests := []struct {
		name        string
		createFn    func() *CoreError
		category    ErrorCategory
		retryable   bool
		recoverable bool
		fatal       bool
	}{
		{
			name:        "validation",
			createFn:    func() *CoreError { return Validation("test", "validation failed") },
			category:    CategoryValidation,
			retryable:   false,
			recoverable: true,
			fatal:       false,
		},
		{
			name:        "network",
			createFn:    func() *CoreError { return Network("test", "network failed") },
			category:    CategoryNetwork,
			retryable:   true,
			recoverable: true,
			fatal:       false,
		},
		{
			name:        "internal",
			createFn:    func() *CoreError { return Internal("test", "internal failed") },
			category:    CategoryInternal,
			retryable:   false,
			recoverable: false,
			fatal:       false,
		},
		{
			name:        "auth",
			createFn:    func() *CoreError { return Auth("test", "auth failed") },
			category:    CategoryAuth,
			retryable:   false,
			recoverable: true,
			fatal:       false,
		},
		{
			name:        "security",
			createFn:    func() *CoreError { return Security("test", "security violation") },
			category:    CategorySecurity,
			retryable:   false,
			recoverable: true,
			fatal:       true,
		},
		{
			name:        "session",
			createFn:    func() *CoreError { return Session("test", "session failed") },
			category:    CategorySession,
			retryable:   false,
			recoverable: false,
			fatal:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.createFn()

			assert.Equal(t, tt.category, err.Category)
			assert.Equal(t, tt.retryable, err.Retryable)
			assert.Equal(t, tt.recoverable, err.Recoverable)
			assert.Equal(t, tt.fatal, err.Fatal)
		})
	}
}

func TestCoreError_WithFiles(t *testing.T) {
	err := New("test", "test error", CategoryBuild)

	files := []string{
		"/path/to/Dockerfile",
		"/path/to/config.yaml",
		"/path/to/source.go",
	}

	err.WithFiles(files)

	assert.Equal(t, files, err.RelatedFiles)
	assert.Len(t, err.RelatedFiles, 3)
}

func TestCoreError_WithLogs(t *testing.T) {
	err := New("test", "test error", CategoryBuild)

	logs := []LogEntry{
		{
			Timestamp: time.Now(),
			Level:     "ERROR",
			Message:   "Build failed at step 5",
			Source:    "docker",
		},
		{
			Timestamp: time.Now().Add(-time.Minute),
			Level:     "WARN",
			Message:   "Deprecated dependency detected",
			Source:    "npm",
		},
	}

	err.WithLogs(logs)

	assert.Equal(t, logs, err.LogEntries)
	assert.Len(t, err.LogEntries, 2)
}

// Benchmark tests for error creation and serialization
func BenchmarkCoreError_Creation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = New("test", "benchmark error", CategoryValidation)
	}
}

func BenchmarkCoreError_WithContext(b *testing.B) {
	err := New("test", "benchmark error", CategoryValidation)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err.WithContext(fmt.Sprintf("key_%d", i), fmt.Sprintf("value_%d", i))
	}
}

func BenchmarkCoreError_ToJSON(b *testing.B) {
	err := New("test", "benchmark error", CategoryValidation)
	err.WithContext("user_id", "user123")
	err.WithSession("session123", "tool", "stage", "component")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = err.ToJSON()
	}
}

func BenchmarkCoreError_Wrap(b *testing.B) {
	originalErr := fmt.Errorf("original error")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Wrap(originalErr, "wrapper", "wrapped error")
	}
}
