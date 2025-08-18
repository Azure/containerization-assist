package messaging

import (
	"context"
	"log/slog"
	"time"

	"github.com/Azure/containerization-assist/pkg/mcp/api"
	"github.com/mark3labs/mcp-go/server"
)

// MCPDirectEmitter sends progress directly via MCP protocol without intermediate layers
type MCPDirectEmitter struct {
	server *server.MCPServer
	token  interface{}
	logger *slog.Logger

	// Rate limiting to prevent spam
	lastSent    time.Time
	minInterval time.Duration
}

// NewMCPDirectEmitter creates a new direct MCP progress emitter
func NewMCPDirectEmitter(srv *server.MCPServer, token interface{}, logger *slog.Logger) *MCPDirectEmitter {
	return &MCPDirectEmitter{
		server:      srv,
		token:       token,
		logger:      logger.With("component", "mcp_direct_progress"),
		minInterval: 100 * time.Millisecond, // Prevent notification spam
	}
}

// Emit sends a simple progress update
func (e *MCPDirectEmitter) Emit(ctx context.Context, stage string, percent int, message string) error {
	return e.EmitDetailed(ctx, api.ProgressUpdate{
		Stage:      stage,
		Percentage: percent,
		Message:    message,
		Status:     "running",
	})
}

// EmitDetailed sends a detailed progress update with all fields
func (e *MCPDirectEmitter) EmitDetailed(ctx context.Context, update api.ProgressUpdate) error {
	// Rate limiting to prevent overwhelming the client
	if time.Since(e.lastSent) < e.minInterval && update.Status == "running" {
		return nil
	}

	if e.server == nil {
		return nil // No-op if no server
	}

	// Build MCP-compliant progress notification payload
	payload := map[string]any{
		"progressToken": e.token,
		"percentage":    update.Percentage,
		"message":       update.Message,
		"status":        update.Status,
	}

	// Add optional fields only if they have values
	if update.Stage != "" {
		payload["stage"] = update.Stage
	}
	if update.Step > 0 {
		payload["step"] = update.Step
		payload["total"] = update.Total
	}
	if update.ETA > 0 {
		payload["eta_ms"] = update.ETA.Milliseconds()
	}
	if update.TraceID != "" {
		payload["traceId"] = update.TraceID
	}

	// Add metadata if present
	if update.Metadata != nil && len(update.Metadata) > 0 {
		payload["metadata"] = update.Metadata
	}

	e.lastSent = time.Now()

	// Send the notification directly via MCP protocol
	if err := e.server.SendNotificationToClient(ctx, "notifications/progress", payload); err != nil {
		e.logger.Debug("Failed to send progress notification",
			"error", err,
			"stage", update.Stage,
			"percentage", update.Percentage,
		)
		return err
	}

	// Log successful progress updates at debug level
	e.logger.Debug("Progress notification sent",
		"stage", update.Stage,
		"percentage", update.Percentage,
		"status", update.Status,
	)

	return nil
}

// Close sends a final completion notification
func (e *MCPDirectEmitter) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Send final 100% complete notification
	return e.EmitDetailed(ctx, api.ProgressUpdate{
		Stage:      "complete",
		Percentage: 100,
		Message:    "Workflow completed successfully",
		Status:     "completed",
	})
}

// Ensure MCPDirectEmitter implements ProgressEmitter
var _ api.ProgressEmitter = (*MCPDirectEmitter)(nil)
