package checkpoint

import (
	"time"

	"github.com/rs/zerolog"
	"go.etcd.io/bbolt"
)

// BoltCheckpointManager implements CheckpointManager using BoltDB with compression and integrity checks
type BoltCheckpointManager struct {
	db              *bbolt.DB
	logger          zerolog.Logger
	compressionMode CompressionMode
	enableIntegrity bool
}

// CompressionMode defines how checkpoint data is compressed
type CompressionMode int

const (
	NoCompression CompressionMode = iota
	GzipCompression
)

// CheckpointOptions configures checkpoint behavior
type CheckpointOptions struct {
	Compression     CompressionMode
	EnableIntegrity bool
	IncludeMetrics  bool
	CompactOldData  bool
}

const (
	checkpointsBucket = "workflow_checkpoints"
	metadataBucket    = "checkpoint_metadata"
)

// CheckpointEnvelope wraps checkpoint data with metadata for compression and integrity
type CheckpointEnvelope struct {
	Version    int               `json:"version"`
	Compressed bool              `json:"compressed"`
	Checksum   string            `json:"checksum,omitempty"`
	DataSize   int               `json:"data_size"`
	CreatedAt  time.Time         `json:"created_at"`
	Metadata   map[string]string `json:"metadata,omitempty"` // Changed to map[string]string for type safety
	Data       []byte            `json:"data"`
}

// CheckpointMetrics contains metrics about workflow checkpoints
type CheckpointMetrics struct {
	TotalCheckpoints int            `json:"total_checkpoints"`
	SessionCounts    map[string]int `json:"session_counts"`
	StageCounts      map[string]int `json:"stage_counts"`
	LastCheckpoint   time.Time      `json:"last_checkpoint"`
}
