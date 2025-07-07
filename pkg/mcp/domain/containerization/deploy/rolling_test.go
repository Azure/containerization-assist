package deploy

import (
	"io"
	"strings"
	"testing"

	"log/slog"
)

// Test NewRollingUpdateStrategy constructor
func TestNewRollingUpdateStrategy(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	strategy := NewRollingUpdateStrategy(logger)

	if strategy == nil {
		t.Error("NewRollingUpdateStrategy should not return nil")
	}
	if strategy.BaseStrategy == nil {
		t.Error("BaseStrategy should not be nil")
	}
}

// Test RollingUpdateStrategy GetName
func TestRollingUpdateStrategyGetName(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	strategy := NewRollingUpdateStrategy(logger)

	name := strategy.GetName()
	expected := "rolling"

	if name != expected {
		t.Errorf("Expected GetName() to return '%s', got '%s'", expected, name)
	}
}

// Test RollingUpdateStrategy GetDescription
func TestRollingUpdateStrategyGetDescription(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	strategy := NewRollingUpdateStrategy(logger)

	description := strategy.GetDescription()
	expectedSubstring := "Rolling update deployment"

	if len(description) == 0 {
		t.Error("GetDescription() should not return empty string")
	}

	// Check if description contains expected content
	if !strings.Contains(description, expectedSubstring) {
		t.Errorf("Description should contain '%s', got '%s'", expectedSubstring, description)
	}
}
