package transport

import (
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDefaultConfig(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	config := NewDefaultConfig(logger)

	assert.Equal(t, "stdio_transport", config.Component)
	assert.True(t, config.EnableErrorHandler)
	assert.Equal(t, "info", config.LogLevel)
	assert.Equal(t, 0, config.BufferSize)
}

func TestNewConfigWithComponent(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	config := NewConfigWithComponent(logger, "test_component")

	assert.Equal(t, "test_component", config.Component)
	assert.True(t, config.EnableErrorHandler)
}

func TestConfigValidate(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	t.Run("valid config", func(t *testing.T) {
		t.Parallel()
		config := NewDefaultConfig(logger)
		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("empty component", func(t *testing.T) {
		t.Parallel()
		config := NewDefaultConfig(logger)
		config.Component = ""
		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "component name is required")
	})

	t.Run("negative buffer size", func(t *testing.T) {
		t.Parallel()
		config := NewDefaultConfig(logger)
		config.BufferSize = -1
		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "buffer size cannot be negative")
	})
}

func TestCreateLogger(t *testing.T) {
	t.Parallel()
	baseLogger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	config := NewConfigWithComponent(baseLogger, "test_component")

	logger := config.CreateLogger()

	// Logger should have the transport and component context
	// We can't easily inspect the logger internals, but we can verify it doesn't panic
	logger.Info("Test message")
}

func TestCreateDefaultLogger(t *testing.T) {
	t.Parallel()
	logger := CreateDefaultLogger("test_component")

	// Should not panic and should be usable
	logger.Info("Test message")
}
