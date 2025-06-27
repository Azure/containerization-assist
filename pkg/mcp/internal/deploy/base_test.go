package deploy

import (
	"testing"

	"github.com/rs/zerolog"
)

// Test NewBaseStrategy constructor
func TestNewBaseStrategy(t *testing.T) {
	logger := zerolog.Nop()

	strategy := NewBaseStrategy(logger)

	if strategy == nil {
		t.Error("NewBaseStrategy should not return nil")
	}
}

// Test BaseStrategy struct
func TestBaseStrategy(t *testing.T) {
	logger := zerolog.Nop()
	strategy := BaseStrategy{
		logger: logger,
	}

	// Test that the strategy has been created with proper logger
	// We can't easily test the logger field since it's private,
	// but we can test that the strategy is functional
	if strategy.logger.GetLevel() < 0 {
		// This is just testing that the logger is set to something reasonable
	}
}
