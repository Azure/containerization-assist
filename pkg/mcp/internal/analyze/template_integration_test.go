package analyze

import (
	"testing"

	"github.com/rs/zerolog"
)

// Test TemplateIntegration constructor
func TestNewTemplateIntegration(t *testing.T) {
	logger := zerolog.Nop()

	// Test constructor
	integration := NewTemplateIntegration(logger)
	if integration == nil {
		t.Error("NewTemplateIntegration should not return nil")
	}
}
