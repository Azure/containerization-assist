package types

import (
	"fmt"
	"testing"

	commonUtils "github.com/Azure/container-kit/pkg/commonutils"
)

// Test ToolError type
func TestToolError(t *testing.T) {
	// Test basic error
	err := &ToolError{
		Code:     "TEST_ERROR",
		Message:  "This is a test error",
		Type:     ErrTypeValidation,
		Severity: SeverityHigh,
	}

	if err.Error() != "TEST_ERROR: This is a test error" {
		t.Errorf("Expected 'TEST_ERROR: This is a test error', got %s", err.Error())
	}

	// Test error with cause
	causeErr := fmt.Errorf("original error")
	errWithCause := &ToolError{
		Code:     "WRAPPED_ERROR",
		Message:  "Wrapped error",
		Type:     ErrTypeSystem,
		Severity: SeverityCritical,
		Cause:    causeErr,
	}

	expectedMsg := "WRAPPED_ERROR: Wrapped error (caused by: original error)"
	if errWithCause.Error() != expectedMsg {
		t.Errorf("Expected %s, got %s", expectedMsg, errWithCause.Error())
	}
}

// Test ErrorType constants
func TestErrorTypes(t *testing.T) {
	types := []ErrorType{
		ErrTypeValidation,
		ErrTypeNotFound,
		ErrTypeSystem,
		ErrTypeBuild,
		ErrTypeDeployment,
		ErrTypeSecurity,
		ErrTypeConfig,
		ErrTypeNetwork,
		ErrTypePermission,
	}

	for _, errType := range types {
		if string(errType) == "" {
			t.Errorf("Error type %v should not be empty", errType)
		}
	}

	// Test specific values
	if ErrTypeValidation != "validation" {
		t.Errorf("Expected ErrTypeValidation to be 'validation', got %s", ErrTypeValidation)
	}
	if ErrTypeBuild != "build" {
		t.Errorf("Expected ErrTypeBuild to be 'build', got %s", ErrTypeBuild)
	}
}

// Test ErrorSeverity constants
func TestErrorSeverities(t *testing.T) {
	severities := []ErrorSeverity{
		SeverityCritical,
		SeverityHigh,
		SeverityMedium,
		SeverityLow,
	}

	for _, severity := range severities {
		if string(severity) == "" {
			t.Errorf("Severity %v should not be empty", severity)
		}
	}

	// Test specific values
	if SeverityCritical != "critical" {
		t.Errorf("Expected SeverityCritical to be 'critical', got %s", SeverityCritical)
	}
	if SeverityLow != "low" {
		t.Errorf("Expected SeverityLow to be 'low', got %s", SeverityLow)
	}
}

// Test ValidationErrorSet
func TestValidationErrorSet(t *testing.T) {
	errorSet := NewValidationErrorSet()

	// Test empty set
	if errorSet.HasErrors() {
		t.Error("New error set should not have errors")
	}
	if errorSet.Error() != "" {
		t.Error("Empty error set should return empty string")
	}

	// Add a field error
	errorSet.AddField("username", "cannot be empty")

	if !errorSet.HasErrors() {
		t.Error("Error set should have errors after adding one")
	}

	errors := errorSet.Errors()
	if len(errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(errors))
	}

	expectedMsg := "Field 'username': cannot be empty"
	if !commonUtils.Contains(errorSet.Error(), expectedMsg) {
		t.Errorf("Error message should contain '%s', got '%s'", expectedMsg, errorSet.Error())
	}

	// Add another error
	customError := &ToolError{
		Code:     "CUSTOM_ERROR",
		Message:  "Custom validation error",
		Type:     ErrTypeValidation,
		Severity: SeverityMedium,
	}
	errorSet.Add(customError)

	if len(errorSet.Errors()) != 2 {
		t.Errorf("Expected 2 errors, got %d", len(errorSet.Errors()))
	}

	// Test error string contains both messages
	errorStr := errorSet.Error()
	if !commonUtils.Contains(errorStr, "username") || !commonUtils.Contains(errorStr, "CUSTOM_ERROR") {
		t.Errorf("Error string should contain both errors: %s", errorStr)
	}
}

