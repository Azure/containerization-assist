package pipeline

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestSchedulerBasicOperations(t *testing.T) {
	logger := zerolog.Nop()
	s := NewScheduler(logger)

	// Test start
	if err := s.Start(); err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}

	// Test job submission
	jobExecuted := false
	job := Job{
		ID: "test-job",
		Run: func(ctx context.Context) error {
			jobExecuted = true
			return nil
		},
		Timeout: time.Second,
	}

	if err := s.Submit(job); err != nil {
		t.Fatalf("Failed to submit job: %v", err)
	}

	// Give time for job execution
	time.Sleep(100 * time.Millisecond)

	// Test stop
	if err := s.Stop(); err != nil {
		t.Fatalf("Failed to stop scheduler: %v", err)
	}

	if !jobExecuted {
		t.Error("Job was not executed")
	}
}

func TestSchedulerOptions(t *testing.T) {
	logger := zerolog.Nop()
	s := NewScheduler(logger, WithWorkers(8), WithQueueSize(200))

	if s.workers != 8 {
		t.Errorf("Expected 8 workers, got %d", s.workers)
	}

	if cap(s.queue) != 200 {
		t.Errorf("Expected queue size 200, got %d", cap(s.queue))
	}
}

func TestSchedulerMultipleJobs(t *testing.T) {
	logger := zerolog.Nop()
	s := NewScheduler(logger, WithWorkers(2))

	if err := s.Start(); err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}
	defer s.Stop()

	var counter int32
	jobCount := 10

	for i := 0; i < jobCount; i++ {
		job := Job{
			ID: "job-" + string(rune(i)),
			Run: func(ctx context.Context) error {
				atomic.AddInt32(&counter, 1)
				return nil
			},
		}
		if err := s.Submit(job); err != nil {
			t.Fatalf("Failed to submit job %d: %v", i, err)
		}
	}

	// Wait for all jobs to complete
	time.Sleep(200 * time.Millisecond)

	finalCount := atomic.LoadInt32(&counter)
	if finalCount != int32(jobCount) {
		t.Errorf("Expected %d jobs executed, got %d", jobCount, finalCount)
	}
}

func TestSchedulerJobTimeout(t *testing.T) {
	logger := zerolog.Nop()
	s := NewScheduler(logger)

	if err := s.Start(); err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}
	defer s.Stop()

	jobCompleted := false
	job := Job{
		ID: "timeout-job",
		Run: func(ctx context.Context) error {
			select {
			case <-time.After(2 * time.Second):
				jobCompleted = true
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
		Timeout: 100 * time.Millisecond,
	}

	if err := s.Submit(job); err != nil {
		t.Fatalf("Failed to submit job: %v", err)
	}

	// Wait for timeout
	time.Sleep(300 * time.Millisecond)

	if jobCompleted {
		t.Error("Job should have timed out")
	}
}

func TestSchedulerErrorHandling(t *testing.T) {
	logger := zerolog.Nop()
	s := NewScheduler(logger)

	if err := s.Start(); err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}
	defer s.Stop()

	job := Job{
		ID: "error-job",
		Run: func(ctx context.Context) error {
			return errors.New("intentional error")
		},
	}

	// Should not return error on submission
	if err := s.Submit(job); err != nil {
		t.Fatalf("Failed to submit job: %v", err)
	}

	// Give time for job execution
	time.Sleep(100 * time.Millisecond)
	// Test passes if no panic occurs
}

func TestSchedulerGracefulShutdown(t *testing.T) {
	logger := zerolog.Nop()
	s := NewScheduler(logger, WithWorkers(4))

	if err := s.Start(); err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}

	// Submit jobs that take some time
	for i := 0; i < 10; i++ {
		job := Job{
			ID: "slow-job",
			Run: func(ctx context.Context) error {
				select {
				case <-time.After(50 * time.Millisecond):
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			},
		}
		if err := s.Submit(job); err != nil {
			t.Fatalf("Failed to submit job: %v", err)
		}
	}

	// Stop should wait for all jobs to complete
	if err := s.Stop(); err != nil {
		t.Fatalf("Failed to stop scheduler: %v", err)
	}

	// Try to submit after stop
	job := Job{
		ID: "post-stop-job",
		Run: func(ctx context.Context) error {
			return nil
		},
	}

	err := s.Submit(job)
	if err == nil {
		t.Error("Expected error when submitting after stop")
	}
}

func TestSchedulerStartOnce(t *testing.T) {
	logger := zerolog.Nop()
	s := NewScheduler(logger)

	// Start multiple times should be idempotent
	for i := 0; i < 3; i++ {
		if err := s.Start(); err != nil {
			t.Fatalf("Failed to start scheduler on attempt %d: %v", i+1, err)
		}
	}

	if err := s.Stop(); err != nil {
		t.Fatalf("Failed to stop scheduler: %v", err)
	}
}

func TestSchedulerStopOnce(t *testing.T) {
	logger := zerolog.Nop()
	s := NewScheduler(logger)

	if err := s.Start(); err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}

	// Stop multiple times should be idempotent
	for i := 0; i < 3; i++ {
		if err := s.Stop(); err != nil {
			t.Fatalf("Failed to stop scheduler on attempt %d: %v", i+1, err)
		}
	}
}

func TestLegacyManagerCompatibility(t *testing.T) {
	logger := zerolog.Nop()
	
	// Test that legacy constructor works
	m := NewManager(logger)
	
	// Test that it's actually a Scheduler
	if _, ok := interface{}(m).(*Scheduler); !ok {
		t.Error("NewManager should return a *Scheduler")
	}
	
	// Test basic operations work
	if err := m.Start(); err != nil {
		t.Fatalf("Failed to start legacy manager: %v", err)
	}
	
	if err := m.Stop(); err != nil {
		t.Fatalf("Failed to stop legacy manager: %v", err)
	}
}