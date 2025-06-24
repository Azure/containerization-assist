package core

import (
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/adapter"
	"github.com/Azure/container-copilot/pkg/mcp/internal/orchestration"
	"github.com/Azure/container-copilot/pkg/mcp/internal/store"
	"github.com/Azure/container-copilot/pkg/mcp/internal/store/session"
	"github.com/Azure/container-copilot/pkg/mcp/internal/tools"
)

// ServerStats provides comprehensive server statistics
type ServerStats struct {
	Uptime          time.Duration                                 `json:"uptime"`
	Sessions        *session.SessionManagerStats                  `json:"sessions"`
	Workspace       *store.WorkspaceStats                         `json:"workspace"`
	CircuitBreakers map[string]*orchestration.CircuitBreakerStats `json:"circuit_breakers"`
	Transport       string                                        `json:"transport"`
}

// GetStats returns server statistics
func (s *Server) GetStats() *ServerStats {
	sessionStats := s.sessionManager.GetStats()
	workspaceStats := s.workspaceManager.GetStats()
	circuitStats := s.circuitBreakers.GetStats()

	return &ServerStats{
		Uptime:          time.Since(s.startTime),
		Sessions:        sessionStats,
		Workspace:       workspaceStats,
		CircuitBreakers: circuitStats,
		Transport:       s.transport.Name(),
	}
}

// GetWorkspaceStats returns workspace statistics
func (s *Server) GetWorkspaceStats() adapter.WorkspaceStats {
	stats := s.workspaceManager.GetStats()
	return adapter.WorkspaceStats{
		TotalDiskUsage: stats.TotalDiskUsage,
		SessionCount:   stats.TotalSessions,
	}
}

// GetSessionManagerStats returns session manager statistics
func (s *Server) GetSessionManagerStats() adapter.SessionManagerStats {
	stats := s.sessionManager.GetStats()
	return adapter.SessionManagerStats{
		ActiveSessions: stats.ActiveSessions,
		TotalSessions:  stats.TotalSessions,
	}
}

// GetCircuitBreakerStats returns circuit breaker statistics
func (s *Server) GetCircuitBreakerStats() map[string]adapter.CircuitBreakerStats {
	if s.circuitBreakers == nil {
		return nil
	}

	stats := s.circuitBreakers.GetStats()
	result := make(map[string]adapter.CircuitBreakerStats)
	for name, stat := range stats {
		result[name] = adapter.CircuitBreakerStats{
			State:        stat.State,
			FailureCount: stat.FailureCount,
			SuccessCount: stat.SuccessCount,
			LastFailure:  &stat.LastFailure,
		}
	}
	return result
}

// GetConfig returns server configuration
func (s *Server) GetConfig() adapter.ServerConfig {
	return adapter.ServerConfig{
		TotalDiskLimit: s.config.TotalDiskLimit,
	}
}

// GetStartTime returns server start time
func (s *Server) GetStartTime() time.Time {
	return s.startTime
}

// GetConversationAdapter returns the conversation handler if conversation mode is enabled
func (s *Server) GetConversationAdapter() tools.ConversationOperations {
	if s.conversationComponents != nil && s.conversationComponents.Handler != nil {
		return s.conversationComponents.Handler
	}
	return nil
}

// GetTelemetry returns the telemetry manager if enabled
func (s *Server) GetTelemetry() interface{} {
	if s.conversationComponents != nil {
		return s.conversationComponents.Telemetry
	}
	return nil
}
