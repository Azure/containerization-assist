package pipeline

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
)

// Job represents a unit of work to be executed
type Job interface {
	Execute(ctx context.Context) error
	ID() string
	Timeout() time.Duration
}

// JobFunc is a simple adapter to convert functions to Jobs
type JobFunc func() error

func (f JobFunc) Execute(ctx context.Context) error {
	return f()
}

func (f JobFunc) ID() string {
	return fmt.Sprintf("job-%d", time.Now().UnixNano())
}

func (f JobFunc) Timeout() time.Duration {
	return 5 * time.Minute
}

// Scheduler manages job execution with a simple worker pool
type Scheduler struct {
	workers   int
	queue     chan Job
	log       zerolog.Logger
	wg        sync.WaitGroup
	ctx       context.Context
	cancel    context.CancelFunc
	startOnce sync.Once
	stopOnce  sync.Once
	stopped   atomic.Bool
}

// NewScheduler creates a new scheduler
func NewScheduler(workers int, queueSize int, log zerolog.Logger) *Scheduler {
	if workers <= 0 {
		workers = 4
	}
	if queueSize <= 0 {
		queueSize = 100
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		workers: workers,
		queue:   make(chan Job, queueSize),
		log:     log,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Start starts the scheduler workers
func (s *Scheduler) Start() error {
	var err error
	s.startOnce.Do(func() {
		s.log.Info().Int("workers", s.workers).Msg("Starting scheduler")
		
		for i := 0; i < s.workers; i++ {
			s.wg.Add(1)
			go s.worker(i)
		}
	})
	return err
}

// Stop gracefully stops the scheduler
func (s *Scheduler) Stop() error {
	var err error
	s.stopOnce.Do(func() {
		s.log.Info().Msg("Stopping scheduler")
		s.stopped.Store(true)
		s.cancel()
		close(s.queue)
		s.wg.Wait()
		s.log.Info().Msg("Scheduler stopped")
	})
	return err
}

// Submit submits a job to the scheduler
func (s *Scheduler) Submit(job Job) error {
	if s.stopped.Load() {
		return fmt.Errorf("scheduler is stopped")
	}

	select {
	case s.queue <- job:
		s.log.Debug().Str("job_id", job.ID()).Msg("Job submitted")
		return nil
	case <-s.ctx.Done():
		return fmt.Errorf("scheduler is shutting down")
	default:
		return fmt.Errorf("job queue is full")
	}
}

// worker processes jobs from the queue
func (s *Scheduler) worker(id int) {
	defer s.wg.Done()
	s.log.Debug().Int("worker_id", id).Msg("Worker started")

	for {
		select {
		case job, ok := <-s.queue:
			if !ok {
				s.log.Debug().Int("worker_id", id).Msg("Worker stopping")
				return
			}
			s.executeJob(job)
		case <-s.ctx.Done():
			s.log.Debug().Int("worker_id", id).Msg("Worker stopping due to context cancellation")
			return
		}
	}
}

// executeJob executes a single job with timeout
func (s *Scheduler) executeJob(job Job) {
	ctx, cancel := context.WithTimeout(s.ctx, job.Timeout())
	defer cancel()

	s.log.Debug().Str("job_id", job.ID()).Msg("Executing job")
	start := time.Now()

	err := job.Execute(ctx)
	duration := time.Since(start)

	if err != nil {
		s.log.Error().
			Err(err).
			Str("job_id", job.ID()).
			Dur("duration", duration).
			Msg("Job failed")
	} else {
		s.log.Debug().
			Str("job_id", job.ID()).
			Dur("duration", duration).
			Msg("Job completed")
	}
}