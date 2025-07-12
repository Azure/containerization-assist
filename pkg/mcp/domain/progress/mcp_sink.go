package progress

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/mark3labs/mcp-go/server"
)

// MCPSink implements progress reporting via MCP notifications.
type MCPSink struct {
	server        *server.MCPServer
	progressToken interface{}
	logger        *slog.Logger
}

// NewMCPSink creates a new MCP progress sink.
func NewMCPSink(server *server.MCPServer, progressToken interface{}, logger *slog.Logger) *MCPSink {
	return &MCPSink{
		server:        server,
		progressToken: progressToken,
		logger:        logger.With("component", "mcp-sink"),
	}
}

// Publish sends a progress update via MCP notification.
func (s *MCPSink) Publish(ctx context.Context, u Update) error {
	if s.server == nil {
		s.logger.Debug("No MCP server available for progress notification")
		return nil
	}

	// Construct MCP notification payload
	params := map[string]interface{}{
		"progressToken": s.progressToken,
		"progress":      u.Step,
		"total":         u.Total,
		"message":       fmt.Sprintf("[%d%%] %s", u.Percentage, u.Message),
		"metadata": map[string]interface{}{
			"step":       u.Step,
			"total":      u.Total,
			"percentage": u.Percentage,
			"message":    u.Message,
			"status":     u.Status,
			"trace_id":   u.TraceID,
			"started_at": u.StartedAt,
		},
	}

	// Add ETA if available
	if u.ETA > 0 {
		params["eta_ms"] = u.ETA.Milliseconds()
	}

	// Add user metadata
	if u.UserMeta != nil {
		if metadata, ok := params["metadata"].(map[string]interface{}); ok {
			for k, v := range u.UserMeta {
				metadata[k] = v
			}
		}
	}

	// Send notification
	err := s.server.SendNotificationToClient(ctx, "notifications/progress", params)
	if err != nil {
		s.logger.Warn("Failed to send MCP progress notification", "error", err)
		return err
	}

	s.logger.Debug("Sent MCP progress notification",
		"step", u.Step,
		"total", u.Total,
		"percentage", u.Percentage)

	return nil
}

// Close cleans up the sink.
func (s *MCPSink) Close() error {
	// No cleanup needed for MCP sink
	return nil
}
