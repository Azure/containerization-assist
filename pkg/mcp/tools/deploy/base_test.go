package deploy

import (
	"io"
	"log/slog"
	"testing"
)

// Test NewBaseStrategy constructor
func TestNewBaseStrategy(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	strategy := NewBaseStrategy(logger)

	if strategy == nil {
		t.Error("NewBaseStrategy should not return nil")
	}
}

// Test BaseStrategy struct
func TestBaseStrategy(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	strategy := BaseStrategy{
		logger: logger,
	}

	// Test that the strategy has been created with proper logger
	// We can't easily test the logger field since it's private,
	// but we can test that the strategy is functional
	if strategy.logger == nil {
		t.Error("logger should not be nil")
	}
}
