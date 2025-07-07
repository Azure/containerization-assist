package common

import (
	"context"
	"time"
)

// PeriodicTask represents a task that runs periodically
type PeriodicTask struct {
	Name     string
	Interval time.Duration
	Task     func(ctx context.Context) error
	ctx      context.Context
	cancel   context.CancelFunc
	started  bool
}

// NewPeriodicTask creates a new periodic task
func NewPeriodicTask(name string, interval time.Duration, task func(ctx context.Context) error) *PeriodicTask {
	ctx, cancel := context.WithCancel(context.Background())
	return &PeriodicTask{
		Name:     name,
		Interval: interval,
		Task:     task,
		ctx:      ctx,
		cancel:   cancel,
		started:  false,
	}
}

// Start begins the periodic execution
func (pt *PeriodicTask) Start() {
	if pt.started {
		return
	}
	pt.started = true

	go func() {
		ticker := time.NewTicker(pt.Interval)
		defer ticker.Stop()

		for {
			select {
			case <-pt.ctx.Done():
				return
			case <-ticker.C:
				if err := pt.Task(pt.ctx); err != nil {
					continue
				}
			}
		}
	}()
}

// Stop stops the periodic task with a grace period
func (pt *PeriodicTask) Stop(gracePeriod time.Duration) error {
	if pt.cancel != nil {
		pt.cancel()
	}
	pt.started = false

	time.Sleep(gracePeriod)

	return nil
}

// IsRunning returns true if the task is currently running
func (pt *PeriodicTask) IsRunning() bool {
	return pt.started
}
