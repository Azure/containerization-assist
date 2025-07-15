package progress

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/api"
)

// CLIDirectEmitter outputs progress to console with simple formatting
type CLIDirectEmitter struct {
	logger      *slog.Logger
	lastPercent int
	lastStage   string
	startTime   time.Time
}

// NewCLIDirectEmitter creates a new CLI progress emitter
func NewCLIDirectEmitter(logger *slog.Logger) *CLIDirectEmitter {
	return &CLIDirectEmitter{
		logger:      logger.With("component", "cli_progress"),
		lastPercent: -1,
		startTime:   time.Now(),
	}
}

// Emit sends a simple progress update to the console
func (e *CLIDirectEmitter) Emit(ctx context.Context, stage string, percent int, message string) error {
	// Only log if percentage changed significantly (5% increments) or stage changed
	if percent < e.lastPercent+5 && percent < 100 && stage == e.lastStage {
		return nil
	}

	// Format progress bar
	progressBar := e.formatProgressBar(percent)
	elapsed := time.Since(e.startTime).Round(time.Second)

	// Log with structured format
	e.logger.Info("Progress",
		"stage", stage,
		"percent", percent,
		"progress", progressBar,
		"message", message,
		"elapsed", elapsed.String(),
	)

	e.lastPercent = percent
	e.lastStage = stage
	return nil
}

// EmitDetailed sends a detailed progress update to the console
func (e *CLIDirectEmitter) EmitDetailed(ctx context.Context, update api.ProgressUpdate) error {
	// Handle different status types with appropriate formatting
	switch update.Status {
	case "failed", "error":
		e.logger.Error("Progress failed",
			"stage", update.Stage,
			"message", update.Message,
			"percent", update.Percentage,
		)
		return nil
	case "completed":
		elapsed := time.Since(e.startTime).Round(time.Second)
		e.logger.Info("Progress completed",
			"stage", update.Stage,
			"message", update.Message,
			"total_time", elapsed.String(),
		)
		return nil
	case "warning":
		e.logger.Warn("Progress warning",
			"stage", update.Stage,
			"message", update.Message,
			"percent", update.Percentage,
		)
		return nil
	default:
		// Regular progress update
		return e.Emit(ctx, update.Stage, update.Percentage, update.Message)
	}
}

// formatProgressBar creates a simple ASCII progress bar
func (e *CLIDirectEmitter) formatProgressBar(percent int) string {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}

	barWidth := 20
	filled := int(float64(barWidth) * float64(percent) / 100.0)
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	return fmt.Sprintf("[%s] %3d%%", bar, percent)
}

// Close logs the final summary
func (e *CLIDirectEmitter) Close() error {
	totalTime := time.Since(e.startTime).Round(time.Second)
	e.logger.Info("Workflow finished",
		"total_time", totalTime.String(),
		"final_percent", e.lastPercent,
	)
	return nil
}

// Ensure CLIDirectEmitter implements ProgressEmitter
var _ api.ProgressEmitter = (*CLIDirectEmitter)(nil)
