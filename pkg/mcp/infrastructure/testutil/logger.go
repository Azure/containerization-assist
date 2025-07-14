// Package testutil provides common testing utilities for the MCP codebase.
// It includes helpers for logging, mocking, fixtures, and assertions.
package testutil

import (
	"bytes"
	"log/slog"
	"testing"
)

// TestLogger represents a logger for testing that captures output
type TestLogger struct {
	*slog.Logger
	Buffer *bytes.Buffer
}

// NewTestLogger creates a new test logger that captures all output
func NewTestLogger(t *testing.T) *TestLogger {
	t.Helper()

	buffer := &bytes.Buffer{}
	handler := slog.NewTextHandler(buffer, &slog.HandlerOptions{
		Level: slog.LevelDebug, // Capture all levels in tests
	})

	logger := slog.New(handler).With("test", t.Name())

	return &TestLogger{
		Logger: logger,
		Buffer: buffer,
	}
}

// NewDiscardLogger creates a logger that discards all output
func NewDiscardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
}

// GetOutput returns the captured log output as a string
func (tl *TestLogger) GetOutput() string {
	return tl.Buffer.String()
}

// Contains checks if the log output contains a specific string
func (tl *TestLogger) Contains(substr string) bool {
	return bytes.Contains(tl.Buffer.Bytes(), []byte(substr))
}

// Reset clears the captured log output
func (tl *TestLogger) Reset() {
	tl.Buffer.Reset()
}

// AssertLogged checks if a message was logged and fails the test if not
func AssertLogged(t *testing.T, logger *TestLogger, message string) {
	t.Helper()

	if !logger.Contains(message) {
		t.Errorf("Expected log message not found: %q\nActual output:\n%s", message, logger.GetOutput())
	}
}

// AssertNotLogged checks if a message was NOT logged and fails the test if it was
func AssertNotLogged(t *testing.T, logger *TestLogger, message string) {
	t.Helper()

	if logger.Contains(message) {
		t.Errorf("Unexpected log message found: %q\nActual output:\n%s", message, logger.GetOutput())
	}
}
