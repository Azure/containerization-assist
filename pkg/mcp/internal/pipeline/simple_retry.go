package pipeline

import (
	"context"
	"math"
	"math/rand"
	"time"
)

// SimpleRetry provides basic retry logic with exponential backoff
type SimpleRetry struct {
	MaxRetries int
	BaseDelay  time.Duration
	MaxDelay   time.Duration
	Multiplier float64
}

// NewSimpleRetry creates basic retry handler
func NewSimpleRetry() *SimpleRetry {
	return &SimpleRetry{
		MaxRetries: 3,
		BaseDelay:  100 * time.Millisecond,
		MaxDelay:   5 * time.Second,
		Multiplier: 2.0,
	}
}

// Execute runs operation with simple retry logic
func (r *SimpleRetry) Execute(ctx context.Context, operation func() error) error {
	var lastErr error

	for attempt := 0; attempt <= r.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := time.Duration(float64(r.BaseDelay) *
				math.Pow(r.Multiplier, float64(attempt-1)))
			if delay > r.MaxDelay {
				delay = r.MaxDelay
			}

			// Add jitter (±25%)
			jitter := time.Duration(rand.Float64() * float64(delay) * 0.5)
			delay = delay + jitter - time.Duration(float64(delay)*0.25)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		if err := operation(); err != nil {
			lastErr = err
			continue
		}

		return nil // Success
	}

	return lastErr
}
