// Package progress provides test helpers for progress tracking
package progress

import (
	"context"
	"sync"
	"time"

	"github.com/Azure/containerization-assist/pkg/api"
	"github.com/mark3labs/mcp-go/mcp"
)

// TestProgressEmitter is a test implementation that records all progress updates
type TestProgressEmitter struct {
	mu      sync.Mutex
	updates []api.ProgressUpdate
	closed  bool
}

// NewTestProgressEmitter creates a new test progress emitter
func NewTestProgressEmitter() *TestProgressEmitter {
	return &TestProgressEmitter{
		updates: make([]api.ProgressUpdate, 0),
	}
}

// Emit records a simple progress update
func (e *TestProgressEmitter) Emit(ctx context.Context, stage string, percent int, message string) error {
	e.mu.Lock()
	stepNum := len(e.updates) + 1
	e.mu.Unlock()

	return e.EmitDetailed(ctx, api.ProgressUpdate{
		Step:       stepNum,
		Total:      10, // Default workflow steps
		Stage:      stage,
		Percentage: percent,
		Message:    message,
		Status:     "running",
	})
}

// EmitDetailed records a detailed progress update
func (e *TestProgressEmitter) EmitDetailed(ctx context.Context, update api.ProgressUpdate) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Set StartedAt if not set
	if update.StartedAt.IsZero() {
		update.StartedAt = time.Now()
	}

	e.updates = append(e.updates, update)
	return nil
}

// Close marks the emitter as closed and emits a completion status
func (e *TestProgressEmitter) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Emit a completion update if not already closed
	if !e.closed {
		// Find the highest step number and percentage from existing updates
		maxStep := 0
		totalSteps := 10 // Default for workflow
		for _, u := range e.updates {
			if u.Step > maxStep {
				maxStep = u.Step
			}
			if u.Total > 0 {
				totalSteps = u.Total
			}
		}

		e.updates = append(e.updates, api.ProgressUpdate{
			Step:       totalSteps,
			Total:      totalSteps,
			Stage:      "completed",
			Message:    "Workflow completed",
			Percentage: 100,
			Status:     "completed",
		})
	}

	e.closed = true
	return nil
}

// GetUpdates returns all recorded updates (thread-safe)
func (e *TestProgressEmitter) GetUpdates() []api.ProgressUpdate {
	e.mu.Lock()
	defer e.mu.Unlock()

	result := make([]api.ProgressUpdate, len(e.updates))
	copy(result, e.updates)
	return result
}

// IsClosed returns whether Close() was called
func (e *TestProgressEmitter) IsClosed() bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	return e.closed
}

// Reset clears all recorded updates
func (e *TestProgressEmitter) Reset() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.updates = e.updates[:0]
	e.closed = false
}

// TestDirectProgressFactory creates test progress emitters
type TestDirectProgressFactory struct {
	emitter *TestProgressEmitter
}

// NewTestDirectProgressFactory creates a factory that returns a test emitter
func NewTestDirectProgressFactory() *TestDirectProgressFactory {
	return &TestDirectProgressFactory{
		emitter: NewTestProgressEmitter(),
	}
}

// CreateEmitter returns the test emitter
func (f *TestDirectProgressFactory) CreateEmitter(ctx context.Context, req *mcp.CallToolRequest, totalSteps int) api.ProgressEmitter {
	return f.emitter
}

// GetTestEmitter returns the underlying test emitter for assertions
func (f *TestDirectProgressFactory) GetTestEmitter() *TestProgressEmitter {
	return f.emitter
}

// Ensure implementations satisfy interfaces
var (
	_ api.ProgressEmitter = (*TestProgressEmitter)(nil)
	// Note: TestDirectProgressFactory implements workflow.ProgressEmitterFactory
	// but we can't verify it here due to import cycles
)
