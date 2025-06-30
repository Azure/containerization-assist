package deploy

import (
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

// Test NewRecreateStrategy constructor
func TestNewRecreateStrategy(t *testing.T) {
	logger := zerolog.Nop()

	strategy := NewRecreateStrategy(logger)

	if strategy == nil {
		t.Error("NewRecreateStrategy should not return nil")
	}
	if strategy.BaseStrategy == nil {
		t.Error("BaseStrategy should not be nil")
	}
}

// Test RecreateStrategy GetName
func TestRecreateStrategyGetName(t *testing.T) {
	logger := zerolog.Nop()
	strategy := NewRecreateStrategy(logger)

	name := strategy.GetName()
	expected := "recreate"

	if name != expected {
		t.Errorf("Expected GetName() to return '%s', got '%s'", expected, name)
	}
}

// Test RecreateStrategy GetDescription
func TestRecreateStrategyGetDescription(t *testing.T) {
	logger := zerolog.Nop()
	strategy := NewRecreateStrategy(logger)

	description := strategy.GetDescription()
	expectedSubstring := "Recreate deployment"

	if len(description) == 0 {
		t.Error("GetDescription() should not return empty string")
	}

	// Check if description contains expected content
	if !strings.Contains(description, expectedSubstring) {
		t.Errorf("Description should contain '%s', got '%s'", expectedSubstring, description)
	}
}
