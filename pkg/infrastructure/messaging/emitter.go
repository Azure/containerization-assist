package messaging

import (
	"context"

	"github.com/Azure/containerization-assist/pkg/api"
)

// NoOpEmitter is a no-operation progress emitter for testing or when progress reporting is disabled
type NoOpEmitter struct{}

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
