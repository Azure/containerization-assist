package progress

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/progress"
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
func (s *CLISink) Publish(ctx context.Context, u progress.Update) error {
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

	// Enhanced status indicator with retries
	statusIcon := s.getStatusIcon(u)

	// Enhanced message with step name and sub-step info
	message := s.formatEnhancedMessage(u)

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

	s.logger.Debug("Enhanced CLI progress update",
		"step", u.Step,
		"total", u.Total,
		"percentage", u.Percentage,
		"step_name", s.getStepName(u),
		"substep_name", s.getSubstepName(u),
		"status", u.Status,
		"message", u.Message)

	return nil
}

// getStatusIcon returns an enhanced status icon based on the update
func (s *CLISink) getStatusIcon(u progress.Update) string {
	switch u.Status {
	case "completed":
		return "✅"
	case "failed":
		return "❌"
	case "retrying":
		if attempt, ok := u.UserMeta["attempt"].(int); ok {
			return fmt.Sprintf("🔄(%d)", attempt)
		}
		return "🔄"
	case "started":
		return "🚀"
	case "running":
		return "⚡"
	case "generating": // For LLM token generation
		return "🧠"
	default:
		return "▶️"
	}
}

// formatEnhancedMessage creates a rich message with step and sub-step information
func (s *CLISink) formatEnhancedMessage(u progress.Update) string {
	stepName := s.getStepName(u)
	substepName := s.getSubstepName(u)

	// If we have a step name, use it instead of generic message
	if stepName != "" {
		message := stepName

		// Add sub-step information if available
		if substepName != "" {
			message = fmt.Sprintf("%s (%s)", message, substepName)
		}

		// Add retry information
		if attempt, ok := u.UserMeta["attempt"].(int); ok && attempt > 1 {
			message = fmt.Sprintf("%s - Attempt %d", message, attempt)
		}

		// Add additional context for specific statuses
		switch u.Status {
		case "failed":
			if errorMsg, ok := u.UserMeta["error"].(string); ok && len(errorMsg) > 0 {
				// Show first 50 chars of error
				if len(errorMsg) > 50 {
					errorMsg = errorMsg[:47] + "..."
				}
				message = fmt.Sprintf("%s - Error: %s", message, errorMsg)
			}
		case "generating": // For LLM operations
			if tokensGenerated, ok := u.UserMeta["tokens_generated"].(int); ok {
				if estimatedTotal, ok := u.UserMeta["estimated_total"].(int); ok {
					message = fmt.Sprintf("AI generating tokens: %d/%d", tokensGenerated, estimatedTotal)
				}
			}
		}

		return message
	}

	// Fallback to original message
	return u.Message
}

// getStepName extracts step_name from metadata
func (s *CLISink) getStepName(u progress.Update) string {
	if stepName, ok := u.UserMeta["step_name"].(string); ok {
		return stepName
	}
	return ""
}

// getSubstepName extracts substep_name from metadata
func (s *CLISink) getSubstepName(u progress.Update) string {
	if substepName, ok := u.UserMeta["substep_name"].(string); ok {
		return substepName
	}
	return ""
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