// Test NewValidationError
func TestNewValidationError(t *testing.T) {
	err := NewValidationError("email", "invalid format")

	if err.Code != "VALIDATION_ERROR" {
		t.Errorf("Expected Code to be 'VALIDATION_ERROR', got %s", err.Code)
	}
	if err.Type != ErrTypeValidation {
		t.Errorf("Expected Type to be ErrTypeValidation, got %s", err.Type)
	}
	if err.Severity != SeverityMedium {
		t.Errorf("Expected Severity to be SeverityMedium, got %s", err.Severity)
	}

	expectedMsg := "Field 'email': invalid format"
	if !commonUtils.Contains(err.Message, expectedMsg) {
		t.Errorf("Expected message to contain '%s', got '%s'", expectedMsg, err.Message)
	}

	// Check context
	if err.Context.Fields == nil {
		t.Error("Expected Context.Fields to be non-nil")
	}
	if field, ok := err.Context.Fields["field"]; !ok || field != "email" {
		t.Errorf("Expected Context.Fields['field'] to be 'email', got %v", field)
	}
}

// Test ValidationOptions
func TestValidationOptions(t *testing.T) {
	options := ValidationOptions{
		StrictMode: true,
		MaxErrors:  10,
		SkipFields: []string{"password", "token"},
	}

	if !options.StrictMode {
		t.Error("Expected StrictMode to be true")
	}
	if options.MaxErrors != 10 {
		t.Errorf("Expected MaxErrors to be 10, got %d", options.MaxErrors)
	}
	if len(options.SkipFields) != 2 {
		t.Errorf("Expected 2 skip fields, got %d", len(options.SkipFields))
	}
}

// Test ValidationResult
func TestValidationResult(t *testing.T) {
	errors := []*ToolError{
		NewValidationError("field1", "error1"),
		NewValidationError("field2", "error2"),
	}
	warnings := []*ToolError{
		{Code: "WARN1", Message: "Warning 1", Type: ErrTypeValidation, Severity: SeverityLow},
	}

	result := ValidationResult{
		Valid:    false,
		Errors:   errors,
		Warnings: warnings,
		Metadata: ValidationMetadata{
			ValidatedAt: "2023-01-01T00:00:00Z",
			Duration:    "100ms",
			Rules:       []string{"required", "format"},
			Version:     "1.0.0",
		},
	}

	if result.Valid {
		t.Error("Expected Valid to be false")
	}
	if len(result.Errors) != 2 {
		t.Errorf("Expected 2 errors, got %d", len(result.Errors))
	}
	if len(result.Warnings) != 1 {
		t.Errorf("Expected 1 warning, got %d", len(result.Warnings))
	}
	if result.Metadata.Version != "1.0.0" {
		t.Errorf("Expected Version to be '1.0.0', got %s", result.Metadata.Version)
	}
}

// Test ErrorContext
func TestErrorContext(t *testing.T) {
	context := ErrorContext{
		Tool:      "test-tool",
		Operation: "validate",
		Stage:     "preprocessing",
		SessionID: "session-123",
		Fields: map[string]interface{}{
			"field1": "value1",
			"field2": 123,
		},
	}

	if context.Tool != "test-tool" {
		t.Errorf("Expected Tool to be 'test-tool', got %s", context.Tool)
	}
	if context.SessionID != "session-123" {
		t.Errorf("Expected SessionID to be 'session-123', got %s", context.SessionID)
	}
	if len(context.Fields) != 2 {
		t.Errorf("Expected 2 fields, got %d", len(context.Fields))
	}
	if context.Fields["field1"] != "value1" {
		t.Errorf("Expected field1 to be 'value1', got %v", context.Fields["field1"])
	}
}
