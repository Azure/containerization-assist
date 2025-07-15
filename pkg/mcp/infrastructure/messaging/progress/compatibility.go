package progress

import (
	"context"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/Azure/container-kit/pkg/mcp/domain/progress"
)

// EmitterToSinkAdapter adapts the new ProgressEmitter interface to the old Sink interface
// This allows gradual migration while maintaining compatibility with existing code
type EmitterToSinkAdapter struct {
	emitter api.ProgressEmitter
}

// NewEmitterToSinkAdapter creates an adapter that allows using new emitters where old sinks are expected
func NewEmitterToSinkAdapter(emitter api.ProgressEmitter) progress.Sink {
	return &EmitterToSinkAdapter{emitter: emitter}
}

// Publish implements progress.Sink by converting to ProgressEmitter calls
func (a *EmitterToSinkAdapter) Publish(ctx context.Context, u progress.Update) error {
	// Extract stage from metadata if available
	stage := ""
	if u.UserMeta != nil {
		if s, ok := u.UserMeta["stage"].(string); ok {
			stage = s
		} else if s, ok := u.UserMeta["step_name"].(string); ok {
			stage = s
		}
	}

	// Convert domain Update to API ProgressUpdate
	update := api.ProgressUpdate{
		Step:       u.Step,
		Total:      u.Total,
		Stage:      stage,
		Message:    u.Message,
		Percentage: u.Percentage,
		StartedAt:  u.StartedAt,
		ETA:        u.ETA,
		Status:     u.Status,
		TraceID:    u.TraceID,
		Metadata:   u.UserMeta,
	}

	return a.emitter.EmitDetailed(ctx, update)
}

// Close implements progress.Sink
func (a *EmitterToSinkAdapter) Close() error {
	return a.emitter.Close()
}

// Ensure adapter implements Sink interface
var _ progress.Sink = (*EmitterToSinkAdapter)(nil)

// SinkToEmitterAdapter adapts the old Sink interface to the new ProgressEmitter interface
// This is useful when we have old sink implementations but need a ProgressEmitter
type SinkToEmitterAdapter struct {
	sink   progress.Sink
	logger interface{} // Keep for compatibility but not used
}

// NewSinkToEmitterAdapter creates an adapter that allows using old sinks where new emitters are expected
func NewSinkToEmitterAdapter(sink progress.Sink) api.ProgressEmitter {
	return &SinkToEmitterAdapter{sink: sink}
}

// Emit implements api.ProgressEmitter
func (a *SinkToEmitterAdapter) Emit(ctx context.Context, stage string, percent int, message string) error {
	update := progress.Update{
		Step:       0, // Will be set by tracker if needed
		Total:      100,
		Message:    message,
		Percentage: percent,
		Status:     "running",
		StartedAt:  time.Now(),
		UserMeta: map[string]interface{}{
			"stage": stage,
		},
	}

	return a.sink.Publish(ctx, update)
}

// EmitDetailed implements api.ProgressEmitter
func (a *SinkToEmitterAdapter) EmitDetailed(ctx context.Context, update api.ProgressUpdate) error {
	// Convert API ProgressUpdate to domain Update
	u := progress.Update{
		Step:       update.Step,
		Total:      update.Total,
		Message:    update.Message,
		Percentage: update.Percentage,
		StartedAt:  update.StartedAt,
		ETA:        update.ETA,
		Status:     update.Status,
		TraceID:    update.TraceID,
		UserMeta:   update.Metadata,
	}

	// Ensure stage is in metadata
	if u.UserMeta == nil {
		u.UserMeta = make(map[string]interface{})
	}
	if update.Stage != "" {
		u.UserMeta["stage"] = update.Stage
	}

	return a.sink.Publish(ctx, u)
}

// Close implements api.ProgressEmitter
func (a *SinkToEmitterAdapter) Close() error {
	return a.sink.Close()
}

// Ensure adapter implements ProgressEmitter interface
var _ api.ProgressEmitter = (*SinkToEmitterAdapter)(nil)
