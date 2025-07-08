package pipeline

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testJob struct {
	id       string
	execFunc func(ctx context.Context) error
	timeout  time.Duration
}

func (t *testJob) Execute(ctx context.Context) error {
	if t.execFunc != nil {
		return t.execFunc(ctx)
	}
	return nil
}

func (t *testJob) ID() string {
	return t.id
}

func (t *testJob) Timeout() time.Duration {
	if t.timeout > 0 {
		return t.timeout
	}
	return 1 * time.Second
}

func TestSchedulerBasicOperations(t *testing.T) {
	log := zerolog.Nop()
	scheduler := NewScheduler(2, 10, log)

	// Test Start
	err := scheduler.Start()
	require.NoError(t, err)

	// Test Submit
	executed := atomic.Bool{}
	job := &testJob{
		id: "test-job-1",
		execFunc: func(ctx context.Context) error {
			executed.Store(true)
			return nil
		},
	}

	err = scheduler.Submit(job)
	require.NoError(t, err)

	// Wait for execution
	time.Sleep(100 * time.Millisecond)
	assert.True(t, executed.Load())

	// Test Stop
	err = scheduler.Stop()
	require.NoError(t, err)
}

func TestSchedulerConcurrentJobs(t *testing.T) {
	log := zerolog.Nop()
	scheduler := NewScheduler(4, 20, log)
	require.NoError(t, scheduler.Start())
	defer scheduler.Stop()

	var wg sync.WaitGroup
	executed := atomic.Int32{}

	// Submit 10 concurrent jobs
	for i := 0; i < 10; i++ {
		wg.Add(1)
		job := &testJob{
			id: "test-job-" + string(rune(i)),
			execFunc: func(ctx context.Context) error {
				executed.Add(1)
				wg.Done()
				return nil
			},
		}
		err := scheduler.Submit(job)
		require.NoError(t, err)
	}

	wg.Wait()
	assert.Equal(t, int32(10), executed.Load())
}

func TestSchedulerJobTimeout(t *testing.T) {
	log := zerolog.Nop()
	scheduler := NewScheduler(1, 5, log)
	require.NoError(t, scheduler.Start())
	defer scheduler.Stop()

	done := make(chan bool)
	job := &testJob{
		id:      "timeout-job",
		timeout: 50 * time.Millisecond,
		execFunc: func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				done <- true
				return ctx.Err()
			case <-time.After(1 * time.Second):
				done <- false
				return nil
			}
		},
	}

	err := scheduler.Submit(job)
	require.NoError(t, err)

	// Job should timeout
	timedOut := <-done
	assert.True(t, timedOut)
}

func TestSchedulerQueueFull(t *testing.T) {
	log := zerolog.Nop()
	scheduler := NewScheduler(1, 2, log) // Small queue
	require.NoError(t, scheduler.Start())
	defer scheduler.Stop()

	// Block the worker
	blockCh := make(chan struct{})
	job1 := &testJob{
		id: "blocker",
		execFunc: func(ctx context.Context) error {
			<-blockCh
			return nil
		},
	}
	scheduler.Submit(job1)

	// Fill the queue
	scheduler.Submit(&testJob{id: "job2"})
	scheduler.Submit(&testJob{id: "job3"})

	// This should fail - queue full
	err := scheduler.Submit(&testJob{id: "job4"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "queue is full")

	close(blockCh)
}

func TestSchedulerStopSubmit(t *testing.T) {
	log := zerolog.Nop()
	scheduler := NewScheduler(2, 10, log)
	require.NoError(t, scheduler.Start())
	require.NoError(t, scheduler.Stop())

	// Submit after stop should fail
	err := scheduler.Submit(&testJob{id: "late-job"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "scheduler is stopped")
}

func TestSchedulerJobError(t *testing.T) {
	log := zerolog.Nop()
	scheduler := NewScheduler(1, 5, log)
	require.NoError(t, scheduler.Start())
	defer scheduler.Stop()

	executed := atomic.Bool{}
	job := &testJob{
		id: "error-job",
		execFunc: func(ctx context.Context) error {
			executed.Store(true)
			return errors.New("job failed")
		},
	}

	err := scheduler.Submit(job)
	require.NoError(t, err)

	// Wait for execution
	time.Sleep(100 * time.Millisecond)
	assert.True(t, executed.Load())
}

func TestSchedulerGracefulShutdown(t *testing.T) {
	log := zerolog.Nop()
	scheduler := NewScheduler(2, 10, log)
	require.NoError(t, scheduler.Start())

	var wg sync.WaitGroup
	completed := atomic.Int32{}

	// Submit jobs that take some time
	for i := 0; i < 5; i++ {
		wg.Add(1)
		job := &testJob{
			id: "slow-job-" + string(rune(i)),
			execFunc: func(ctx context.Context) error {
				time.Sleep(50 * time.Millisecond)
				completed.Add(1)
				wg.Done()
				return nil
			},
		}
		scheduler.Submit(job)
	}

	// Stop should wait for all jobs to complete
	go func() {
		time.Sleep(10 * time.Millisecond)
		scheduler.Stop()
	}()

	wg.Wait()
	assert.Equal(t, int32(5), completed.Load())
}

func TestSchedulerMultipleStartStop(t *testing.T) {
	log := zerolog.Nop()
	scheduler := NewScheduler(2, 10, log)

	// Multiple starts should be safe
	err1 := scheduler.Start()
	err2 := scheduler.Start()
	assert.NoError(t, err1)
	assert.NoError(t, err2)

	// Multiple stops should be safe
	err1 = scheduler.Stop()
	err2 = scheduler.Stop()
	assert.NoError(t, err1)
	assert.NoError(t, err2)
}

func TestJobFuncAdapter(t *testing.T) {
	executed := false
	f := JobFunc(func() error {
		executed = true
		return nil
	})

	// Test Execute
	err := f.Execute(context.Background())
	assert.NoError(t, err)
	assert.True(t, executed)

	// Test ID generation
	id := f.ID()
	assert.Contains(t, id, "job-")

	// Test default timeout
	timeout := f.Timeout()
	assert.Equal(t, 5*time.Minute, timeout)
}