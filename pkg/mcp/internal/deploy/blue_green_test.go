package deploy

import (
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

// Test NewBlueGreenStrategy constructor
func TestNewBlueGreenStrategy(t *testing.T) {
	logger := zerolog.Nop()

	strategy := NewBlueGreenStrategy(logger)

	if strategy == nil {
		t.Error("NewBlueGreenStrategy should not return nil")
	}
	if strategy.BaseStrategy == nil {
		t.Error("BaseStrategy should not be nil")
	}
}

// Test BlueGreenStrategy GetName
func TestBlueGreenStrategyGetName(t *testing.T) {
	logger := zerolog.Nop()
	strategy := NewBlueGreenStrategy(logger)

	name := strategy.GetName()
	expected := "blue_green"

	if name != expected {
		t.Errorf("Expected GetName() to return '%s', got '%s'", expected, name)
	}
}

// Test BlueGreenStrategy GetDescription
func TestBlueGreenStrategyGetDescription(t *testing.T) {
	logger := zerolog.Nop()
	strategy := NewBlueGreenStrategy(logger)

	description := strategy.GetDescription()
	expectedSubstring := "Blue-green deployment"

	if len(description) == 0 {
		t.Error("GetDescription() should not return empty string")
	}

	// Check if description contains expected content
	if !strings.Contains(description, expectedSubstring) {
		t.Errorf("Description should contain '%s', got '%s'", expectedSubstring, description)
	}
}
