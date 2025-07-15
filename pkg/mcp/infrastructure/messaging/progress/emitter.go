// Package progress provides simplified ProgressEmitter implementations
package progress

import (
	"context"

	"github.com/Azure/container-kit/pkg/mcp/api"
)

// NoOpEmitter is a no-operation progress emitter for testing or when progress reporting is disabled
type NoOpEmitter struct{}

// NewNoOpEmitter creates a no-operation progress emitter
func NewNoOpEmitter() *NoOpEmitter {
	return &NoOpEmitter{}
}

// Emit implements api.ProgressEmitter with no operation
func (e *NoOpEmitter) Emit(ctx context.Context, stage string, percent int, message string) error {
	return nil
}

// EmitDetailed implements api.ProgressEmitter with no operation
func (e *NoOpEmitter) EmitDetailed(ctx context.Context, update api.ProgressUpdate) error {
	return nil
}

// Close implements api.ProgressEmitter with no operation
func (e *NoOpEmitter) Close() error {
	return nil
}
