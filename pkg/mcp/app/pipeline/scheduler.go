package pipeline

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
)

type (
	// Job describes an atomic action (build, scan, deploy...)
	Job struct {
		ID      string
		Run     func(ctx context.Context) error
		Timeout time.Duration
	}

	// Option configures Scheduler
	Option func(*Scheduler)

	// Scheduler is a minimal orchestration primitive that replaces
	// Manager + BackgroundWorkerManager + JobOrchestrator
	Scheduler struct {
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
)

// NewScheduler creates a Scheduler with sane defaults
func NewScheduler(l zerolog.Logger, opts ...Option) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Scheduler{
		workers: 4,
		queue:   make(chan Job, 100),
		log:     l,
		ctx:     ctx,
		cancel:  cancel,
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

func WithWorkers(n int) Option { return func(s *Scheduler) { s.workers = n } }
func WithQueueSize(n int) Option {
	return func(s *Scheduler) { s.queue = make(chan Job, n) }
}

/*** Public API ***/

// Start spins up worker goroutines *once*
func (s *Scheduler) Start() error {
	s.startOnce.Do(func() {
		for i := 0; i < s.workers; i++ {
			s.wg.Add(1)
			go s.worker(i)
		}
		s.log.Info().Int("workers", s.workers).Msg("scheduler started")
	})
	return nil
}

// Stop attempts a graceful shutdown
func (s *Scheduler) Stop() error {
	s.stopOnce.Do(func() {
		s.stopped.Store(true)
		s.cancel()
		close(s.queue)
		s.wg.Wait()
		s.log.Info().Msg("scheduler stopped")
	})
	return nil
}

// Submit is non-blocking; it enqueues the job or returns an error if the
// context has been cancelled
func (s *Scheduler) Submit(j Job) error {
	if s.stopped.Load() {
		return context.Canceled
	}
	
	select {
	case s.queue <- j:
		return nil
	case <-s.ctx.Done():
		return s.ctx.Err()
	}
}

/*** Internal ***/

func (s *Scheduler) worker(idx int) {
	defer s.wg.Done()
	for {
		select {
		case j, ok := <-s.queue:
			if !ok {
				return
			}
			s.runJob(j, idx)
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *Scheduler) runJob(j Job, idx int) {
	ctx := s.ctx
	if j.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, j.Timeout)
		defer cancel()
	}
	err := j.Run(ctx)
	if err != nil {
		s.log.Error().Err(err).Str("job", j.ID).Int("worker", idx).Msg("job failed")
	} else {
		s.log.Info().Str("job", j.ID).Int("worker", idx).Msg("job finished")
	}
}

/*** Bridge for legacy callers ***/

// LegacyManager wraps Scheduler to keep old code compiling; delete once
// callers move
type LegacyManager = Scheduler

// NewManager is kept for compatibility
func NewManager(l zerolog.Logger) *LegacyManager { return NewScheduler(l) }