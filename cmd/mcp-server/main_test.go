package main

import (
	"strings"
	"testing"
)

func TestGetVersion(t *testing.T) {
	// Test default values
	version := getVersion()
	if !strings.Contains(version, "dev") {
		t.Errorf("Expected version to contain 'dev', got: %s", version)
	}

	// Test with set values
	Version = "1.0.0"
	GitCommit = "abc123"
	BuildTime = "2024-01-01T00:00:00Z"

	version = getVersion()
	expected := "v1.0.0 (commit: abc123, built: 2024-01-01T00:00:00Z)"
	if version != expected {
		t.Errorf("Expected version '%s', got: %s", expected, version)
	}

	// Reset for other tests
	Version = "dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
}
