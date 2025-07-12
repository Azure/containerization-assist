// Package monitoring handles server monitoring and diagnostics
package monitoring

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/lifecycle"
	"github.com/Azure/container-kit/pkg/mcp/application/session"
	"github.com/Azure/container-kit/pkg/mcp/application/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

// Stats represents server statistics
type Stats struct {
	Uptime        time.Duration `json:"uptime"`
	State         string        `json:"state"`
	StartTime     time.Time     `json:"start_time"`
	Transport     string        `json:"transport"`
	Sessions      SessionStats  `json:"sessions"`
	Resources     ResourceStats `json:"resources"`
	Memory        MemoryStats   `json:"memory"`
	Tools         []string      `json:"tools"`
	ServerVersion string        `json:"server_version"`
}

// SessionStats represents session statistics
type SessionStats struct {
	Active     int `json:"active"`
	Total      int `json:"total"`
	MaxAllowed int `json:"max_allowed"`
}

// ResourceStats represents resource statistics
type ResourceStats struct {
	Count       int       `json:"count"`
	LastCleanup time.Time `json:"last_cleanup"`
}

// MemoryStats represents memory statistics
type MemoryStats struct {
	Allocated      uint64 `json:"allocated_mb"`
	TotalAllocated uint64 `json:"total_allocated_mb"`
	System         uint64 `json:"system_mb"`
	GCCount        uint32 `json:"gc_count"`
}

// Monitor handles server monitoring and diagnostics
type Monitor interface {
	GetStats() (*Stats, error)
	RegisterDiagnosticTools(transport transport.MCPTransport) error
	SetSessionManager(sm session.SessionManager)
	SetLifecycleManager(lm lifecycle.Manager)
	SetResourceStore(rs ResourceStatsProvider)
}

// monitorImpl implements the server monitor
type monitorImpl struct {
	logger           *slog.Logger
	serverVersion    string
	transportType    string
	sessionManager   session.SessionManager
	lifecycleManager lifecycle.Manager
	resourceStore    ResourceStatsProvider
	registeredTools  []string
}

// ResourceStatsProvider provides resource statistics
type ResourceStatsProvider interface {
	GetResourceCount() int
	GetLastCleanupTime() time.Time
}

// NewMonitor creates a new server monitor
func NewMonitor(serverVersion, transportType string, logger *slog.Logger) Monitor {
	return &monitorImpl{
		logger:        logger.With("component", "server_monitor"),
		serverVersion: serverVersion,
		transportType: transportType,
	}
}

// SetSessionManager sets the session manager for monitoring
func (m *monitorImpl) SetSessionManager(sm session.SessionManager) {
	m.sessionManager = sm
}

// SetLifecycleManager sets the lifecycle manager for monitoring
func (m *monitorImpl) SetLifecycleManager(lm lifecycle.Manager) {
	m.lifecycleManager = lm
}

// SetResourceStore sets the resource store for monitoring
func (m *monitorImpl) SetResourceStore(rs ResourceStatsProvider) {
	m.resourceStore = rs
}

// GetStats returns current server statistics
func (m *monitorImpl) GetStats() (*Stats, error) {
	stats := &Stats{
		Transport:     m.transportType,
		ServerVersion: m.serverVersion,
		Tools:         m.registeredTools,
	}

	// Get lifecycle status
	if m.lifecycleManager != nil {
		status := m.lifecycleManager.GetStatus()
		stats.State = status.State.String()
		stats.StartTime = status.StartTime
		stats.Uptime = status.Uptime
	}

	// Get session stats
	if m.sessionManager != nil {
		sessionStats, err := m.sessionManager.GetStats()
		if err == nil {
			stats.Sessions = SessionStats{
				Active:     sessionStats.ActiveSessions,
				Total:      sessionStats.TotalSessions,
				MaxAllowed: sessionStats.MaxSessions,
			}
		}
	}

	// Get resource stats
	if m.resourceStore != nil {
		stats.Resources = ResourceStats{
			Count:       m.resourceStore.GetResourceCount(),
			LastCleanup: m.resourceStore.GetLastCleanupTime(),
		}
	}

	// Get memory stats
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	stats.Memory = MemoryStats{
		Allocated:      memStats.Alloc / 1024 / 1024,
		TotalAllocated: memStats.TotalAlloc / 1024 / 1024,
		System:         memStats.Sys / 1024 / 1024,
		GCCount:        memStats.NumGC,
	}

	return stats, nil
}

// RegisterDiagnosticTools registers diagnostic tools with the transport
func (m *monitorImpl) RegisterDiagnosticTools(transport transport.MCPTransport) error {
	m.logger.Info("Registering diagnostic tools")

	// Register ping tool
	pingTool := mcp.Tool{
		Name:        "ping",
		Description: "Simple ping tool to test MCP connectivity",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"message": map[string]interface{}{
					"type":        "string",
					"description": "Optional message to echo back",
				},
			},
		},
	}

	err := transport.RegisterTool(pingTool, m.handlePing)
	if err != nil {
		return fmt.Errorf("failed to register ping tool: %w", err)
	}
	m.registeredTools = append(m.registeredTools, "ping")

	// Register server status tool
	statusTool := mcp.Tool{
		Name:        "server_status",
		Description: "Get comprehensive server status information",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"details": map[string]interface{}{
					"type":        "boolean",
					"description": "Include detailed information",
				},
			},
		},
	}

	err = transport.RegisterTool(statusTool, m.handleServerStatus)
	if err != nil {
		return fmt.Errorf("failed to register status tool: %w", err)
	}
	m.registeredTools = append(m.registeredTools, "server_status")

	m.logger.Info("Diagnostic tools registered successfully", "count", len(m.registeredTools))
	return nil
}

// handlePing handles the ping diagnostic tool
func (m *monitorImpl) handlePing(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	arguments := req.GetArguments()
	message, _ := arguments["message"].(string)

	response := "pong"
	if message != "" {
		response = "pong: " + message
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf(`{"response":"%s","timestamp":"%s"}`, response, time.Now().Format(time.RFC3339)),
			},
		},
	}, nil
}

// handleServerStatus handles the server status diagnostic tool
func (m *monitorImpl) handleServerStatus(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	arguments := req.GetArguments()
	details, _ := arguments["details"].(bool)

	stats, err := m.GetStats()
	if err != nil {
		return nil, fmt.Errorf("failed to get server stats: %w", err)
	}

	// Create response based on detail level
	var response interface{}
	if details {
		response = stats
	} else {
		// Simplified response
		response = map[string]interface{}{
			"status":   stats.State,
			"uptime":   stats.Uptime.String(),
			"sessions": fmt.Sprintf("%d/%d", stats.Sessions.Active, stats.Sessions.MaxAllowed),
		}
	}

	// Convert to JSON string for response
	responseText := fmt.Sprintf("%+v", response)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: responseText,
			},
		},
	}, nil
}
