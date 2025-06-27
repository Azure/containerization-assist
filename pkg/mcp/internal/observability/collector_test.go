package observability

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
)

func TestNewCollector(t *testing.T) {
	logger := zerolog.Nop()
	collector := NewCollector(logger)

	if collector == nil {
		t.Fatal("NewCollector returned nil")
	}
}

func TestCollector_CollectSystemState(t *testing.T) {
	logger := zerolog.Nop()
	collector := NewCollector(logger)
	ctx := context.Background()

	state := collector.CollectSystemState(ctx)

	// State should have basic fields populated
	if state.DiskSpaceMB < 0 {
		t.Error("Disk space should not be negative")
	}
}

func TestCollector_CollectResourceUsage(t *testing.T) {
	logger := zerolog.Nop()
	collector := NewCollector(logger)

	usage := collector.CollectResourceUsage()

	// Usage should have basic memory info
	if usage.MemoryMB < 0 {
		t.Error("Memory usage should not be negative")
	}
}

func TestCollector_CollectBuildDiagnostics(t *testing.T) {
	logger := zerolog.Nop()
	collector := NewCollector(logger)
	ctx := context.Background()

	diagnostics := collector.CollectBuildDiagnostics(ctx, "/tmp/nonexistent")

	if diagnostics == nil {
		t.Fatal("CollectBuildDiagnostics returned nil")
	}

	// Should have some diagnostic data
	if len(diagnostics) == 0 {
		t.Error("Diagnostics should not be empty")
	}
}

func TestCollector_CollectDeploymentDiagnostics(t *testing.T) {
	logger := zerolog.Nop()
	collector := NewCollector(logger)
	ctx := context.Background()

	diagnostics := collector.CollectDeploymentDiagnostics(ctx, "test-namespace")

	if diagnostics == nil {
		t.Fatal("CollectDeploymentDiagnostics returned nil")
	}

	// Should have some diagnostic data
	if len(diagnostics) == 0 {
		t.Error("Diagnostics should not be empty")
	}
}
