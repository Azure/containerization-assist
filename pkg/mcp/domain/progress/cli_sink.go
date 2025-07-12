package progress

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// CLISink implements console progress reporting with ANSI progress bars.
type CLISink struct {
	logger    *slog.Logger
	barWidth  int
	spinner   []string
	spinIndex int
}

// NewCLISink creates a new CLI progress sink.
func NewCLISink(logger *slog.Logger) *CLISink {
	return &CLISink{
		logger:   logger.With("component", "cli-sink"),
		barWidth: 40,
		spinner:  []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
	}
}

// Publish outputs a progress update to the console.
func (s *CLISink) Publish(ctx context.Context, u Update) error {
	// Create progress bar
	bar := s.createProgressBar(u.Percentage)

	// Get spinner for heartbeat updates
	spinChar := ""
	if kind, ok := u.UserMeta["kind"].(string); ok && kind == "heartbeat" {
		spinChar = s.spinner[s.spinIndex%len(s.spinner)]
		s.spinIndex++
	}

	// Format ETA
	etaStr := ""
	if u.ETA > 0 {
		etaStr = fmt.Sprintf(" ETA: %s", u.ETA.Round(time.Second))
	}

	// Status indicator
	statusIcon := "▶"
	switch u.Status {
	case "completed":
		statusIcon = "✓"
	case "failed":
		statusIcon = "✗"
	case "running":
		statusIcon = "▶"
	}

	// Construct output line
	line := fmt.Sprintf("\r%s%s %s [%d/%d] %s%s%s",
		statusIcon,
		spinChar,
		bar,
		u.Step,
		u.Total,
		u.Message,
		etaStr,
		strings.Repeat(" ", 10), // Clear any remaining characters
	)

	// Print to stdout (not logger to avoid formatting)
	fmt.Print(line)

	// Add newline for completed status
	if u.Status == "completed" || u.Status == "failed" {
		fmt.Println()
	}

	s.logger.Debug("CLI progress update",
		"step", u.Step,
		"total", u.Total,
		"percentage", u.Percentage,
		"message", u.Message)

	return nil
}

// Close cleans up the sink.
func (s *CLISink) Close() error {
	// Ensure we end with a newline
	fmt.Println()
	return nil
}

// createProgressBar creates an ANSI progress bar.
func (s *CLISink) createProgressBar(percentage int) string {
	if percentage < 0 {
		percentage = 0
	}
	if percentage > 100 {
		percentage = 100
	}

	filled := int(float64(percentage) / 100.0 * float64(s.barWidth))
	bar := strings.Repeat("█", filled) + strings.Repeat("░", s.barWidth-filled)

	return fmt.Sprintf("[%s] %3d%%", bar, percentage)
}
