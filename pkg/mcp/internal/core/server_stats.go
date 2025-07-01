package core

import (
	"time"

	coreinterfaces "github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/internal/orchestration"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/Azure/container-kit/pkg/mcp/internal/utils"
)

// ServerStats provides comprehensive server statistics
type ServerStats struct {
	Uptime          time.Duration                                 `json:"uptime"`
	Sessions        *coreinterfaces.SessionManagerStats           `json:"sessions"`
	Workspace       *utils.WorkspaceStats                         `json:"workspace"`
	CircuitBreakers map[string]*orchestration.CircuitBreakerStats `json:"circuit_breakers"`
	Transport       string                                        `json:"transport"`
}

// GetStats returns server statistics
func (s *Server) GetStats() *coreinterfaces.ServerStats {
	sessionStats := s.sessionManager.GetStats()
	workspaceStats := s.workspaceManager.GetStats()

	return &coreinterfaces.ServerStats{
		Transport: s.config.TransportType,
		Sessions: &coreinterfaces.SessionManagerStats{
			ActiveSessions:    sessionStats.ActiveSessions,
			TotalSessions:     sessionStats.TotalSessions,
			FailedSessions:    sessionStats.FailedSessions,
			ExpiredSessions:   sessionStats.ExpiredSessions,
			SessionsWithJobs:  sessionStats.SessionsWithJobs,
			AverageSessionAge: sessionStats.AverageSessionAge,
			SessionErrors:     sessionStats.SessionErrors,
			TotalDiskUsage:    sessionStats.TotalDiskUsage,
			ServerStartTime:   sessionStats.ServerStartTime,
		},
		Workspace: &coreinterfaces.WorkspaceStats{
			TotalDiskUsage: workspaceStats.TotalDiskUsage,
			SessionCount:   workspaceStats.TotalSessions,
			TotalFiles:     0, // Not available in utils.WorkspaceStats
			DiskLimit:      workspaceStats.TotalDiskLimit,
		},
		Uptime:    time.Since(s.startTime),
		StartTime: s.startTime,
	}
}

// GetWorkspaceStats returns workspace statistics
func (s *Server) GetWorkspaceStats() *coreinterfaces.WorkspaceStats {
	stats := s.workspaceManager.GetStats()
	return &coreinterfaces.WorkspaceStats{
		TotalDiskUsage: stats.TotalDiskUsage,
		SessionCount:   stats.TotalSessions,
		TotalFiles:     0, // Not available in utils.WorkspaceStats
		DiskLimit:      stats.TotalDiskLimit,
	}
}

// GetSessionManagerStats returns session manager statistics
func (s *Server) GetSessionManagerStats() *coreinterfaces.SessionManagerStats {
	stats := s.sessionManager.GetStats()
	return &coreinterfaces.SessionManagerStats{
		ActiveSessions:    stats.ActiveSessions,
		TotalSessions:     stats.TotalSessions,
		FailedSessions:    stats.FailedSessions,
		ExpiredSessions:   stats.ExpiredSessions,
		SessionsWithJobs:  stats.SessionsWithJobs,
		AverageSessionAge: stats.AverageSessionAge,
		SessionErrors:     stats.SessionErrors,
		TotalDiskUsage:    stats.TotalDiskUsage,
		ServerStartTime:   stats.ServerStartTime,
	}
}

// GetCircuitBreakerStats returns circuit breaker statistics
func (s *Server) GetCircuitBreakerStats() map[string]types.CircuitBreakerStats {
	if s.circuitBreakers == nil {
		return nil
	}

	stats := s.circuitBreakers.GetStats()
	result := make(map[string]types.CircuitBreakerStats)
	for name, stat := range stats {
		result[name] = types.CircuitBreakerStats{
			State:        stat.State,
			FailureCount: stat.FailureCount,
			SuccessCount: int64(stat.SuccessCount),
			LastFailure:  &stat.LastFailure,
		}
	}
	return result
}

// GetConfig returns server configuration
func (s *Server) GetConfig() types.ServerConfig {
	return types.ServerConfig{
		TotalDiskLimit: s.config.TotalDiskLimit,
	}
}

// GetStartTime returns server start time
func (s *Server) GetStartTime() time.Time {
	return s.startTime
}

// GetConversationAdapter returns the conversation handler if conversation mode is enabled
func (s *Server) GetConversationAdapter() interface{} {
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
