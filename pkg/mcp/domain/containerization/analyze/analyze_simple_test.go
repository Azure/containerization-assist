package analyze

import (
	"io"
	"testing"

	"log/slog"
)

func TestNewAnalyzer(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	analyzer := NewAnalyzer(logger)

	if analyzer == nil {
		t.Fatal("NewAnalyzer returned nil")
	}

	// Analyzer should have been created successfully
}
