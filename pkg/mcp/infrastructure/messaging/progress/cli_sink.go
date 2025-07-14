package progress

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/domain/progress"
)

// CLISink implements console progress reporting with ANSI progress bars.
type CLISink struct {
	*baseSink
	barWidth  int
	spinner   []string
	spinIndex int
}

// NewCLISink creates a new CLI progress sink.
func NewCLISink(logger *slog.Logger) *CLISink {
	return &CLISink{
		baseSink: newBaseSink(logger, "cli-sink"),
		barWidth: 40,
		spinner:  []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
	}
}

// Publish outputs a progress update to the console.
func (s *CLISink) Publish(ctx context.Context, u progress.Update) error {
	// Create progress bar
	bar := s.createProgressBar(u.Percentage)

	// Get spinner for heartbeat updates
	spinChar := ""
	if kind, ok := u.UserMeta["kind"].(string); ok && kind == "heartbeat" {
		spinChar = s.spinner[s.spinIndex%len(s.spinner)]
		s.spinIndex++
	}

	// Format ETA using base sink
	etaStr := ""
	if etaFormatted := s.formatETA(u.ETA); etaFormatted != "" {
		etaStr = fmt.Sprintf(" %s", etaFormatted)
	}

	// Enhanced status indicator with retries
	statusInfo := s.getStatusInfo(u)
	statusIcon := statusInfo.Icon

	// Enhanced message with step name and sub-step info
	message := s.buildEnhancedMessage(u)

	// Construct output line with rich information
	line := fmt.Sprintf("\r%s%s %s [%d%%] %s%s%s",
		statusIcon,
		spinChar,
		bar,
		u.Percentage,
		message,
		etaStr,
		strings.Repeat(" ", 10), // Clear any remaining characters
	)

	// Print to stdout (not logger to avoid formatting)
	fmt.Print(line)

	// Add newline for completed status or errors
	if u.Status == "completed" || u.Status == "failed" || strings.Contains(u.Status, "retry") {
		fmt.Println()
	}

	// Use base sink debug logging
	s.logDebugInfo(u, "CLI")

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
