package analyze

import (
	"io"
	"log/slog"
	"testing"
)

// Test TemplateIntegration constructor
func TestNewTemplateIntegration(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Test constructor
	integration := NewTemplateIntegration(logger)
	if integration == nil {
		t.Error("NewTemplateIntegration should not return nil")
	}
}
