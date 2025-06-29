package transport

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/mcp"
	"github.com/localrivet/gomcp/server"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStdioErrorHandler_HandleToolError(t *testing.T) {
	logger := zerolog.Nop()
	handler := NewStdioErrorHandler(logger)

	tests := []struct {
		name     string
		err      error
		toolName string
		wantType string
	}{
		{
			name:     "generic error",
			err:      errors.New("something went wrong"),
			toolName: "test_tool",
			wantType: "generic_error",
		},
		{
			name:     "network error",
			err:      errors.New("network connection failed"),
			toolName: "test_tool",
			wantType: "network_error",
		},
		{
			name:     "timeout error",
			err:      errors.New("operation timed out"),
			toolName: "test_tool",
			wantType: "timeout_error",
		},
		{
			name:     "permission error",
			err:      errors.New("permission denied"),
			toolName: "test_tool",
			wantType: "permission_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result, err := handler.HandleToolError(ctx, tt.toolName, tt.err)

			require.NoError(t, err)
			require.NotNil(t, result)

			// Check response structure
			response, ok := result.(map[string]interface{})
			require.True(t, ok, "result should be a map")

			// Verify error response structure
			isError, ok := response["isError"].(bool)
			assert.True(t, ok, "isError should be a bool")
			assert.True(t, isError)

			errorInfo, ok := response["error"].(map[string]interface{})
			require.True(t, ok, "error field should be a map")

			assert.Equal(t, tt.toolName, errorInfo["tool"])
			errorType, ok := errorInfo["type"].(string)
			assert.True(t, ok, "type should be a string")
			assert.Contains(t, errorType, "error")
			assert.NotEmpty(t, errorInfo["message"])
			assert.NotNil(t, errorInfo["timestamp"])
		})
	}
}

func TestStdioErrorHandler_HandleRichError(t *testing.T) {
	logger := zerolog.Nop()
	handler := NewStdioErrorHandler(logger)

	// Create a rich error with comprehensive information
	richErr := mcp.NewRichError("BUILD_FAILED", "Docker build failed", "build_error")
	richErr.Severity = "high"
	richErr.Context.Operation = "docker_build"
	richErr.Context.Stage = "compilation"
	richErr.Context.Component = "dockerfile"
	richErr.Diagnostics.RootCause = "Missing dependency"
	richErr.Diagnostics.Symptoms = []string{"Package not found", "Build step failed"}

	// Add resolution steps
	richErr.Resolution.ImmediateSteps = []mcp.ResolutionStep{
		{
			Order:       1,
			Action:      "Check Dockerfile",
			Description: "Review Dockerfile for syntax errors",
			Command:     "docker build --dry-run .",
			Expected:    "Dockerfile should validate successfully",
		},
	}

	// Add alternatives
	richErr.Resolution.Alternatives = []mcp.Alternative{
		{
			Name:        "Use different base image",
			Description: "Try using alpine instead of ubuntu",
			Steps:       []string{"Update FROM line", "Adjust package commands"},
			Confidence:  0.8,
		},
	}

	// Add retry strategy
	richErr.Resolution.RetryStrategy = mcp.RetryStrategy{
		Recommended:     true,
		WaitTime:        30 * time.Second,
		MaxAttempts:     3,
		BackoffStrategy: "exponential",
		Conditions:      []string{"Fix Dockerfile", "Check network"},
	}

	result := handler.handleRichError(richErr, "build_image")

	// Verify comprehensive error response
	response, ok := result.(map[string]interface{})
	require.True(t, ok)

	isError, ok := response["isError"].(bool)
	assert.True(t, ok, "isError should be a bool")
	assert.True(t, isError)

	// Check error information
	errorInfo, ok := response["error"].(map[string]interface{})
	require.True(t, ok, "error should be a map")
	assert.Equal(t, "BUILD_FAILED", errorInfo["code"])
	assert.Equal(t, "build_error", errorInfo["type"])
	assert.Equal(t, "high", errorInfo["severity"])
	assert.Equal(t, "build_image", errorInfo["tool"])

	// Check resolution steps
	steps, ok := response["resolution_steps"].([]map[string]interface{})
	require.True(t, ok)
	require.Len(t, steps, 1)
	assert.Equal(t, "Check Dockerfile", steps[0]["action"])
	assert.Equal(t, "docker build --dry-run .", steps[0]["command"])

	// Check alternatives
	alternatives, ok := response["alternatives"].([]map[string]interface{})
	require.True(t, ok)
	require.Len(t, alternatives, 1)
	assert.Equal(t, "Use different base image", alternatives[0]["name"])
	assert.Equal(t, 0.8, alternatives[0]["confidence"])

	// Check retry strategy
	retryStrategy, ok := response["retry_strategy"].(map[string]interface{})
	require.True(t, ok, "retry_strategy should be a map")
	recommended, ok := retryStrategy["recommended"].(bool)
	assert.True(t, ok, "recommended should be a bool")
	assert.True(t, recommended)
	assert.Equal(t, float64(30), retryStrategy["wait_time"])
	assert.Equal(t, 3, retryStrategy["max_attempts"])

	// Check diagnostics
	diagnostics, ok := response["diagnostics"].(map[string]interface{})
	require.True(t, ok, "diagnostics should be a map")
	assert.Equal(t, "Missing dependency", diagnostics["root_cause"])
	assert.Contains(t, diagnostics["symptoms"], "Package not found")
}

