// Package progress contains domain-level progress tracking primitives.
// It is transport-agnostic: all I/O is delegated to a Sink.
package progress

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Sink is a thin port that writes updates somewhere (CLI, MCP, log, …).
type Sink interface {
	Publish(ctx context.Context, u Update) error
	Close() error
}

// Update describes an atomic change in workflow state.
type Update struct {
	Step       int                    `json:"step"`
	Total      int                    `json:"total"`
	Message    string                 `json:"message"`
	StartedAt  time.Time              `json:"started_at"`
	Percentage int                    `json:"percentage"` // 0-100
	ETA        time.Duration          `json:"eta,omitempty"`
	Status     string                 `json:"status,omitempty"`
	TraceID    string                 `json:"trace_id,omitempty"`
	UserMeta   map[string]interface{} `json:"user_meta,omitempty"`
}

// Option configures a Tracker.
type Option func(*Tracker)

// WithHeartbeat makes Tracker emit an update every d while work is in progress.
func WithHeartbeat(d time.Duration) Option {
	return func(t *Tracker) { t.heartbeat = d }
}

// WithThrottle sets a minimum gap between consecutive updates.
func WithThrottle(d time.Duration) Option {
	return func(t *Tracker) { t.throttle = d }
}

// WithTraceID sets a trace ID for correlation.
func WithTraceID(traceID string) Option {
	return func(t *Tracker) { t.traceID = traceID }
}

// Tracker is the simplified replacement for ChannelManager.
type Tracker struct {
	sink        Sink
	total       int
	start       time.Time
	last        time.Time
	curStep     int
	heartbeat   time.Duration
	throttle    time.Duration
	traceID     string
	errorBudget *ErrorBudget

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.Mutex
}

// NewTracker creates a progress tracker.
func NewTracker(ctx context.Context, total int, sink Sink, opts ...Option) *Tracker {
	ctx, cancel := context.WithCancel(ctx)
	t := &Tracker{
		sink:        sink,
		total:       total,
		start:       time.Now(),
		last:        time.Now(),
		heartbeat:   15 * time.Second,
		throttle:    100 * time.Millisecond,
		errorBudget: NewErrorBudget(5, 10*time.Minute),
		ctx:         ctx,
		cancel:      cancel,
	}
	for _, o := range opts {
		o(t)
	}
	return t
}

// Begin must be called once.
func (t *Tracker) Begin(msg string) {
	t.publish(0, msg, map[string]interface{}{"status": "started"})
	if t.heartbeat > 0 {
		t.wg.Add(1)
		go t.runHeartbeat()
	}
}

// Update moves progress forward.
func (t *Tracker) Update(step int, msg string, meta map[string]interface{}) {
	if meta == nil {
		meta = make(map[string]interface{})
	}
	meta["status"] = "running"
	t.publish(step, msg, meta)
}

// Complete marks the workflow done.
func (t *Tracker) Complete(msg string) {
	t.publish(t.total, msg, map[string]interface{}{"status": "completed"})
	t.Finish()
}

// Error marks an error occurred.
func (t *Tracker) Error(step int, msg string, err error) {
	meta := map[string]interface{}{
		"status": "failed",
		"error":  err.Error(),
	}
	t.publish(step, msg, meta)
}

// Finish stops heartbeat & drains sink.
func (t *Tracker) Finish() {
	t.cancel()
	t.wg.Wait()
	_ = t.sink.Close()
}

// GetCurrent returns current step (thread-safe).
func (t *Tracker) GetCurrent() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.curStep
}

// SetCurrent sets the current step.
func (t *Tracker) SetCurrent(step int) {
	t.Update(step, fmt.Sprintf("Step %d/%d", step, t.total), nil)
}

// GetTotal returns total steps.
func (t *Tracker) GetTotal() int {
	return t.total
}

// IsComplete checks if all steps are done.
func (t *Tracker) IsComplete() bool {
	return t.GetCurrent() >= t.total
}

// GetTraceID returns the trace ID.
func (t *Tracker) GetTraceID() string {
	return t.traceID
}

// RecordError records an error and returns true if within budget.
func (t *Tracker) RecordError(err error) bool {
	return t.errorBudget.RecordError(err)
}

// RecordSuccess records a successful operation.
func (t *Tracker) RecordSuccess() {
	t.errorBudget.RecordSuccess()
}

// IsCircuitOpen returns true if error budget is exceeded.
func (t *Tracker) IsCircuitOpen() bool {
	return t.errorBudget.IsCircuitOpen()
}

// GetErrorBudgetStatus returns current error budget status.
func (t *Tracker) GetErrorBudgetStatus() ErrorBudgetStatus {
	return t.errorBudget.GetStatus()
}

// UpdateWithErrorHandling updates progress with error handling.
func (t *Tracker) UpdateWithErrorHandling(step int, msg string, meta map[string]interface{}, err error) bool {
	if meta == nil {
		meta = make(map[string]interface{})
	}

	if err != nil {
		withinBudget := t.RecordError(err)
		if !withinBudget {
			meta["error_budget_exceeded"] = true
			meta["circuit_open"] = true
		}
		meta["error"] = err.Error()
		meta["status"] = "failed"
		t.publish(step, msg, meta)
		return withinBudget
	} else {
		t.RecordSuccess()
		meta["status"] = "completed"
		t.publish(step, msg, meta)
		return true
	}
}

// ---- Internals -------------------------------------------------------------

func (t *Tracker) publish(step int, msg string, meta map[string]interface{}) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Throttling (except final step)
	if time.Since(t.last) < t.throttle && step < t.total {
		return
	}
	t.curStep = step
	t.last = time.Now()

	u := Update{
		Step:       step,
		Total:      t.total,
		Message:    msg,
		StartedAt:  t.start,
		Percentage: int(float64(step) / float64(t.total) * 100),
		TraceID:    t.traceID,
		UserMeta:   meta,
	}

	// Calculate ETA for non-zero progress
	if step > 0 && step < t.total {
		elapsed := time.Since(t.start)
		eta := time.Duration(float64(elapsed) / float64(step) * float64(t.total-step))
		u.ETA = eta
	}

	if status, ok := meta["status"].(string); ok {
		u.Status = status
	}

	_ = t.sink.Publish(t.ctx, u)
}

func (t *Tracker) runHeartbeat() {
	defer t.wg.Done()
	ticker := time.NewTicker(t.heartbeat)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			t.mu.Lock()
			msg := "Still working..."
			step := t.curStep
			t.mu.Unlock()
			t.publish(step, msg, map[string]interface{}{"kind": "heartbeat"})
		case <-t.ctx.Done():
			return
		}
	}
}
