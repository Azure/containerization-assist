package checkpoint

import (
	"github.com/rs/zerolog"
	"go.etcd.io/bbolt"
)

// NewBoltCheckpointManager creates a new BoltDB-backed checkpoint manager
func NewBoltCheckpointManager(db *bbolt.DB, logger zerolog.Logger) *BoltCheckpointManager {
	return &BoltCheckpointManager{
		db:              db,
		logger:          logger.With().Str("component", "checkpoint_manager").Logger(),
		compressionMode: GzipCompression,
		enableIntegrity: true,
	}
}

// NewBoltCheckpointManagerWithOptions creates a checkpoint manager with custom options
func NewBoltCheckpointManagerWithOptions(db *bbolt.DB, logger zerolog.Logger, opts CheckpointOptions) *BoltCheckpointManager {
	return &BoltCheckpointManager{
		db:              db,
		logger:          logger.With().Str("component", "checkpoint_manager").Logger(),
		compressionMode: opts.Compression,
		enableIntegrity: opts.EnableIntegrity,
	}
}
