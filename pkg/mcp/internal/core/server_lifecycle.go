package core

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/constants"
	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/internal/utils"
)

// Start starts the MCP server
func (s *Server) Start(ctx context.Context) error {
	s.logger.Info().
		Str("transport", s.config.TransportType).
		Str("workspace_dir", s.config.WorkspaceDir).
		Int("max_sessions", s.config.MaxSessions).
		Msg("Starting Container Kit MCP Server")

	// Start session cleanup routine
	s.sessionManager.StartCleanupRoutine()

	// Initialize and configure gomcp server
	if s.gomcpManager == nil {
		return fmt.Errorf("gomcp manager is nil - server initialization failed")
	}
	if err := s.gomcpManager.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize gomcp manager: %w", err)
	}

	// Set the tool orchestrator reference
	s.gomcpManager.SetToolOrchestrator(s.toolOrchestrator)

	// Register all tools with gomcp
	if err := s.gomcpManager.RegisterTools(s); err != nil {
		return fmt.Errorf("failed to register tools with gomcp: %w", err)
	}

	// If using HTTP transport, register HTTP handlers
	if err := s.gomcpManager.RegisterHTTPHandlers(s.transport); err != nil {
		return fmt.Errorf("failed to register HTTP handlers: %w", err)
	}

	// Set the server as the request handler for the transport
	if setter, ok := s.transport.(interface{ SetHandler(interface{}) }); ok {
		setter.SetHandler(s)
	}

	// Start transport serving
	transportDone := make(chan error, 1)
	go func() {
		// Start transport - use gomcp manager since transport doesn't have Serve method
		transportDone <- s.gomcpManager.StartServer()
	}()

	// Wait for context cancellation or transport error
	select {
	case err := <-transportDone:
		if err != nil {
			s.logger.Error().Err(err).Msg("Transport error")
			return err
		}
		return nil
	case <-ctx.Done():
		s.logger.Info().Msg("Context cancelled")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), constants.ShutdownTimeout)
		defer cancel()
		return s.Shutdown(shutdownCtx)
	}
}

// HandleRequest implements the LocalRequestHandler interface
func (s *Server) HandleRequest(ctx context.Context, req *core.MCPRequest) (*core.MCPResponse, error) {
	// This is handled by the underlying MCP library for stdio transport
	// For HTTP transport, we would implement custom request routing here
	return &core.MCPResponse{
		ID: req.ID,
		Error: &core.MCPError{
			Code:    -32601,
			Message: "direct request handling not implemented",
		},
	}, nil
}

// Stop gracefully stops the MCP server
func (s *Server) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), constants.ShutdownTimeout)
	defer cancel()
	return s.Shutdown(ctx)
}

// shutdown gracefully shuts down the server
func (s *Server) shutdown() error {
	s.shutdownMutex.Lock()
	defer s.shutdownMutex.Unlock()

	// Check if already shutting down to prevent concurrent shutdown calls
	if s.isShuttingDown {
		s.logger.Debug().Msg("Server already shutting down")
		return nil
	}

	s.isShuttingDown = true
	s.logger.Info().Msg("Starting graceful shutdown of MCP server")

	var shutdownErrors []error

	// Step 1: Stop accepting new requests (transport specific)
	s.logger.Info().Msg("Stopping transport from accepting new requests")
	// Transport-specific stop handling is done via the unified Close() method

	// Step 2: Wait for in-flight requests to complete (with timeout)
	s.logger.Info().Msg("Waiting for in-flight requests to complete")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), constants.ShutdownTimeout)
	defer cancel()

	// Check if job manager has active jobs
	if s.jobManager != nil {
		jobStats := s.jobManager.GetStats()
		activeJobs := jobStats.PendingJobs + jobStats.RunningJobs
		if activeJobs > 0 {
			s.logger.Info().
				Int("pending_jobs", jobStats.PendingJobs).
				Int("running_jobs", jobStats.RunningJobs).
				Msg("Waiting for active jobs to complete")

			// Wait for jobs to complete or timeout
			ticker := time.NewTicker(500 * time.Millisecond)
			defer ticker.Stop()

			for {
				select {
				case <-shutdownCtx.Done():
					jobStats = s.jobManager.GetStats()
					remainingJobs := jobStats.PendingJobs + jobStats.RunningJobs
					s.logger.Warn().Int("remaining_jobs", remainingJobs).Msg("Timeout waiting for jobs to complete")
					goto CONTINUE_SHUTDOWN
				case <-ticker.C:
					jobStats = s.jobManager.GetStats()
					activeJobs = jobStats.PendingJobs + jobStats.RunningJobs
					if activeJobs == 0 {
						s.logger.Info().Msg("All jobs completed")
						goto CONTINUE_SHUTDOWN
					}
				}
			}
		}
	}

