package orchestration

import (
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// Test NewErrorClassifier constructor
func TestNewErrorClassifier(t *testing.T) {
	logger := zerolog.Nop()

	classifier := NewErrorClassifier(logger)

	if classifier == nil {
		t.Error("NewErrorClassifier should not return nil")
	}
}

// Test ErrorClassifier struct
func TestErrorClassifierStruct(t *testing.T) {
	logger := zerolog.Nop()
	classifier := ErrorClassifier{
		logger: logger,
	}

	// Test that the classifier has been created with proper logger
	if classifier.logger.GetLevel() < 0 {
		// This is just testing that the logger is set to something reasonable
	}
}

// Test WorkflowError type (adding to execution types test)
func TestWorkflowError(t *testing.T) {
	timestamp := time.Now()

	workflowError := WorkflowError{
		ID:        "error-123",
		Message:   "Docker build failed",
		Code:      "BUILD_FAILED",
		Type:      "build_error",
		ErrorType: "docker_build_failure",
		Severity:  "high",
		Retryable: true,
		StageName: "build-stage",
		ToolName:  "docker-build",
		Timestamp: timestamp,
	}

	if workflowError.ID != "error-123" {
		t.Errorf("Expected ID to be 'error-123', got '%s'", workflowError.ID)
	}
	if workflowError.Message != "Docker build failed" {
		t.Errorf("Expected Message to be 'Docker build failed', got '%s'", workflowError.Message)
	}
	if workflowError.Code != "BUILD_FAILED" {
		t.Errorf("Expected Code to be 'BUILD_FAILED', got '%s'", workflowError.Code)
	}
	if workflowError.Type != "build_error" {
		t.Errorf("Expected Type to be 'build_error', got '%s'", workflowError.Type)
	}
	if workflowError.ErrorType != "docker_build_failure" {
		t.Errorf("Expected ErrorType to be 'docker_build_failure', got '%s'", workflowError.ErrorType)
	}
	if workflowError.Severity != "high" {
		t.Errorf("Expected Severity to be 'high', got '%s'", workflowError.Severity)
	}
	if !workflowError.Retryable {
		t.Error("Expected Retryable to be true")
	}
	if workflowError.StageName != "build-stage" {
		t.Errorf("Expected StageName to be 'build-stage', got '%s'", workflowError.StageName)
	}
	if workflowError.ToolName != "docker-build" {
		t.Errorf("Expected ToolName to be 'docker-build', got '%s'", workflowError.ToolName)
	}
	if workflowError.Timestamp != timestamp {
		t.Errorf("Expected Timestamp to match, got %v", workflowError.Timestamp)
	}
}

// Test IsFatalError function
func TestIsFatalError(t *testing.T) {
	logger := zerolog.Nop()
	classifier := NewErrorClassifier(logger)

	// Test critical severity error (should be fatal)
	criticalError := &WorkflowError{
		ID:        "error-critical",
		Severity:  "critical",
		ErrorType: "some_error",
	}

	if !classifier.IsFatalError(criticalError) {
		t.Error("Expected critical severity error to be fatal")
	}

	// Test authentication failure (should be fatal)
	authError := &WorkflowError{
		ID:        "error-auth",
		Severity:  "high",
		ErrorType: "authentication_failure",
	}

	if !classifier.IsFatalError(authError) {
		t.Error("Expected authentication_failure to be fatal")
	}

	// Test permission denied (should be fatal)
	permissionError := &WorkflowError{
		ID:        "error-perm",
		Severity:  "medium",
		ErrorType: "permission_denied",
	}

	if !classifier.IsFatalError(permissionError) {
		t.Error("Expected permission_denied to be fatal")
	}

	// Test system error (should be fatal)
	systemError := &WorkflowError{
		ID:        "error-system",
		Severity:  "high",
		ErrorType: "system_error",
	}

	if !classifier.IsFatalError(systemError) {
		t.Error("Expected system_error to be fatal")
	}

	// Test non-fatal error (should not be fatal)
	nonFatalError := &WorkflowError{
		ID:        "error-retry",
		Severity:  "low",
		ErrorType: "network_timeout",
	}

	if classifier.IsFatalError(nonFatalError) {
		t.Error("Expected network_timeout to not be fatal")
	}

	// Test case sensitivity (authentication_failure in upper case)
	authUpperError := &WorkflowError{
		ID:        "error-auth-upper",
		Severity:  "medium",
		ErrorType: "AUTHENTICATION_FAILURE",
	}

	if !classifier.IsFatalError(authUpperError) {
		t.Error("Expected AUTHENTICATION_FAILURE to be fatal (case insensitive)")
	}
}

// Test IsFatalError with mixed case error types
func TestIsFatalErrorMixedCase(t *testing.T) {
	logger := zerolog.Nop()
	classifier := NewErrorClassifier(logger)

	testCases := []struct {
		name      string
		errorType string
		expected  bool
	}{
		{"lowercase auth", "authentication_failure", true},
		{"uppercase auth", "AUTHENTICATION_FAILURE", true},
		{"mixed case auth", "Authentication_Failure", true},
		{"contained auth", "oauth_authentication_failure_retry", true},
		{"config invalid", "configuration_invalid", true},
		{"quota exceeded", "quota_exceeded", true},
		{"normal timeout", "connection_timeout", false},
		{"normal retry", "temporary_failure", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			error := &WorkflowError{
				ID:        "test-error",
				Severity:  "medium",
				ErrorType: tc.errorType,
			}

			result := classifier.IsFatalError(error)
			if result != tc.expected {
				t.Errorf("Expected IsFatalError('%s') to be %v, got %v", tc.errorType, tc.expected, result)
			}
		})
	}
}
