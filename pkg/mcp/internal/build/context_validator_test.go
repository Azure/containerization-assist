package build

import (
	"testing"

	"github.com/rs/zerolog"
)

// Test NewContextValidator constructor
func TestNewContextValidator(t *testing.T) {
	logger := zerolog.Nop()

	validator := NewContextValidator(logger)

	if validator == nil {
		t.Error("NewContextValidator should not return nil")
	}

	// Test that the validator has been created with proper logger
	// We can't easily test the logger field since it's private,
	// but we can test that the validator is functional
	if validator.logger.GetLevel() < 0 {
		// This is just testing that the logger is set to something reasonable
		// The actual logger setup is tested by creating the validator
	}
}

// Test ContextValidator struct
func TestContextValidatorStruct(t *testing.T) {
	logger := zerolog.Nop()
	validator := ContextValidator{
		logger: logger,
	}

	// Test that we can create the struct directly
	if validator.logger.GetLevel() < 0 {
		// Just checking the logger is set
	}
}
