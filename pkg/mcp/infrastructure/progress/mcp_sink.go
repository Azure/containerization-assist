package progress

import (
	"context"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/progress"
	"github.com/mark3labs/mcp-go/server"
)

// MCPSink publishes rich progress updates to the connected MCP client.
type MCPSink struct {
	srv           *server.MCPServer
	token         interface{}
	logger        *slog.Logger
	lastHeartbeat time.Time
}

// NewMCPSink creates a new MCP progress sink with enhanced capabilities.
func NewMCPSink(srv *server.MCPServer, token interface{}, lg *slog.Logger) *MCPSink {
	return &MCPSink{
		srv:    srv,
		token:  token,
		logger: lg.With("component", "mcp-sink"),
	}
}

// Publish sends a rich progress update via MCP notification.
func (s *MCPSink) Publish(ctx context.Context, u progress.Update) error {
	if s.srv == nil {
		s.logger.Debug("No MCP server in context; skipping progress publish")
		return nil
	}

	payload := map[string]interface{}{
		"progressToken": s.token,
		"step":          u.Step,
		"total":         u.Total,
		"percentage":    u.Percentage, // TOP-LEVEL for AI consumption
		"status":        u.Status,     // TOP-LEVEL for AI consumption
		"message":       u.Message,
		"trace_id":      u.TraceID,
		"started_at":    u.StartedAt,
		// Backward compatibility - keep metadata block
		"metadata": map[string]interface{}{
			"step":       u.Step,
			"total":      u.Total,
			"percentage": u.Percentage,
			"status":     u.Status,
			"eta_ms":     u.ETA.Milliseconds(),
			"user_meta":  u.UserMeta,
		},
	}

	// Enhanced fields for rich AI experience
	if u.ETA > 0 {
		payload["eta_ms"] = u.ETA.Milliseconds()
	}
	if name, ok := u.UserMeta["step_name"].(string); ok && name != "" {
		payload["step_name"] = name
	}
	if sub, ok := u.UserMeta["substep_name"].(string); ok && sub != "" {
		payload["substep_name"] = sub
	}
	if canAbort, ok := u.UserMeta["can_abort"].(bool); ok {
		payload["can_abort"] = canAbort
	}

	// Throttle heartbeat noise to once every 2s
	if kind, _ := u.UserMeta["kind"].(string); kind == "heartbeat" {
		if time.Since(s.lastHeartbeat) < 2*time.Second {
			s.logger.Debug("Throttling heartbeat update")
			return nil
		}
		s.lastHeartbeat = time.Now()
	}

	// Send notification
	if err := s.srv.SendNotificationToClient(ctx, "notifications/progress", payload); err != nil {
		s.logger.Warn("Failed to send progress notification", "err", err)
		return err
	}

	s.logger.Debug("Sent enhanced MCP progress notification",
		"step", u.Step,
		"total", u.Total,
		"percentage", u.Percentage,
		"status", u.Status,
		"step_name", payload["step_name"],
		"substep_name", payload["substep_name"])

	return nil
}

// Close implements progress.Sink.
func (s *MCPSink) Close() error {
	return nil
}

// Assert interface compliance.
var _ progress.Sink = (*MCPSink)(nil)
