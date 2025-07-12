// Package progress provides MCP-based progress reporting
package progress

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// MCPReporter implements progress reporting via MCP notifications
type MCPReporter struct {
	ctx           context.Context
	server        NotificationSender
	progressToken interface{}
	logger        *slog.Logger
	startTime     time.Time
	current       int
	total         int
}

// NewMCPReporter creates a new MCP progress reporter
func NewMCPReporter(ctx context.Context, server NotificationSender, progressToken interface{}, totalSteps int, logger *slog.Logger) Reporter {
	return &MCPReporter{
		ctx:           ctx,
		server:        server,
		progressToken: progressToken,
		logger:        logger,
		startTime:     time.Now(),
		total:         totalSteps,
	}
}

// Begin starts the progress tracking
func (r *MCPReporter) Begin(message string) error {
	params := map[string]interface{}{
		"progressToken": r.progressToken,
		"progress":      float64(0),
		"total":         float64(r.total),
		"message":       message,
	}

	return r.server.SendNotificationToClient(r.ctx, "notifications/progress", params)
}

// Update advances the progress
func (r *MCPReporter) Update(step, total int, message string) error {
	r.current = step
	percentage := int((float64(step) / float64(total)) * 100)
	formattedMsg := fmt.Sprintf("[%d%%] %s", percentage, message)

	params := map[string]interface{}{
		"progressToken": r.progressToken,
		"progress":      float64(step),
		"total":         float64(total),
		"message":       formattedMsg,
	}

	return r.server.SendNotificationToClient(r.ctx, "notifications/progress", params)
}

// Complete finishes the progress tracking
func (r *MCPReporter) Complete(message string) error {
	duration := time.Since(r.startTime)
	finalMsg := fmt.Sprintf("%s (completed in %s)", message, duration.Round(time.Second))

	params := map[string]interface{}{
		"progressToken": r.progressToken,
		"progress":      float64(r.total),
		"total":         float64(r.total),
		"message":       finalMsg,
	}

	return r.server.SendNotificationToClient(r.ctx, "notifications/progress", params)
}

// Close cleans up resources
func (r *MCPReporter) Close() error {
	// Nothing to clean up for MCP reporter
	return nil
}
