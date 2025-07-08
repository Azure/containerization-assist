// Package pipeline provides migration examples from the old manager chain to the new scheduler
package pipeline

import (
	"context"
	"time"

	"github.com/rs/zerolog"
)

// Example: Migrating from old Manager pattern to new Scheduler
//
// BEFORE (old pattern):
//   manager := pipeline.NewManager(...)
//   manager.RegisterWorker("cleanup", "Cleanup Worker", cleanupFunc, 5*time.Minute)
//   manager.Start()
//   manager.SubmitJob(someJob)
//   manager.Stop()
//
// AFTER (new pattern with adapter):
//   scheduler := pipeline.NewScheduler(4, 100, log)
//   adapter := pipeline.NewManagerAdapter(scheduler)
//   adapter.RegisterWorker("cleanup", "Cleanup Worker", cleanupFunc, 5*time.Minute)
//   adapter.Start()
//   adapter.SubmitJob(someJob)
//   adapter.Stop()
//
// AFTER (direct scheduler usage - recommended):
//   scheduler := pipeline.NewScheduler(4, 100, log)
//   scheduler.Start()
//   scheduler.Submit(&CustomJob{...})
//   scheduler.Stop()

// ExampleMigration shows how to migrate from Manager to Scheduler
func ExampleMigration(log zerolog.Logger) {
	// Option 1: Use adapter for minimal code changes
	scheduler := NewScheduler(4, 100, log)
	adapter := NewManagerAdapter(scheduler)
	
	// This looks like the old API but uses the new scheduler
	adapter.Start()
	adapter.RegisterWorker("example", "Example Worker", func(ctx context.Context) error {
		// Worker logic here
		return nil
	}, 1*time.Minute)
	
	// Option 2: Direct scheduler usage (recommended)
	scheduler2 := NewScheduler(4, 100, log)
	scheduler2.Start()
	
	// Create custom job types
	job := &CustomJob{
		id:   "custom-job-1",
		work: func() error {
			// Job logic here
			return nil
		},
	}
	scheduler2.Submit(job)
	
	// Cleanup
	adapter.Stop()
	scheduler2.Stop()
}

// CustomJob shows how to implement the Job interface
type CustomJob struct {
	id   string
	work func() error
}

func (c *CustomJob) Execute(ctx context.Context) error {
	// Check context for cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return c.work()
	}
}

func (c *CustomJob) ID() string {
	return c.id
}

func (c *CustomJob) Timeout() time.Duration {
	return 5 * time.Minute
}

// Migration Notes:
//
// 1. The new Scheduler is much simpler - only Start(), Stop(), Submit()
// 2. No more RegisterWorker - create recurring jobs with a ticker instead
// 3. No more GetJob/CancelJob - use context for cancellation
// 4. No more complex stats - basic health check only
// 5. Jobs must implement the Job interface (Execute, ID, Timeout)
//
// Benefits:
// - 90% less code (125 lines vs 1,223 lines)
// - Clearer separation of concerns
// - Better testability
// - Context-based cancellation
// - Simpler mental model