package shared

import (
	"context"
	"sync"
	"time"
)

// Lifecycle manages the lifecycle of goroutines
type Lifecycle struct {
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	shutdown bool
	mu       sync.RWMutex
}

// NewLifecycle creates a new lifecycle manager
func NewLifecycle() *Lifecycle {
	ctx, cancel := context.WithCancel(context.Background())
	return &Lifecycle{
		ctx:    ctx,
		cancel: cancel,
	}
}

// Go starts a goroutine managed by this lifecycle
func (l *Lifecycle) Go(fn func(context.Context)) error {
	l.mu.RLock()
	if l.shutdown {
		l.mu.RUnlock()
		return context.Canceled
	}
	l.mu.RUnlock()

	l.wg.Add(1)
	go func() {
		defer l.wg.Done()
		fn(l.ctx)
	}()

	return nil
}

// Shutdown shuts down the lifecycle and waits for all goroutines to complete
func (l *Lifecycle) Shutdown(timeout time.Duration) error {
	l.mu.Lock()
	if l.shutdown {
		l.mu.Unlock()
		return nil
	}
	l.shutdown = true
	l.cancel()
	l.mu.Unlock()

	// Wait for all goroutines to complete with timeout
	done := make(chan struct{})
	go func() {
		l.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		return context.DeadlineExceeded
	}
}
