// Package progress provides ProgressEmitter implementations for various transport modes
package progress

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/Azure/container-kit/pkg/mcp/domain/progress"
)

// TrackerEmitter adapts the existing progress.Tracker to implement api.ProgressEmitter
type TrackerEmitter struct {
	tracker    *progress.Tracker
	totalSteps int
	startTime  time.Time
}

// NewTrackerEmitter creates a ProgressEmitter that wraps the existing progress.Tracker
func NewTrackerEmitter(ctx context.Context, totalSteps int, sink progress.Sink, opts ...progress.Option) *TrackerEmitter {
	tracker := progress.NewTracker(ctx, totalSteps, sink, opts...)
	tracker.Begin("Starting workflow")

	return &TrackerEmitter{
		tracker:    tracker,
		totalSteps: totalSteps,
		startTime:  time.Now(),
	}
}

// Emit implements api.ProgressEmitter by converting to tracker update
func (e *TrackerEmitter) Emit(ctx context.Context, stage string, percent int, message string) error {
	// Convert percentage to step number
	step := int(float64(percent) / 100.0 * float64(e.totalSteps))
	if step > e.totalSteps {
		step = e.totalSteps
	}

	// Create metadata with stage information
	meta := map[string]interface{}{
		"stage": stage,
	}

	e.tracker.Update(step, message, meta)
	return nil
}

// EmitDetailed implements api.ProgressEmitter with full structured update
func (e *TrackerEmitter) EmitDetailed(ctx context.Context, update api.ProgressUpdate) error {
	// Convert api.ProgressUpdate to tracker format
	meta := map[string]interface{}{
		"stage":  update.Stage,
		"status": update.Status,
	}

	// Add custom metadata
	for k, v := range update.Metadata {
		meta[k] = v
	}

	e.tracker.Update(update.Step, update.Message, meta)
	return nil
}

// Close implements api.ProgressEmitter
func (e *TrackerEmitter) Close() error {
	e.tracker.Complete("Workflow completed")
	return nil
}

// BatchedEmitter collects progress updates and emits them in batches
type BatchedEmitter struct {
	emitter    api.ProgressEmitter
	updates    []api.ProgressUpdate
	batchSize  int
	flushTimer *time.Timer
	done       chan struct{}
}

// NewBatchedEmitter creates a batched progress emitter
func NewBatchedEmitter(emitter api.ProgressEmitter, batchSize int, flushInterval time.Duration) *BatchedEmitter {
	b := &BatchedEmitter{
		emitter:   emitter,
		updates:   make([]api.ProgressUpdate, 0, batchSize),
		batchSize: batchSize,
		done:      make(chan struct{}),
	}

	// Start flush timer
	b.flushTimer = time.AfterFunc(flushInterval, func() { _ = b.flush() })

	return b
}

// Emit implements api.ProgressEmitter with batching
func (b *BatchedEmitter) Emit(ctx context.Context, stage string, percent int, message string) error {
	update := api.ProgressUpdate{
		Stage:      stage,
		Message:    message,
		Percentage: percent,
		StartedAt:  time.Now(),
	}

	return b.EmitDetailed(ctx, update)
}

// EmitDetailed implements api.ProgressEmitter with batching
func (b *BatchedEmitter) EmitDetailed(ctx context.Context, update api.ProgressUpdate) error {
	b.updates = append(b.updates, update)

	// Flush if batch is full
	if len(b.updates) >= b.batchSize {
		return b.flush()
	}

	return nil
}

// Close implements api.ProgressEmitter
func (b *BatchedEmitter) Close() error {
	close(b.done)
	if b.flushTimer != nil {
		b.flushTimer.Stop()
	}

	// Flush remaining updates
	if err := b.flush(); err != nil {
		return err
	}

	return b.emitter.Close()
}

func (b *BatchedEmitter) flush() error {
	if len(b.updates) == 0 {
		return nil
	}

	// Send all updates in batch
	for _, update := range b.updates {
		if err := b.emitter.EmitDetailed(context.Background(), update); err != nil {
			return fmt.Errorf("batch flush failed: %w", err)
		}
	}

	// Clear the batch
	b.updates = b.updates[:0]

	// Reset timer
	if b.flushTimer != nil {
		b.flushTimer.Reset(5 * time.Second) // Default flush interval
	}

	return nil
}

// StreamingEmitter sends progress updates immediately (real-time)
type StreamingEmitter struct {
	emitter api.ProgressEmitter
}

// NewStreamingEmitter creates a streaming progress emitter
func NewStreamingEmitter(emitter api.ProgressEmitter) *StreamingEmitter {
	return &StreamingEmitter{
		emitter: emitter,
	}
}

// Emit implements api.ProgressEmitter with immediate sending
func (s *StreamingEmitter) Emit(ctx context.Context, stage string, percent int, message string) error {
	return s.emitter.Emit(ctx, stage, percent, message)
}

// EmitDetailed implements api.ProgressEmitter with immediate sending
func (s *StreamingEmitter) EmitDetailed(ctx context.Context, update api.ProgressUpdate) error {
	return s.emitter.EmitDetailed(ctx, update)
}

// Close implements api.ProgressEmitter
func (s *StreamingEmitter) Close() error {
	return s.emitter.Close()
}
