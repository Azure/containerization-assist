package errors

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMCPError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *MCPError
		expected string
	}{
		{
			name: "error with module",
			err: &MCPError{
				Module:  "test-module",
				Message: "test error message",
			},
			expected: "mcp/test-module: test error message",
		},
		{
			name: "error without module",
			err: &MCPError{
				Message: "test error message",
			},
			expected: "mcp: test error message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.err.Error())
		})
	}
}

func TestMCPError_Unwrap(t *testing.T) {
	originalErr := fmt.Errorf("original error")
	mcpErr := &MCPError{
		Module:  "test",
		Message: "wrapped error",
		Cause:   originalErr,
	}

	assert.Equal(t, originalErr, mcpErr.Unwrap())
}

func TestMCPError_Is(t *testing.T) {
	t.Run("matches same category and module", func(t *testing.T) {
		err1 := &MCPError{Category: CategoryValidation, Module: "test"}
		err2 := &MCPError{Category: CategoryValidation, Module: "test"}
		assert.True(t, err1.Is(err2))
	})

	t.Run("different category", func(t *testing.T) {
		err1 := &MCPError{Category: CategoryValidation, Module: "test"}
		err2 := &MCPError{Category: CategoryNetwork, Module: "test"}
		assert.False(t, err1.Is(err2))
	})

	t.Run("different module", func(t *testing.T) {
		err1 := &MCPError{Category: CategoryValidation, Module: "test1"}
		err2 := &MCPError{Category: CategoryValidation, Module: "test2"}
		assert.False(t, err1.Is(err2))
	})

	t.Run("checks cause with errors.Is", func(t *testing.T) {
		originalErr := fmt.Errorf("original")
		mcpErr := &MCPError{Cause: originalErr}
		assert.True(t, mcpErr.Is(originalErr))
	})
}

func TestMCPError_WithContext(t *testing.T) {
	err := &MCPError{Module: "test", Message: "test error"}
	_ = err.WithContext("key1", "value1").WithContext("key2", 42)

	assert.Equal(t, "value1", err.Context["key1"])
	assert.Equal(t, 42, err.Context["key2"])
}

func TestNew(t *testing.T) {
	err := New("test-module", "test message", CategoryValidation)

	assert.Equal(t, "test-module", err.Module)
	assert.Equal(t, "test message", err.Message)
	assert.Equal(t, CategoryValidation, err.Category)
	assert.NotNil(t, err.Context)
}

func TestNewf(t *testing.T) {
	err := Newf("test-module", CategoryValidation, "test %s with %d", "message", 42)

	assert.Equal(t, "test-module", err.Module)
	assert.Equal(t, "test message with 42", err.Message)
	assert.Equal(t, CategoryValidation, err.Category)
}

func TestWrap(t *testing.T) {
	t.Run("wrap nil error returns nil", func(t *testing.T) {
		result := Wrap(nil, "module", "message")
		assert.Nil(t, result)
	})

	t.Run("wrap regular error", func(t *testing.T) {
		originalErr := fmt.Errorf("original error")
		wrapped := Wrap(originalErr, "test-module", "wrapped message")

		assert.Equal(t, "test-module", wrapped.Module)
		assert.Equal(t, "wrapped message", wrapped.Message)
		assert.Equal(t, CategoryInternal, wrapped.Category)
		assert.Equal(t, originalErr, wrapped.Cause)
	})

	t.Run("wrap existing MCPError preserves category", func(t *testing.T) {
		originalMCPErr := &MCPError{
			Category:  CategoryNetwork,
			Module:    "original-module",
			Message:   "original message",
			Operation: "test-op",
			Retryable: true,
		}
		wrapped := Wrap(originalMCPErr, "new-module", "new message")

		assert.Equal(t, "new-module", wrapped.Module)
		assert.Equal(t, "new message", wrapped.Message)
		assert.Equal(t, CategoryNetwork, wrapped.Category) // Preserved
		assert.Equal(t, "test-op", wrapped.Operation)      // Preserved
		assert.True(t, wrapped.Retryable)                  // Preserved
		assert.Equal(t, originalMCPErr, wrapped.Cause)
	})
}

