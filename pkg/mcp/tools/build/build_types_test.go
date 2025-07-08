package build

import (
	"io"
	"testing"

	"log/slog"
)

// TestBuildTypesExist verifies that the split types are accessible
func TestBuildTypesExist(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	// Test that we can create instances of the split types
	analyzer := NewBuildAnalyzer(logger)
	if analyzer == nil {
		t.Fatal("NewBuildAnalyzer returned nil")
	}
	troubleshooter := NewBuildTroubleshooter(logger)
	if troubleshooter == nil {
		t.Fatal("NewBuildTroubleshooter returned nil")
	}
	scanner := NewBuildSecurityScanner(logger)
	if scanner == nil {
		t.Fatal("NewBuildSecurityScanner returned nil")
	}
	// Test that we can use them in BuildExecutorService
	executor := NewBuildExecutor(nil, nil, logger)
	if executor == nil {
		t.Fatal("NewBuildExecutor returned nil")
	}
	if executor.analyzer == nil {
		t.Fatal("BuildExecutor analyzer is nil")
	}
	if executor.troubleshooter == nil {
		t.Fatal("BuildExecutor troubleshooter is nil")
	}
	if executor.securityScanner == nil {
		t.Fatal("BuildExecutor securityScanner is nil")
	}
}
