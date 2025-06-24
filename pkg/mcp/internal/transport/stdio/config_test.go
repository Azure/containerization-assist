package stdio

import (
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestNewDefaultConfig(t *testing.T) {
	logger := zerolog.New(os.Stderr)
	config := NewDefaultConfig(logger)

	assert.Equal(t, "stdio_transport", config.Component)
	assert.True(t, config.EnableErrorHandler)
	assert.Equal(t, "info", config.LogLevel)
	assert.Equal(t, 0, config.BufferSize)
}

func TestNewConfigWithComponent(t *testing.T) {
	logger := zerolog.New(os.Stderr)
	config := NewConfigWithComponent(logger, "test_component")

	assert.Equal(t, "test_component", config.Component)
	assert.True(t, config.EnableErrorHandler)
}

func TestConfigValidate(t *testing.T) {
	logger := zerolog.New(os.Stderr)

	t.Run("valid config", func(t *testing.T) {
		config := NewDefaultConfig(logger)
		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("empty component", func(t *testing.T) {
		config := NewDefaultConfig(logger)
		config.Component = ""
		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "component name is required")
	})

	t.Run("negative buffer size", func(t *testing.T) {
		config := NewDefaultConfig(logger)
		config.BufferSize = -1
		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "buffer size cannot be negative")
	})
}

func TestCreateLogger(t *testing.T) {
	baseLogger := zerolog.New(os.Stderr).With().Str("test", "value").Logger()
	config := NewConfigWithComponent(baseLogger, "test_component")

	logger := config.CreateLogger()

	// Logger should have the transport and component context
	// We can't easily inspect the logger internals, but we can verify it doesn't panic
	logger.Info().Msg("Test message")
}

func TestCreateDefaultLogger(t *testing.T) {
	logger := CreateDefaultLogger("test_component")

	// Should not panic and should be usable
	logger.Info().Msg("Test message")
}