func TestWrapf(t *testing.T) {
	originalErr := fmt.Errorf("original error")
	wrapped := Wrapf(originalErr, "test-module", "wrapped %s with %d", "message", 42)

	assert.Equal(t, "test-module", wrapped.Module)
	assert.Equal(t, "wrapped message with 42", wrapped.Message)
	assert.Equal(t, originalErr, wrapped.Cause)
}

func TestValidation(t *testing.T) {
	err := Validation("test-module", "validation failed")

	assert.Equal(t, "test-module", err.Module)
	assert.Equal(t, "validation failed", err.Message)
	assert.Equal(t, CategoryValidation, err.Category)
}

func TestValidationf(t *testing.T) {
	err := Validationf("test-module", "field %s is %s", "name", "required")

	assert.Equal(t, "test-module", err.Module)
	assert.Equal(t, "field name is required", err.Message)
	assert.Equal(t, CategoryValidation, err.Category)
}

func TestNetwork(t *testing.T) {
	err := Network("test-module", "connection failed")

	assert.Equal(t, "test-module", err.Module)
	assert.Equal(t, "connection failed", err.Message)
	assert.Equal(t, CategoryNetwork, err.Category)
	assert.True(t, err.Retryable)
}

func TestNetworkf(t *testing.T) {
	err := Networkf("test-module", "failed to connect to %s:%d", "localhost", 8080)

	assert.Equal(t, "test-module", err.Module)
	assert.Equal(t, "failed to connect to localhost:8080", err.Message)
	assert.Equal(t, CategoryNetwork, err.Category)
	assert.True(t, err.Retryable)
}

func TestInternal(t *testing.T) {
	err := Internal("test-module", "internal error")

	assert.Equal(t, "test-module", err.Module)
	assert.Equal(t, "internal error", err.Message)
	assert.Equal(t, CategoryInternal, err.Category)
}

func TestInternalf(t *testing.T) {
	err := Internalf("test-module", "unexpected error: %v", "database failure")

	assert.Equal(t, "test-module", err.Module)
	assert.Equal(t, "unexpected error: database failure", err.Message)
	assert.Equal(t, CategoryInternal, err.Category)
}

func TestResource(t *testing.T) {
	err := Resource("test-module", "resource not found")

	assert.Equal(t, "test-module", err.Module)
	assert.Equal(t, "resource not found", err.Message)
	assert.Equal(t, CategoryResource, err.Category)
}

func TestResourcef(t *testing.T) {
	err := Resourcef("test-module", "resource %s not found in %s", "file.txt", "/tmp")

	assert.Equal(t, "test-module", err.Module)
	assert.Equal(t, "resource file.txt not found in /tmp", err.Message)
	assert.Equal(t, CategoryResource, err.Category)
}

func TestTimeout(t *testing.T) {
	err := Timeout("test-module", "operation timed out")

	assert.Equal(t, "test-module", err.Module)
	assert.Equal(t, "operation timed out", err.Message)
	assert.Equal(t, CategoryTimeout, err.Category)
	assert.True(t, err.Retryable)
}

func TestTimeoutf(t *testing.T) {
	err := Timeoutf("test-module", "operation timed out after %d seconds", 30)

	assert.Equal(t, "test-module", err.Module)
	assert.Equal(t, "operation timed out after 30 seconds", err.Message)
	assert.Equal(t, CategoryTimeout, err.Category)
	assert.True(t, err.Retryable)
}

func TestConfig(t *testing.T) {
	err := Config("test-module", "invalid configuration")

	assert.Equal(t, "test-module", err.Module)
	assert.Equal(t, "invalid configuration", err.Message)
	assert.Equal(t, CategoryConfig, err.Category)
}

func TestConfigf(t *testing.T) {
	err := Configf("test-module", "missing required config: %s", "database_url")

	assert.Equal(t, "test-module", err.Module)
	assert.Equal(t, "missing required config: database_url", err.Message)
	assert.Equal(t, CategoryConfig, err.Category)
}

func TestAuth(t *testing.T) {
	err := Auth("test-module", "access denied")

	assert.Equal(t, "test-module", err.Module)
	assert.Equal(t, "access denied", err.Message)
	assert.Equal(t, CategoryAuth, err.Category)
}

