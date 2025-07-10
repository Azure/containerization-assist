package errors

import (
	"errors"
	"testing"
)

func TestNewMissingParam(t *testing.T) {
	err := NewMissingParam("username")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	richErr, ok := err.(*RichError)
	if !ok {
		t.Fatal("expected RichError type")
	}

	if richErr.Code != CodeMissingParameter {
		t.Errorf("expected code %s, got %s", CodeMissingParameter, richErr.Code)
	}

	if richErr.Type != ErrTypeValidation {
		t.Errorf("expected type %s, got %s", ErrTypeValidation, richErr.Type)
	}
}

func TestNewValidationFailed(t *testing.T) {
	err := NewValidationFailed("email", "invalid format")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	richErr, ok := err.(*RichError)
	if !ok {
		t.Fatal("expected RichError type")
	}

	if richErr.Code != CodeValidationFailed {
		t.Errorf("expected code %s, got %s", CodeValidationFailed, richErr.Code)
	}

	if field, ok := richErr.Context["field"]; !ok || field != "email" {
		t.Error("expected field context")
	}

	if reason, ok := richErr.Context["reason"]; !ok || reason != "invalid format" {
		t.Error("expected reason context")
	}
}

func TestNewInternalError(t *testing.T) {
	cause := errors.New("database connection failed")
	err := NewInternalError("user_creation", cause)

	richErr, ok := err.(*RichError)
	if !ok {
		t.Fatal("expected RichError type")
	}

	if richErr.Code != CodeInternalError {
		t.Errorf("expected code %s, got %s", CodeInternalError, richErr.Code)
	}

	if richErr.Cause != cause {
		t.Error("expected cause to be wrapped")
	}
}

func TestNewConfigurationError(t *testing.T) {
	err := NewConfigurationError("database", "invalid connection string")

	richErr, ok := err.(*RichError)
	if !ok {
		t.Fatal("expected RichError type")
	}

	if richErr.Code != CodeConfigurationInvalid {
		t.Errorf("expected code %s, got %s", CodeConfigurationInvalid, richErr.Code)
	}
}

func TestNewNotFoundError(t *testing.T) {
	err := NewNotFoundError("user", "12345")

	richErr, ok := err.(*RichError)
	if !ok {
		t.Fatal("expected RichError type")
	}

	if richErr.Code != CodeNotFound {
		t.Errorf("expected code %s, got %s", CodeNotFound, richErr.Code)
	}
}

func TestNewMultiError(t *testing.T) {
	err1 := errors.New("error 1")
	err2 := errors.New("error 2")
	err3 := errors.New("error 3")

	// Test with no errors
	err := NewMultiError("operation", []error{})
	if err != nil {
		t.Error("expected nil for empty error slice")
	}

	// Test with single error
	err = NewMultiError("operation", []error{err1})
	if err != err1 {
		t.Error("expected single error to be returned directly")
	}

	// Test with multiple errors
	err = NewMultiError("operation", []error{err1, err2, err3})
	richErr, ok := err.(*RichError)
	if !ok {
		t.Fatal("expected RichError type")
	}

	if richErr.Code != CodeOperationFailed {
		t.Errorf("expected code %s, got %s", CodeOperationFailed, richErr.Code)
	}

	if count, ok := richErr.Context["error_count"]; !ok || count != 3 {
		t.Error("expected error_count context")
	}
}

func TestErrorHelperLocation(t *testing.T) {
	err := NewMissingParam("test_field")
	richErr, ok := err.(*RichError)
	if !ok {
		t.Fatal("expected RichError type")
	}

	if richErr.Location == nil {
		t.Error("expected location to be captured")
	}

	if richErr.Location != nil && richErr.Location.File == "" {
		t.Error("expected file location to be captured")
	}
}
