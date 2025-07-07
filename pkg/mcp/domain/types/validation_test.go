package types

import (
	"testing"

	"github.com/Azure/container-kit/pkg/mcp/domain/validation"
)

// Test ValidationError type
func TestValidationError(t *testing.T) {
	// Test basic error
	err := &ValidationError{
		Code:     "TEST_ERROR",
		Message:  "This is a test error",
		Field:    "test_field",
		Severity: SeverityHigh,
		Context:  make(map[string]string),
	}

	// Test that Error() method works
	errorMsg := err.Error()
	if errorMsg == "" {
		t.Error("Error() method should return a non-empty string")
	}

	// Test field access
	if err.Code != "TEST_ERROR" {
		t.Errorf("Expected Code 'TEST_ERROR', got %s", err.Code)
	}
	if err.Message != "This is a test error" {
		t.Errorf("Expected Message 'This is a test error', got %s", err.Message)
	}
	if err.Severity != SeverityHigh {
		t.Errorf("Expected Severity %v, got %v", SeverityHigh, err.Severity)
	}
}

// Test ValidationWarning type
func TestValidationWarning(t *testing.T) {
	warning := &ValidationWarning{
		Code:    "TEST_WARNING",
		Message: "This is a test warning",
		Field:   "test_field",
		Context: make(map[string]string),
	}

	if warning.Code != "TEST_WARNING" {
		t.Errorf("Expected Code 'TEST_WARNING', got %s", warning.Code)
	}
	if warning.Message != "This is a test warning" {
		t.Errorf("Expected Message 'This is a test warning', got %s", warning.Message)
	}
}

// Test ValidationResult type
func TestValidationResult(t *testing.T) {
	result := NewValidationResult()

	if result == nil {
		t.Error("NewValidationResult() should return a non-nil result")
	}

	// Test that we can add errors and warnings
	if len(result.Errors) != 0 {
		t.Error("New validation result should have no errors initially")
	}
	if len(result.Warnings) != 0 {
		t.Error("New validation result should have no warnings initially")
	}
}

// Test Severity constants
func TestSeverityLevels(t *testing.T) {
	severities := []validation.ErrorSeverity{
		SeverityLow,
		SeverityMedium,
		SeverityHigh,
		SeverityCritical,
	}

	for _, severity := range severities {
		if string(severity) == "" {
			t.Errorf("Severity constant should not be empty: %v", severity)
		}
	}
}

// Test BuildValidationResult type alias
func TestBuildValidationResult(t *testing.T) {
	result := NewBuildResult()

	if result == nil {
		t.Error("NewBuildResult() should return a non-nil result")
	}

	// Test that BuildValidationResult is compatible with ValidationResult
	var _ *ValidationResult = result
}
