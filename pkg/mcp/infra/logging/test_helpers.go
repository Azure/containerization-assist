package logging

import "os"

// NewTestLogger creates a logger with default test configuration
func NewTestLogger() Standards {
	config := Config{
		Level:                   LevelDebug,
		Output:                  os.Stdout,
		EnableStructuredLogging: true,
		EnableRingBuffer:        true,
		BufferSize:              100,
		EnableCaller:            true,
	}
	return NewLogger(config)
}
