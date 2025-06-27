package analyze

import (
	"testing"

	"github.com/rs/zerolog"
)

func TestNewAnalyzer(t *testing.T) {
	logger := zerolog.Nop()
	analyzer := NewAnalyzer(logger)

	if analyzer == nil {
		t.Fatal("NewAnalyzer returned nil")
	}

	// Analyzer should have been created successfully
}
