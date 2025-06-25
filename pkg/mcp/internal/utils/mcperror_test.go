package utils

import (
	"errors"
	"strings"
	"testing"

	v20250326 "github.com/localrivet/gomcp/mcp/v20250326"
)

func TestNew(t *testing.T) {
	code := v20250326.ErrorCodeInternalServerError
	message := "test error message"

	err := New(code, message)

	if err.Code != code {
		t.Errorf("Expected code %s, got %s", code, err.Code)
	}

	if err.Message != message {
		t.Errorf("Expected message %s, got %s", message, err.Message)
	}

	if err.Error() != message {
		t.Errorf("Expected Error() to return %s, got %s", message, err.Error())
	}
}

func TestNewWithData(t *testing.T) {
	code := v20250326.ErrorCodeInvalidArguments
	message := "invalid field"
	data := map[string]interface{}{"field": "image_name"}

	err := NewWithData(code, message, data)

	if err.Code != code {
		t.Errorf("Expected code %s, got %s", code, err.Code)
	}

	if err.Data == nil {
		t.Error("Expected data to be set")
	}
}

func TestWrap(t *testing.T) {
	originalErr := errors.New("original error")
	code := v20250326.ErrorCodeInternalServerError
	message := "wrapped error"

	err := Wrap(code, message, originalErr)

	if err.Code != code {
		t.Errorf("Expected code %s, got %s", code, err.Code)
	}

	expectedMessage := "wrapped error: original error"
	if err.Message != expectedMessage {
		t.Errorf("Expected message %s, got %s", expectedMessage, err.Message)
	}

	if err.Data == nil {
		t.Error("Expected data to be set with original error")
	}
}

func TestNewSessionNotFound(t *testing.T) {
	sessionID := "test-session-123"

	err := NewSessionNotFound(sessionID)

	if err.Code != CodeSessionNotFound {
		t.Errorf("Expected code %s, got %s", CodeSessionNotFound, err.Code)
	}

	if err.Message != "session not found" {
		t.Errorf("Expected message 'session not found', got %s", err.Message)
	}

	data, ok := err.Data.(map[string]interface{})
	if !ok {
		t.Error("Expected data to be a map")
	}

	if data["session_id"] != sessionID {
		t.Errorf("Expected session_id %s, got %v", sessionID, data["session_id"])
	}
}

func TestNewBuildFailed(t *testing.T) {
	message := "docker build failed due to missing dependency"

	err := NewBuildFailed(message)

	if err.Code != CodeBuildFailed {
		t.Errorf("Expected code %s, got %s", CodeBuildFailed, err.Code)
	}

	expectedMessage := "docker build failed: " + message
	if err.Message != expectedMessage {
		t.Errorf("Expected message %s, got %s", expectedMessage, err.Message)
	}
}

func TestFromError(t *testing.T) {
	tests := []struct {
		name          string
		input         error
		expectedCode  v20250326.ErrorCode
		expectedInMsg string
	}{
		{
			name:          "build failed error",
			input:         errors.New("build failed: missing dependency"),
			expectedCode:  v20250326.ErrorCodeInternalServerError,
			expectedInMsg: "docker build failed",
		},
		{
			name:          "session not found error",
			input:         errors.New("session abc123 not found"),
			expectedCode:  v20250326.ErrorCodeInvalidRequest,
			expectedInMsg: "session not found",
		},
		{
			name:          "dockerfile invalid error",
			input:         errors.New("dockerfile invalid syntax"),
			expectedCode:  v20250326.ErrorCodeInvalidArguments,
			expectedInMsg: "dockerfile invalid",
		},
		{
			name:          "permission denied error",
			input:         errors.New("permission denied for operation"),
			expectedCode:  v20250326.ErrorCodeInvalidRequest,
			expectedInMsg: "permission denied",
		},
		{
			name:          "generic error",
			input:         errors.New("some random error"),
			expectedCode:  v20250326.ErrorCodeInternalServerError,
			expectedInMsg: "some random error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := FromError(tt.input)

			if result.Code != tt.expectedCode {
				t.Errorf("Expected code %s, got %s", tt.expectedCode, result.Code)
			}

			if !strings.Contains(result.Message, tt.expectedInMsg) {
				t.Errorf("Expected message to contain %s, got %s", tt.expectedInMsg, result.Message)
			}
		})
	}
}

func TestIsSessionError(t *testing.T) {
	sessionErr := NewSessionNotFound("test")
	buildErr := NewBuildFailed("test")
	regularErr := errors.New("regular error")

	if !IsSessionError(sessionErr) {
		t.Error("Expected IsSessionError to return true for session error")
	}

	// Build errors are not session errors since they use different MCP codes
	if IsSessionError(buildErr) {
		t.Error("Expected IsSessionError to return false for build error")
	}

	if IsSessionError(regularErr) {
		t.Error("Expected IsSessionError to return false for regular error")
	}
}

func TestGetErrorCategory(t *testing.T) {
	category, ok := GetErrorCategory(v20250326.ErrorCodeInvalidArguments)
	if !ok {
		t.Error("Expected to find error category for invalid arguments")
	}

	if category.Name != "Invalid Arguments" {
		t.Errorf("Expected category name 'Invalid Arguments', got %s", category.Name)
	}

	if category.Retryable != false {
		t.Error("Expected invalid arguments to not be retryable")
	}
}

func TestGetUserFriendlyMessage(t *testing.T) {
	err := New(v20250326.ErrorCodeInvalidArguments, "field validation failed")

	message := GetUserFriendlyMessage(err)
	expected := "Invalid arguments provided. Please check the input parameters."

	if message != expected {
		t.Errorf("Expected message %s, got %s", expected, message)
	}
}

func TestShouldRetry(t *testing.T) {
	retryableErr := New(v20250326.ErrorCodeInternalServerError, "server error")
	nonRetryableErr := New(v20250326.ErrorCodeInvalidArguments, "invalid args")

	if !ShouldRetry(retryableErr) {
		t.Error("Expected internal server error to be retryable")
	}

	if ShouldRetry(nonRetryableErr) {
		t.Error("Expected invalid arguments error to not be retryable")
	}
}
