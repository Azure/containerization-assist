package errors

import (
	"testing"
)

// TestRichErrorStructure validates that RichError has proper structure
func TestRichErrorStructure(t *testing.T) {
	// Test that RichError can be built with all required fields
	err := NewError().
		Code(CodeInvalidParameter).
		Type(ErrTypeValidation).
		Severity(SeverityMedium).
		Message("test error").
		Context("key", "value").
		Suggestion("test suggestion").
		WithLocation().
		Build()

	if err == nil {
		t.Fatal("Expected non-nil error from RichError builder")
	}

	// Validate error implements standard error interface
	var _ error = err

	// Validate error message is not empty
	if err.Error() == "" {
		t.Error("RichError.Error() returned empty string")
	}

	// Validate error has proper fields
	if err.Code != CodeInvalidParameter {
		t.Errorf("Expected code %s, got %s", CodeInvalidParameter, err.Code)
	}

	if err.Type != ErrTypeValidation {
		t.Errorf("Expected type %s, got %s", ErrTypeValidation, err.Type)
	}

	if err.Severity != SeverityMedium {
		t.Errorf("Expected severity %s, got %s", SeverityMedium, err.Severity)
	}

	if err.Message != "test error" {
		t.Errorf("Expected message 'test error', got '%s'", err.Message)
	}

	if len(err.Context) == 0 {
		t.Error("Expected non-empty context")
	}

	if err.Location == nil {
		t.Error("Expected non-nil location")
	}

	if len(err.Suggestions) == 0 {
		t.Error("Expected non-empty suggestions")
	}
}

// TestMissingParameterError validates the MissingParameterError constructor
func TestMissingParameterError(t *testing.T) {
	err := MissingParameterError("test_param")

	if err == nil {
		t.Fatal("Expected non-nil error")
	}

	if err.Code != CodeMissingParameter {
		t.Errorf("Expected code %s, got %s", CodeMissingParameter, err.Code)
	}

	if err.Type != ErrTypeValidation {
		t.Errorf("Expected type %s, got %s", ErrTypeValidation, err.Type)
	}

	if err.Severity != SeverityMedium {
		t.Errorf("Expected severity %s, got %s", SeverityMedium, err.Severity)
	}

	if err.Location == nil {
		t.Error("Expected non-nil location from constructor")
	}

	// Check that parameter name is in context
	if paramValue, ok := err.Context["parameter"]; !ok || paramValue != "test_param" {
		t.Errorf("Expected parameter 'test_param' in context, got %v", paramValue)
	}
}

// TestErrorCodes validates that common error codes are defined
func TestErrorCodes(t *testing.T) {
	expectedCodes := []ErrorCode{
		CodeInvalidParameter,
		CodeMissingParameter,
		CodeToolNotFound,
		CodeToolAlreadyRegistered,
		CodeValidationFailed,
		CodeResourceNotFound,
		CodePermissionDenied,
		CodeInternalError,
	}

	for _, code := range expectedCodes {
		if string(code) == "" {
			t.Errorf("Error code %v is empty", code)
		}

		// Test that each code can be used in error creation
		err := NewError().Code(code).Message("test").Build()
		if err == nil {
			t.Errorf("Failed to create error with code %s", code)
		}
	}
}

// TestErrorTypes validates that common error types are defined
func TestErrorTypes(t *testing.T) {
	expectedTypes := []ErrorType{
		ErrTypeValidation,
		ErrTypeTool,
		ErrTypeConfiguration,
		ErrTypeNetwork,
		ErrTypeIO,
		ErrTypeInternal,
		ErrTypeKubernetes,
		ErrTypeContainer,
	}

	for _, errType := range expectedTypes {
		if string(errType) == "" {
			t.Errorf("Error type %v is empty", errType)
		}

		// Test that each type can be used in error creation
		err := NewError().Type(errType).Message("test").Build()
		if err == nil {
			t.Errorf("Failed to create error with type %s", errType)
		}
	}
}

// TestSeverityLevels validates that all severity levels are defined
func TestSeverityLevels(t *testing.T) {
	expectedSeverities := []ErrorSeverity{
		SeverityLow,
		SeverityMedium,
		SeverityHigh,
		SeverityCritical,
	}

	for _, severity := range expectedSeverities {
		if string(severity) == "" {
			t.Errorf("Severity level %v is empty", severity)
		}

		// Test that each severity can be used in error creation
		err := NewError().Severity(severity).Message("test").Build()
		if err == nil {
			t.Errorf("Failed to create error with severity %s", severity)
		}
	}
}

// TestErrorSerialization validates that RichError can be serialized
func TestErrorSerialization(t *testing.T) {
	originalErr := NewError().
		Code(CodeValidationFailed).
		Type(ErrTypeValidation).
		Severity(SeverityHigh).
		Message("validation failed").
		Context("field", "username").
		Context("value", "invalid@").
		Suggestion("Use a valid username format").
		WithLocation().
		Build()

	// Test JSON marshaling
	data, err := originalErr.MarshalJSON()
	if err != nil {
		t.Fatalf("Failed to marshal error to JSON: %v", err)
	}

	if len(data) == 0 {
		t.Error("Expected non-empty JSON data")
	}

	// Basic validation that JSON contains expected content
	jsonStr := string(data)
	if jsonStr == "{}" {
		t.Error("JSON serialization appears to be empty object")
	}
}