func TestStdioErrorHandler_HandleCancellation(t *testing.T) {
	logger := zerolog.Nop()
	handler := NewStdioErrorHandler(logger)

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := handler.HandleToolError(ctx, "test_tool", errors.New("operation failed"))

	require.NoError(t, err)
	require.NotNil(t, result)

	response, ok := result.(map[string]interface{})
	require.True(t, ok)

	isError, ok := response["isError"].(bool)
	assert.True(t, ok, "isError should be a bool")
	assert.True(t, isError)
	cancelled, ok := response["cancelled"].(bool)
	assert.True(t, ok, "cancelled should be a bool")
	assert.True(t, cancelled)

	errorInfo, ok := response["error"].(map[string]interface{})
	require.True(t, ok, "error should be a map")
	assert.Equal(t, "cancellation", errorInfo["type"])
	retryable, ok := errorInfo["retryable"].(bool)
	assert.True(t, ok, "retryable should be a bool")
	assert.True(t, retryable)
}

func TestStdioErrorHandler_InvalidParametersError(t *testing.T) {
	logger := zerolog.Nop()
	handler := NewStdioErrorHandler(logger)

	invalidParamsErr := &server.InvalidParametersError{
		Message: "field 'name' is required",
	}

	result, err := handler.HandleToolError(context.Background(), "test_tool", invalidParamsErr)

	// Invalid parameters errors should be passed through to gomcp
	assert.Nil(t, result)
	assert.Error(t, err)
	assert.IsType(t, &server.InvalidParametersError{}, err)
}

func TestStdioErrorHandler_CreateErrorResponse(t *testing.T) {
	logger := zerolog.Nop()
	handler := NewStdioErrorHandler(logger)

	response := handler.CreateErrorResponse("test-id", -32603, "Internal error", "additional data")

	assert.Equal(t, "2.0", response["jsonrpc"])
	assert.Equal(t, "test-id", response["id"])

	errorInfo, ok := response["error"].(map[string]interface{})
	require.True(t, ok, "error should be a map")
	assert.Equal(t, -32603, errorInfo["code"])
	assert.Equal(t, "Internal error", errorInfo["message"])
	assert.Equal(t, "additional data", errorInfo["data"])
}

func TestStdioErrorHandler_FormatErrorMessages(t *testing.T) {
	logger := zerolog.Nop()
	handler := NewStdioErrorHandler(logger)

	tests := []struct {
		name     string
		richErr  *mcp.RichError
		expected []string // Substrings that should be in the formatted message
	}{
		{
			name:     "basic error",
			richErr:  mcp.NewRichError("", "Build failed", "build_error"),
			expected: []string{"❌", "build_error", "Build failed"},
		},
		{
			name: "error with context",
			richErr: func() *mcp.RichError {
				err := mcp.NewRichError("", "Deployment failed", "deploy_error")
				err.Context.Operation = "kubernetes_deploy"
				err.Context.Stage = "apply"
				err.Context.Component = "deployment"
				return err
			}(),
			expected: []string{"❌", "deploy_error", "🔍 Context", "kubernetes_deploy", "apply", "deployment"},
		},
		{
			name: "error with resolution steps",
			richErr: func() *mcp.RichError {
				err := mcp.NewRichError("", "Connection failed", "network_error")
				err.Resolution.ImmediateSteps = []mcp.ResolutionStep{
					{Order: 1, Action: "Check network", Command: "ping google.com"},
					{Order: 2, Action: "Restart service", Command: "systemctl restart network"},
				}
				return err
			}(),
			expected: []string{"❌", "network_error", "🔧 Immediate Steps", "1. Check network", "2. Restart service"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatted := handler.formatRichErrorMessage(tt.richErr)

			for _, expected := range tt.expected {
				assert.Contains(t, formatted, expected, "Message should contain: %s", expected)
			}
		})
	}
}

