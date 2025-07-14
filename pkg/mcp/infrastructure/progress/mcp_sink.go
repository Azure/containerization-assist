package progress

import (
	"context"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/progress"
	"github.com/mark3labs/mcp-go/server"
)

// MCPServerInterface defines the interface we need from MCP server
type MCPServerInterface interface {
	SendNotificationToClient(ctx context.Context, method string, params map[string]any) error
}

// MCPSink publishes rich progress updates to the connected MCP client.
type MCPSink struct {
	*baseSink
	srv   MCPServerInterface
	token interface{}
}

// NewMCPSink creates a new MCP progress sink with enhanced capabilities.
func NewMCPSink(srv *server.MCPServer, token interface{}, lg *slog.Logger) *MCPSink {
	return &MCPSink{
		baseSink: newBaseSink(lg, "mcp-sink"),
		srv:      srv,
		token:    token,
	}
}

// Publish sends a rich progress update via MCP notification.
func (s *MCPSink) Publish(ctx context.Context, u progress.Update) error {
	if s.srv == nil {
		s.logger.Debug("No MCP server in context; skipping progress publish")
		return nil
	}

	// Use base sink to build payload
	basePayload := s.buildBasePayload(u)
	basePayload["progressToken"] = s.token

	// Throttle heartbeat noise to once every 2s
	if s.shouldThrottleHeartbeat(u, 2*time.Second) {
		return nil
	}

	// Convert to map[string]any for MCP server interface
	payload := make(map[string]any)
	for k, v := range basePayload {
		payload[k] = v
	}

	// Send notification
	if err := s.srv.SendNotificationToClient(ctx, "notifications/progress", payload); err != nil {
		s.logger.Warn("Failed to send progress notification", "err", err)
		return err
	}

	// Use base sink debug logging
	s.logDebugInfo(u, "MCP")

	return nil
}

// Close implements progress.Sink.
func (s *MCPSink) Close() error {
	return nil
}

// Assert interface compliance.
var _ progress.Sink = (*MCPSink)(nil)
