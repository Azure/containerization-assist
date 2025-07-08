package pipeline

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// ManagerAdapter provides backward compatibility by adapting the new Scheduler
// to work with code expecting the old Manager interface
type ManagerAdapter struct {
	scheduler *Scheduler
	workers   map[string]*WorkerAdapter
	mu        sync.RWMutex
}

// WorkerAdapter adapts between old worker types and new Job interface
type WorkerAdapter struct {
	id       string
	name     string
	jobFunc  func(ctx context.Context) error
	interval time.Duration
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

// NewManagerAdapter creates a new adapter wrapping the scheduler
func NewManagerAdapter(scheduler *Scheduler) *ManagerAdapter {
	return &ManagerAdapter{
		scheduler: scheduler,
		workers:   make(map[string]*WorkerAdapter),
	}
}

// Start starts the underlying scheduler
func (m *ManagerAdapter) Start() error {
	return m.scheduler.Start()
}

// Stop stops the scheduler and all workers
func (m *ManagerAdapter) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Stop all workers
	for _, worker := range m.workers {
		close(worker.stopCh)
		worker.wg.Wait()
	}

	return m.scheduler.Stop()
}

// RegisterWorker adapts the old worker registration to the new scheduler
func (m *ManagerAdapter) RegisterWorker(id, name string, jobFunc func(ctx context.Context) error, interval time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.workers[id]; exists {
		return fmt.Errorf("worker %s already registered", id)
	}

	worker := &WorkerAdapter{
		id:       id,
		name:     name,
		jobFunc:  jobFunc,
		interval: interval,
		stopCh:   make(chan struct{}),
	}

	m.workers[id] = worker

	// Start worker goroutine
	worker.wg.Add(1)
	go m.runWorker(worker)

	return nil
}

// UnregisterWorker stops and removes a worker
func (m *ManagerAdapter) UnregisterWorker(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	worker, exists := m.workers[id]
	if !exists {
		return fmt.Errorf("worker %s not found", id)
	}

	close(worker.stopCh)
	worker.wg.Wait()
	delete(m.workers, id)

	return nil
}

// SubmitJob submits a job to the scheduler
func (m *ManagerAdapter) SubmitJob(job interface{}) error {
	// Convert to scheduler Job type
	switch j := job.(type) {
	case Job:
		return m.scheduler.Submit(j)
	case func() error:
		// Wrap simple functions
		return m.scheduler.Submit(JobFunc(j))
	default:
		return fmt.Errorf("unsupported job type: %T", job)
	}
}

// GetJob retrieves job status (not implemented in new scheduler)
func (m *ManagerAdapter) GetJob(id string) (interface{}, error) {
	// The new scheduler doesn't track individual jobs
	// This is intentional simplification
	return nil, fmt.Errorf("job tracking not supported in new scheduler")
}

// CancelJob cancels a job (not implemented in new scheduler)
func (m *ManagerAdapter) CancelJob(id string) error {
	// The new scheduler doesn't support job cancellation
	// Jobs should use context for cancellation
	return fmt.Errorf("job cancellation not supported - use context instead")
}

// GetStats returns basic statistics
func (m *ManagerAdapter) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"workers":        len(m.workers),
		"scheduler_running": !m.scheduler.stopped.Load(),
	}
}

// GetHealth returns health status
func (m *ManagerAdapter) GetHealth() map[string]interface{} {
	return map[string]interface{}{
		"status": "healthy",
		"scheduler": !m.scheduler.stopped.Load(),
	}
}

// runWorker runs a worker's job function at the specified interval
func (m *ManagerAdapter) runWorker(worker *WorkerAdapter) {
	defer worker.wg.Done()

	ticker := time.NewTicker(worker.interval)
	defer ticker.Stop()

	for {
		select {
		case <-worker.stopCh:
			return
		case <-ticker.C:
			// Submit job to scheduler
			job := &workerJob{
				id:      worker.id,
				name:    worker.name,
				jobFunc: worker.jobFunc,
			}
			if err := m.scheduler.Submit(job); err != nil {
				// Log error but continue
				m.scheduler.log.Error().Err(err).
					Str("worker_id", worker.id).
					Msg("Failed to submit worker job")
			}
		}
	}
}

// workerJob adapts worker functions to the Job interface
type workerJob struct {
	id      string
	name    string
	jobFunc func(ctx context.Context) error
}

func (w *workerJob) Execute(ctx context.Context) error {
	return w.jobFunc(ctx)
}

func (w *workerJob) ID() string {
	return w.id
}

func (w *workerJob) Timeout() time.Duration {
	return 5 * time.Minute // Default timeout
}

// Session represents a minimal session type for the adapter
type Session struct {
	ID        string
	Metadata  map[string]string
	CreatedAt time.Time
	UpdatedAt time.Time
	State     string
}

// SessionManagerAdapter provides compatibility for SessionManager usage
type SessionManagerAdapter struct {
	sessions sync.Map
	log      zerolog.Logger
}

// NewSessionManagerAdapter creates a new session manager adapter
func NewSessionManagerAdapter(log zerolog.Logger) *SessionManagerAdapter {
	return &SessionManagerAdapter{
		log: log,
	}
}

// CreateSession creates a new session
func (s *SessionManagerAdapter) CreateSession(ctx context.Context, metadata map[string]string) (string, error) {
	sessionID := fmt.Sprintf("session-%d", time.Now().UnixNano())
	session := &Session{
		ID:        sessionID,
		Metadata:  metadata,
		CreatedAt: time.Now(),
		State:     "active",
	}
	s.sessions.Store(sessionID, session)
	return sessionID, nil
}

// GetSession retrieves a session
func (s *SessionManagerAdapter) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	if val, ok := s.sessions.Load(sessionID); ok {
		return val.(*Session), nil
	}
	return nil, fmt.Errorf("session %s not found", sessionID)
}

// UpdateSession updates a session
func (s *SessionManagerAdapter) UpdateSession(ctx context.Context, sessionID string, updates map[string]interface{}) error {
	if val, ok := s.sessions.Load(sessionID); ok {
		session := val.(*Session)
		// Apply updates (simplified)
		if state, ok := updates["state"].(string); ok {
			session.State = state
		}
		session.UpdatedAt = time.Now()
		s.sessions.Store(sessionID, session)
		return nil
	}
	return fmt.Errorf("session %s not found", sessionID)
}

// DeleteSession deletes a session
func (s *SessionManagerAdapter) DeleteSession(ctx context.Context, sessionID string) error {
	s.sessions.Delete(sessionID)
	return nil
}

// ListSessions lists all sessions
func (s *SessionManagerAdapter) ListSessions(ctx context.Context) ([]*Session, error) {
	var sessions []*Session
	s.sessions.Range(func(key, value interface{}) bool {
		sessions = append(sessions, value.(*Session))
		return true
	})
	return sessions, nil
}