func TestStdioErrorHandler_ErrorCategorization(t *testing.T) {
	logger := zerolog.Nop()
	handler := NewStdioErrorHandler(logger)

	tests := []struct {
		error     string
		category  string
		retryable bool
	}{
		{"network connection failed", "network_error", true},
		{"operation timeout occurred", "timeout_error", true},
		{"permission denied", "permission_error", false},
		{"file not found", "not_found_error", false},
		{"invalid input format", "validation_error", false},
		{"disk space full", "disk_error", false},
		{"something random happened", "generic_error", false},
		{"resource temporarily unavailable", "generic_error", true},
	}

	for _, tt := range tests {
		t.Run(tt.error, func(t *testing.T) {
			err := errors.New(tt.error)

			category := handler.categorizeError(err)
			assert.Equal(t, tt.category, category)

			retryable := handler.isRetryableError(err)
			assert.Equal(t, tt.retryable, retryable)
		})
	}
}

func TestStdioTransport_ErrorHandling(t *testing.T) {
	logger := zerolog.Nop()
	transport := NewStdioTransportWithLogger(logger)

	t.Run("HandleToolError", func(t *testing.T) {
		ctx := context.Background()
		err := errors.New("test error")

		result, handlerErr := transport.HandleToolError(ctx, "test_tool", err)

		require.NoError(t, handlerErr)
		require.NotNil(t, result)

		response, ok := result.(map[string]interface{})
		require.True(t, ok)
		isError, ok := response["isError"].(bool)
		assert.True(t, ok, "isError should be a bool")
		assert.True(t, isError)
	})

	t.Run("CreateErrorResponse", func(t *testing.T) {
		response := transport.CreateErrorResponse("test-id", -32603, "Test error", nil)

		assert.Equal(t, "2.0", response["jsonrpc"])
		assert.Equal(t, "test-id", response["id"])

		errorInfo, ok := response["error"].(map[string]interface{})
		require.True(t, ok, "error should be a map")
		assert.Equal(t, -32603, errorInfo["code"])
		assert.Equal(t, "Test error", errorInfo["message"])
	})

	t.Run("CreateRecoveryResponse", func(t *testing.T) {
		originalErr := errors.New("build failed")
		recoverySteps := []string{"Check Dockerfile", "Verify dependencies"}
		alternatives := []string{"Use different base image", "Build locally"}

		result := transport.CreateRecoveryResponse(originalErr, recoverySteps, alternatives)

		response, ok := result.(map[string]interface{})
		require.True(t, ok)

		isError, ok := response["isError"].(bool)
		assert.True(t, ok, "isError should be a bool")
		assert.True(t, isError)
		recoveryAvailable, ok := response["recovery_available"].(bool)
		assert.True(t, ok, "recovery_available should be a bool")
		assert.True(t, recoveryAvailable)

		errorInfo, ok := response["error"].(map[string]interface{})
		require.True(t, ok, "error should be a map")
		assert.Equal(t, "build failed", errorInfo["message"])
		assert.Equal(t, recoverySteps, errorInfo["recovery_steps"])
		assert.Equal(t, alternatives, errorInfo["alternatives"])
	})
}

func TestStdioErrorHandler_EnhanceErrorWithContext(t *testing.T) {
	logger := zerolog.Nop()
	handler := NewStdioErrorHandler(logger)

	errorResponse := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      "test-id",
		"error": map[string]interface{}{
			"code":    -32603,
			"message": "Internal error",
		},
	}

	handler.EnhanceErrorWithContext(errorResponse, "session-123", "test_tool")

	errorInfo, ok := errorResponse["error"].(map[string]interface{})
	require.True(t, ok, "error field should be a map")
	assert.Equal(t, "session-123", errorInfo["session_id"])
	assert.Equal(t, "test_tool", errorInfo["tool"])
	assert.Equal(t, "stdio", errorInfo["transport"])
	assert.NotNil(t, errorInfo["timestamp"])

	debugInfo, ok := errorInfo["debug"].(map[string]interface{})
	require.True(t, ok, "debug field should be a map")
	assert.Equal(t, "stdio", debugInfo["transport_type"])
	assert.Equal(t, "stdio_error_handler", debugInfo["error_handler"])
	assert.Equal(t, "2024-11-05", debugInfo["mcp_version"])
}
