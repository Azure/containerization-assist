// Package retry provides retry middleware for the sampling infrastructure
package retry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/sampling"
)

// Config defines retry behavior
type Config struct {
	MaxAttempts     int
	InitialDelay    time.Duration
	MaxDelay        time.Duration
	BackoffFactor   float64
	RetryableErrors func(error) bool
}

// DefaultConfig returns a sensible default retry configuration
func DefaultConfig() Config {
	return Config{
		MaxAttempts:     3,
		InitialDelay:    1 * time.Second,
		MaxDelay:        30 * time.Second,
		BackoffFactor:   2.0,
		RetryableErrors: defaultRetryableErrors,
	}
}

// defaultRetryableErrors determines if an error is retryable
func defaultRetryableErrors(err error) bool {
	// Add your error checking logic here
	// For now, retry on any error except context cancellation
	return !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)
}

// RetrySampler wraps a UnifiedSampler with retry logic
type RetrySampler struct {
	next   sampling.UnifiedSampler
	config Config
	logger *slog.Logger
}

// New creates a new retry middleware wrapper
func New(next sampling.UnifiedSampler, config Config, logger *slog.Logger) *RetrySampler {
	return &RetrySampler{
		next:   next,
		config: config,
		logger: logger,
	}
}

// Sample wraps the Sample method with retry logic
func (r *RetrySampler) Sample(ctx context.Context, req sampling.Request) (sampling.Response, error) {
	var resp sampling.Response
	err := r.retry(ctx, "Sample", func() error {
		var err error
		resp, err = r.next.Sample(ctx, req)
		return err
	})
	return resp, err
}

// Stream wraps the Stream method with retry logic
// Note: Stream operations are not retried as they are stateful
func (r *RetrySampler) Stream(ctx context.Context, req sampling.Request) (<-chan sampling.StreamChunk, error) {
	// Streaming is not retryable due to its stateful nature
	return r.next.Stream(ctx, req)
}

// AnalyzeDockerfile wraps the method with retry logic
func (r *RetrySampler) AnalyzeDockerfile(ctx context.Context, content string) (*sampling.DockerfileAnalysis, error) {
	var result *sampling.DockerfileAnalysis
	err := r.retry(ctx, "AnalyzeDockerfile", func() error {
		var err error
		result, err = r.next.AnalyzeDockerfile(ctx, content)
		return err
	})
	return result, err
}

// AnalyzeKubernetesManifest wraps the method with retry logic
func (r *RetrySampler) AnalyzeKubernetesManifest(ctx context.Context, content string) (*sampling.ManifestAnalysis, error) {
	var result *sampling.ManifestAnalysis
	err := r.retry(ctx, "AnalyzeKubernetesManifest", func() error {
		var err error
		result, err = r.next.AnalyzeKubernetesManifest(ctx, content)
		return err
	})
	return result, err
}

// AnalyzeSecurityScan wraps the method with retry logic
func (r *RetrySampler) AnalyzeSecurityScan(ctx context.Context, scanResults string) (*sampling.SecurityAnalysis, error) {
	var result *sampling.SecurityAnalysis
	err := r.retry(ctx, "AnalyzeSecurityScan", func() error {
		var err error
		result, err = r.next.AnalyzeSecurityScan(ctx, scanResults)
		return err
	})
	return result, err
}

// FixDockerfile wraps the method with retry logic
func (r *RetrySampler) FixDockerfile(ctx context.Context, content string, issues []string) (*sampling.DockerfileFix, error) {
	var result *sampling.DockerfileFix
	err := r.retry(ctx, "FixDockerfile", func() error {
		var err error
		result, err = r.next.FixDockerfile(ctx, content, issues)
		return err
	})
	return result, err
}

// FixKubernetesManifest wraps the method with retry logic
func (r *RetrySampler) FixKubernetesManifest(ctx context.Context, content string, issues []string) (*sampling.ManifestFix, error) {
	var result *sampling.ManifestFix
	err := r.retry(ctx, "FixKubernetesManifest", func() error {
		var err error
		result, err = r.next.FixKubernetesManifest(ctx, content, issues)
		return err
	})
	return result, err
}

// retry implements exponential backoff retry logic
func (r *RetrySampler) retry(ctx context.Context, operation string, fn func() error) error {
	delay := r.config.InitialDelay

	for attempt := 1; attempt <= r.config.MaxAttempts; attempt++ {
		// Check context before attempting
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("context cancelled before attempt %d: %w", attempt, err)
		}

		err := fn()
		if err == nil {
			return nil
		}

		// Check if error is retryable
		if !r.config.RetryableErrors(err) {
			r.logger.Debug("Error is not retryable",
				"operation", operation,
				"attempt", attempt,
				"error", err)
			return err
		}

		// Don't retry if we've exhausted attempts
		if attempt >= r.config.MaxAttempts {
			r.logger.Warn("Max retry attempts reached",
				"operation", operation,
				"attempts", attempt,
				"error", err)
			return fmt.Errorf("max retry attempts (%d) reached: %w", r.config.MaxAttempts, err)
		}

		r.logger.Info("Retrying operation",
			"operation", operation,
			"attempt", attempt,
			"delay", delay,
			"error", err)

		// Wait with exponential backoff
		select {
		case <-time.After(delay):
			// Calculate next delay
			delay = time.Duration(float64(delay) * r.config.BackoffFactor)
			if delay > r.config.MaxDelay {
				delay = r.config.MaxDelay
			}
		case <-ctx.Done():
			return fmt.Errorf("context cancelled during retry delay: %w", ctx.Err())
		}
	}

	// This should never be reached due to the check above, but just in case
	return errors.New("retry loop exited unexpectedly")
}

// Ensure RetrySampler implements UnifiedSampler
var _ sampling.UnifiedSampler = (*RetrySampler)(nil)