func TestAuthf(t *testing.T) {
	err := Authf("test-module", "user %s does not have permission %s", "john", "admin")

	assert.Equal(t, "test-module", err.Module)
	assert.Equal(t, "user john does not have permission admin", err.Message)
	assert.Equal(t, CategoryAuth, err.Category)
}

func TestIsCategory(t *testing.T) {
	t.Run("MCPError with matching category", func(t *testing.T) {
		err := &MCPError{Category: CategoryValidation}
		assert.True(t, IsCategory(err, CategoryValidation))
		assert.False(t, IsCategory(err, CategoryNetwork))
	})

	t.Run("non-MCPError", func(t *testing.T) {
		err := fmt.Errorf("regular error")
		assert.False(t, IsCategory(err, CategoryValidation))
	})
}

func TestIsRetryable(t *testing.T) {
	t.Run("retryable MCPError", func(t *testing.T) {
		err := &MCPError{Retryable: true}
		assert.True(t, IsRetryable(err))
	})

	t.Run("non-retryable MCPError", func(t *testing.T) {
		err := &MCPError{Retryable: false}
		assert.False(t, IsRetryable(err))
	})

	t.Run("non-MCPError", func(t *testing.T) {
		err := fmt.Errorf("regular error")
		assert.False(t, IsRetryable(err))
	})
}

func TestIsRecoverable(t *testing.T) {
	t.Run("recoverable MCPError", func(t *testing.T) {
		err := &MCPError{Recoverable: true}
		assert.True(t, IsRecoverable(err))
	})

	t.Run("non-recoverable MCPError", func(t *testing.T) {
		err := &MCPError{Recoverable: false}
		assert.False(t, IsRecoverable(err))
	})

	t.Run("non-MCPError", func(t *testing.T) {
		err := fmt.Errorf("regular error")
		assert.False(t, IsRecoverable(err))
	})
}

func TestGetModule(t *testing.T) {
	t.Run("MCPError with module", func(t *testing.T) {
		err := &MCPError{Module: "test-module"}
		assert.Equal(t, "test-module", GetModule(err))
	})

	t.Run("non-MCPError", func(t *testing.T) {
		err := fmt.Errorf("regular error")
		assert.Equal(t, "", GetModule(err))
	})
}

func TestGetCategory(t *testing.T) {
	t.Run("MCPError with category", func(t *testing.T) {
		err := &MCPError{Category: CategoryValidation}
		assert.Equal(t, CategoryValidation, GetCategory(err))
	})

	t.Run("non-MCPError defaults to internal", func(t *testing.T) {
		err := fmt.Errorf("regular error")
		assert.Equal(t, CategoryInternal, GetCategory(err))
	})
}

func TestErrorChaining(t *testing.T) {
	// Test error unwrapping chain
	originalErr := fmt.Errorf("root cause")
	wrapped1 := Wrap(originalErr, "module1", "first wrap")
	wrapped2 := Wrap(wrapped1, "module2", "second wrap")

	// Test error chain
	assert.True(t, errors.Is(wrapped2, originalErr))
	assert.True(t, errors.Is(wrapped2, wrapped1))

	// Test unwrapping
	assert.Equal(t, wrapped1, wrapped2.Unwrap())
	assert.Equal(t, originalErr, wrapped1.Unwrap())
}

// Example demonstrating typical usage patterns
func ExampleMCPError() {
	// Create a validation error
	err := Validation("user-service", "email address is required")
	fmt.Println(err.Error())

	// Wrap an existing error
	dbErr := fmt.Errorf("connection refused")
	wrappedErr := Wrap(dbErr, "database", "failed to connect to user database")
	fmt.Println(wrappedErr.Error())

	// Add context to an error
	contextErr := Network("api-client", "request failed").
		WithContext("url", "https://api.example.com").
		WithContext("status_code", 500)
	fmt.Println(contextErr.Error())

	// Output:
	// mcp/user-service: email address is required
	// mcp/database: failed to connect to user database
	// mcp/api-client: request failed
}
