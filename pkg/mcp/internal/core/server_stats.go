package core

import (
	"context"
	"os"
	"time"

	coreinterfaces "github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/core/orchestration"
	"github.com/Azure/container-kit/pkg/mcp/internal/common/utils"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
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

	// Get workspace stats if workspace manager is available
	var workspaceStats *coreinterfaces.WorkspaceStats
	if s.workspaceManager != nil {
		// Refresh disk usage by scanning actual workspace directories
		ctx := context.Background()
		if err := s.refreshWorkspaceDiskUsage(ctx); err != nil {
			s.logger.Warn().Err(err).Msg("Failed to refresh workspace disk usage")
		}

		wsStats := s.workspaceManager.GetStats()
		workspaceStats = &coreinterfaces.WorkspaceStats{
			TotalDiskUsage: wsStats.TotalDiskUsage,
			SessionCount:   wsStats.TotalSessions,
			TotalFiles:     0, // Not available in simplified WorkspaceStats
			DiskLimit:      wsStats.TotalDiskLimit,
		}
	} else {
		// Fallback to zeros if workspace manager not available
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
	}
}

// GetWorkspaceStats returns workspace statistics
func (s *Server) GetWorkspaceStats() *coreinterfaces.WorkspaceStats {
	if s.workspaceManager != nil {
		// Refresh disk usage by scanning actual workspace directories
		ctx := context.Background()
		if err := s.refreshWorkspaceDiskUsage(ctx); err != nil {
			s.logger.Warn().Err(err).Msg("Failed to refresh workspace disk usage")
		}

		wsStats := s.workspaceManager.GetStats()
		return &coreinterfaces.WorkspaceStats{
			TotalDiskUsage: wsStats.TotalDiskUsage,
			SessionCount:   wsStats.TotalSessions,
			TotalFiles:     0, // Not available in simplified WorkspaceStats
			DiskLimit:      wsStats.TotalDiskLimit,
		}
	}

	// Fallback to zeros if workspace manager not available
	return &coreinterfaces.WorkspaceStats{
		TotalDiskUsage: 0,
		SessionCount:   0,
		TotalFiles:     0,
		DiskLimit:      0,
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

// refreshWorkspaceDiskUsage scans all session directories and updates disk usage
func (s *Server) refreshWorkspaceDiskUsage(ctx context.Context) error {
	// Get all session directories
	entries, err := os.ReadDir(s.config.WorkspaceDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Workspace directory doesn't exist yet
		}
		return err
	}

	// Update disk usage for each session directory
	for _, entry := range entries {
		if entry.IsDir() {
			sessionID := entry.Name()
			if err := s.workspaceManager.UpdateDiskUsage(ctx, sessionID); err != nil {
				s.logger.Warn().
					Err(err).
					Str("session_id", sessionID).
					Msg("Failed to update disk usage for session")
			}
		}
	}

	return nil
}