CONTINUE_SHUTDOWN:
	// Step 3: Persist in-flight session data
	s.logger.Info().Msg("Persisting in-flight session data")
	if err := s.persistInFlightSessions(); err != nil {
		s.logger.Error().Err(err).Msg("Error persisting in-flight sessions")
		shutdownErrors = append(shutdownErrors, fmt.Errorf("persist sessions: %w", err))
	}

	// Step 4: Export telemetry metrics on shutdown (if enabled)
	if s.conversationComponents != nil && s.conversationComponents.Telemetry != nil {
		s.logger.Info().Msg("Exporting final telemetry metrics")
		if metrics, err := s.conversationComponents.Telemetry.ExportMetrics(); err == nil {
			// Log a sample of the metrics
			lines := strings.Split(metrics, "\n")
			if len(lines) > 5 {
				s.logger.Info().Str("sample_metrics", strings.Join(lines[:5], "\n")).Msg("Final telemetry snapshot")
			}
		}
	}

	// Step 5: Shutdown conversation components if enabled
	if s.conversationComponents != nil {
		s.logger.Info().Msg("Shutting down conversation components")
		if err := s.ShutdownConversation(); err != nil {
			s.logger.Error().Err(err).Msg("Error shutting down conversation components")
			shutdownErrors = append(shutdownErrors, fmt.Errorf("conversation shutdown: %w", err))
		}
	}

	// Step 6: Stop job manager
	if s.jobManager != nil {
		s.logger.Info().Msg("Stopping job manager")
		s.jobManager.Stop()
	}

	// Step 7: Stop session manager (includes final garbage collection)
	s.logger.Info().Msg("Stopping session manager")
	if err := s.sessionManager.Stop(); err != nil {
		s.logger.Error().Err(err).Msg("Error stopping session manager")
		shutdownErrors = append(shutdownErrors, fmt.Errorf("session manager stop: %w", err))
	}

	// Step 8: Export final logs if log capture is enabled
	if logBuffer := utils.GetGlobalLogBuffer(); logBuffer != nil {
		s.logger.Info().Int("log_count", logBuffer.Size()).Msg("Final log buffer statistics")
	}

	// Step 9: Stop transport
	s.logger.Info().Msg("Stopping transport")
	if stopper, ok := s.transport.(interface{ Stop(context.Context) error }); ok {
		if err := stopper.Stop(context.Background()); err != nil {
			s.logger.Error().Err(err).Msg("Error stopping transport")
			shutdownErrors = append(shutdownErrors, fmt.Errorf("transport stop: %w", err))
		}
	}

	// Step 10: Shutdown OpenTelemetry provider
	if s.otelProvider != nil && s.otelProvider.IsInitialized() {
		s.logger.Info().Msg("Shutting down OpenTelemetry provider")
		otelCtx, otelCancel := context.WithTimeout(context.Background(), constants.ContextTimeout)
		defer otelCancel()

		if err := s.otelProvider.Shutdown(otelCtx); err != nil {
			s.logger.Error().Err(err).Msg("Error shutting down OpenTelemetry provider")
			shutdownErrors = append(shutdownErrors, fmt.Errorf("otel shutdown: %w", err))
		} else {
			s.logger.Info().Msg("OpenTelemetry provider shutdown successfully")
		}
	}

	// Step 11: Final cleanup
	s.logger.Info().Msg("Performing final cleanup")

	// Combine all errors if any occurred
	if len(shutdownErrors) > 0 {
		s.logger.Error().Int("error_count", len(shutdownErrors)).Msg("Shutdown completed with errors")
		return fmt.Errorf("shutdown completed with %d errors: %v", len(shutdownErrors), shutdownErrors)
	}

	s.logger.Info().Dur("uptime", time.Since(s.startTime)).Msg("MCP server shutdown complete")
	return nil
}

// persistInFlightSessions ensures all active session data is persisted
func (s *Server) persistInFlightSessions() error {
	stats := s.sessionManager.GetStats()
	s.logger.Info().
		Int("total_sessions", stats.TotalSessions).
		Int("active_sessions", stats.ActiveSessions).
		Int("sessions_with_jobs", stats.SessionsWithJobs).
		Msg("Persisting active sessions")

	// The session manager already persists sessions automatically,
	// but we can force a final update to ensure everything is saved
	// Note: We don't have a method to list all sessions, but the session manager
	// automatically persists on every update, so this is mainly to log the final state

	return nil
}
