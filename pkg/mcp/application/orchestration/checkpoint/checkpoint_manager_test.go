package checkpoint

import (
	"testing"

	"github.com/rs/zerolog"
)

// TestCompressionMode tests CompressionMode constants
func TestCompressionMode(t *testing.T) {
	if NoCompression != 0 {
		t.Errorf("Expected NoCompression to be 0, got %d", NoCompression)
	}
	if GzipCompression != 1 {
		t.Errorf("Expected GzipCompression to be 1, got %d", GzipCompression)
	}
}

// TestCheckpointOptions tests CheckpointOptions type
func TestCheckpointOptions(t *testing.T) {
	options := CheckpointOptions{
		Compression:     GzipCompression,
		EnableIntegrity: true,
		IncludeMetrics:  false,
		CompactOldData:  true,
	}

	if options.Compression != GzipCompression {
		t.Errorf("Expected Compression to be GzipCompression, got %d", options.Compression)
	}
	if !options.EnableIntegrity {
		t.Error("Expected EnableIntegrity to be true")
	}
	if options.IncludeMetrics {
		t.Error("Expected IncludeMetrics to be false")
	}
	if !options.CompactOldData {
		t.Error("Expected CompactOldData to be true")
	}
}

// TestBoltCheckpointManagerStruct tests BoltCheckpointManager struct without actual database
func TestBoltCheckpointManagerStruct(t *testing.T) {
	logger := zerolog.Nop()

	manager := BoltCheckpointManager{
		db:              nil,
		logger:          logger,
		compressionMode: GzipCompression,
		enableIntegrity: true,
	}

	if manager.compressionMode != GzipCompression {
		t.Errorf("Expected compressionMode to be GzipCompression, got %d", manager.compressionMode)
	}
	if !manager.enableIntegrity {
		t.Error("Expected enableIntegrity to be true")
	}
}

// TestCheckpointOptionsVariations tests that we can create CheckpointOptions with different configurations
func TestCheckpointOptionsVariations(t *testing.T) {
	minimalOptions := CheckpointOptions{
		Compression:     NoCompression,
		EnableIntegrity: false,
		IncludeMetrics:  false,
		CompactOldData:  false,
	}

	if minimalOptions.Compression != NoCompression {
		t.Errorf("Expected Compression to be NoCompression, got %d", minimalOptions.Compression)
	}
	if minimalOptions.EnableIntegrity {
		t.Error("Expected EnableIntegrity to be false")
	}

	maximalOptions := CheckpointOptions{
		Compression:     GzipCompression,
		EnableIntegrity: true,
		IncludeMetrics:  true,
		CompactOldData:  true,
	}

	if maximalOptions.Compression != GzipCompression {
		t.Errorf("Expected Compression to be GzipCompression, got %d", maximalOptions.Compression)
	}
	if !maximalOptions.EnableIntegrity {
		t.Error("Expected EnableIntegrity to be true")
	}
	if !maximalOptions.IncludeMetrics {
		t.Error("Expected IncludeMetrics to be true")
	}
	if !maximalOptions.CompactOldData {
		t.Error("Expected CompactOldData to be true")
	}
}
