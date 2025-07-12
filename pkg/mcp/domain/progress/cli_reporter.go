// Package progress provides CLI-based progress reporting
package progress

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/briandowns/spinner"
)

// CLIReporter implements progress reporting for CLI environments
type CLIReporter struct {
	spinner   *spinner.Spinner
	isCI      bool
	logger    *slog.Logger
	startTime time.Time
	current   int
	total     int
	mu        sync.Mutex // Only protects spinner operations
}

// NewCLIReporter creates a new CLI progress reporter
func NewCLIReporter(ctx context.Context, totalSteps int, logger *slog.Logger) Reporter {
	r := &CLIReporter{
		isCI:      os.Getenv("CI") == "true",
		logger:    logger,
		startTime: time.Now(),
		total:     totalSteps,
	}

	// Only use spinner in non-CI environments
	if !r.isCI {
		r.spinner = spinner.New(spinner.CharSets[14], 100*time.Millisecond)
		r.spinner.Prefix = "Progress: "
		r.spinner.Color("cyan", "bold")
	}

	return r
}

// Begin starts the progress tracking
func (r *CLIReporter) Begin(message string) error {
	if r.isCI {
		fmt.Printf("[BEGIN] %s\n", message)
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.spinner != nil {
		r.spinner.Suffix = fmt.Sprintf(" %s", message)
		r.spinner.Start()
	}

	return nil
}

// Update advances the progress
func (r *CLIReporter) Update(step, total int, message string) error {
	r.current = step
	percentage := int((float64(step) / float64(total)) * 100)

	if r.isCI {
		fmt.Printf("[%d/%d] [%d%%] %s\n", step, total, percentage, message)
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.spinner != nil {
		progressBar := r.renderProgressBar(percentage)
		r.spinner.Suffix = fmt.Sprintf(" %s %s", progressBar, message)
	}

	return nil
}

// Complete finishes the progress tracking
func (r *CLIReporter) Complete(message string) error {
	duration := time.Since(r.startTime)
	finalMsg := fmt.Sprintf("%s (completed in %s)", message, duration.Round(time.Second))

	if r.isCI {
		fmt.Printf("[COMPLETE] %s\n", finalMsg)
		return nil
	}

	r.mu.Lock()
	if r.spinner != nil {
		r.spinner.Stop()
	}
	r.mu.Unlock()

	fmt.Printf("✅ %s\n", finalMsg)
	return nil
}

// Close cleans up resources
func (r *CLIReporter) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.spinner != nil {
		r.spinner.Stop()
	}
	return nil
}

// renderProgressBar creates a visual progress bar
func (r *CLIReporter) renderProgressBar(percentage int) string {
	const barWidth = 20
	filled := (percentage * barWidth) / 100
	empty := barWidth - filled

	return fmt.Sprintf("[%s%s]",
		repeatChar('█', filled),
		repeatChar('░', empty))
}
