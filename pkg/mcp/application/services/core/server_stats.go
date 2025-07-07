package core

import (
	"context"
	"os"
	"time"

	coreinterfaces "github.com/Azure/container-kit/pkg/mcp/application/core"
	"github.com/Azure/container-kit/pkg/mcp/application/orchestration/execution"
	"github.com/Azure/container-kit/pkg/mcp/domain/types/config"
	"github.com/Azure/container-kit/pkg/mcp/infra/runtime"
)

// ServerStats provides comprehensive server statistics
type ServerStats struct {
	Uptime          time.Duration                             `json:"uptime"`
	Sessions        *coreinterfaces.SessionManagerStats       `json:"sessions"`
	Workspace       *runtime.WorkspaceStats                   `json:"workspace"`
	CircuitBreakers map[string]*execution.CircuitBreakerStats `json:"circuit_breakers"`
	Transport       string                                    `json:"transport"`
}

// GetStatsWithContext returns server statistics with context
func (s *Server) GetStatsWithContext(ctx context.Context) (*coreinterfaces.ServerStats, error) {
	sessionStats, err := s.sessionManager.GetStats(ctx)
	if err != nil {
		s.logger.Error("Failed to get session stats", "error", err)
		return nil, err
	}

	var workspaceStats *coreinterfaces.WorkspaceStats
	if s.workspaceManager != nil {
		ctx := context.Background()
		if err := s.refreshWorkspaceDiskUsage(ctx); err != nil {
			s.logger.Warn("Failed to refresh workspace disk usage", "error", err)
		}

		wsStats := s.workspaceManager.GetStats()
		workspaceStats = &coreinterfaces.WorkspaceStats{
			TotalDiskUsage: wsStats.TotalDiskUsage,
			SessionCount:   wsStats.TotalSessions,
			TotalFiles:     0, // Not available in simplified WorkspaceStats
			DiskLimit:      wsStats.TotalDiskLimit,
		}
	} else {
		workspaceStats = &coreinterfaces.WorkspaceStats{
			TotalDiskUsage: 0,
			SessionCount:   0,
			TotalFiles:     0,
			DiskLimit:      0,
		}
	}

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
		Workspace: workspaceStats,
		Uptime:    time.Since(s.startTime),
		StartTime: s.startTime,
	}, nil
}

// GetWorkspaceStats returns workspace statistics
func (s *Server) GetWorkspaceStats() *coreinterfaces.WorkspaceStats {
	if s.workspaceManager != nil {
		ctx := context.Background()
		if err := s.refreshWorkspaceDiskUsage(ctx); err != nil {
			s.logger.Warn("Failed to refresh workspace disk usage", "error", err)
		}

		wsStats := s.workspaceManager.GetStats()
		return &coreinterfaces.WorkspaceStats{
			TotalDiskUsage: wsStats.TotalDiskUsage,
			SessionCount:   wsStats.TotalSessions,
			TotalFiles:     0, // Not available in simplified WorkspaceStats
			DiskLimit:      wsStats.TotalDiskLimit,
		}
	}

	return &coreinterfaces.WorkspaceStats{
		TotalDiskUsage: 0,
		SessionCount:   0,
		TotalFiles:     0,
		DiskLimit:      0,
	}
}

// GetSessionManagerStats returns session manager statistics
func (s *Server) GetSessionManagerStats() *coreinterfaces.SessionManagerStats {
	ctx := context.Background()
	stats, err := s.sessionManager.GetStats(ctx)
	if err != nil {
		s.logger.Error("Failed to get session stats", "error", err)
		return &coreinterfaces.SessionManagerStats{}
	}
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
func (s *Server) GetCircuitBreakerStats() map[string]config.CircuitBreakerStats {
	if s.circuitBreakers == nil {
		return nil
	}

	stats := s.circuitBreakers.GetStats()
	result := make(map[string]config.CircuitBreakerStats)
	for name, stat := range stats {
		result[name] = config.CircuitBreakerStats{
			Name:           name,
			State:          stat.State,
			TotalRequests:  int64(stat.SuccessCount + stat.FailureCount),
			FailedRequests: int64(stat.FailureCount),
			SuccessRate:    float64(stat.SuccessCount) / float64(stat.SuccessCount+stat.FailureCount),
			LastFailure:    stat.LastFailure,
		}
	}
	return result
}

// GetConfig returns server configuration
func (s *Server) GetConfig() config.ServerConfig {
	return config.ServerConfig{
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

// refreshWorkspaceDiskUsage scans all session directories and updates disk usage
func (s *Server) refreshWorkspaceDiskUsage(ctx context.Context) error {
	entries, err := os.ReadDir(s.config.WorkspaceDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			sessionID := entry.Name()
			if err := s.workspaceManager.UpdateDiskUsage(ctx, sessionID); err != nil {
				s.logger.Warn("Failed to update disk usage for session",
					"error", err,
					"session_id", sessionID)
			}
		}
	}

	return nil
}
