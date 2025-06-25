package core

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/tools"
	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
)

// Handler methods for direct GoMCP tool registration

// handleServerStatus implements the server_status tool logic
func (gm *GomcpManager) handleServerStatus(deps *ToolDependencies, args *ServerStatusArgs) (*ServerStatusResult, error) {
	// Use server health check mode if no session provided
	sessionID := args.SessionID
	if sessionID == "" {
		sessionID = "server-health-check"
	}

	// Fast path for basic health checks
	if !args.DetailedAnalysis && !args.IncludeDetails {
		return &ServerStatusResult{
			Healthy: true,
			Status:  "operational",
			Version: "1.0.0",
		}, nil
	}

	// Detailed health check using atomic tool
	healthTool := tools.NewAtomicCheckHealthTool(
		deps.PipelineAdapter,
		deps.AtomicSessionMgr,
		deps.Logger.With().Str("tool", "check_health_atomic").Logger(),
	)

	atomicArgs := tools.AtomicCheckHealthArgs{
		BaseToolArgs: types.BaseToolArgs{
			SessionID: sessionID,
			DryRun:    args.DryRun,
		},
		DetailedAnalysis: args.DetailedAnalysis || args.IncludeDetails,
	}

	stdCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resultInterface, err := healthTool.Execute(stdCtx, atomicArgs)
	if err != nil {
		// Fallback to basic server health
		sessionStats := deps.Server.sessionManager.GetStats()
		workspaceStats := deps.Server.workspaceManager.GetStats()

		return &ServerStatusResult{
			Healthy: true,
			Version: "1.0.0",
			Details: map[string]interface{}{
				"services": map[string]interface{}{
					"session_manager": map[string]interface{}{
						"healthy":         true,
						"active_sessions": sessionStats.ActiveSessions,
						"total_sessions":  sessionStats.TotalSessions,
					},
					"workspace_manager": map[string]interface{}{
						"healthy":          true,
						"total_disk_usage": workspaceStats.TotalDiskUsage,
						"total_sessions":   workspaceStats.TotalSessions,
					},
				},
				"error": fmt.Sprintf("atomic health check failed: %v", err),
			},
		}, nil
	}

	// Type assert to get the actual result
	result, ok := resultInterface.(*tools.AtomicCheckHealthResult)
	if !ok {
		return nil, fmt.Errorf("unexpected result type: %T", resultInterface)
	}

	// Convert atomic result to expected format
	return &ServerStatusResult{
		Healthy:   result.Success,
		SessionID: result.SessionID,
		Version:   "1.0.0",
		DryRun:    result.DryRun,
	}, nil
}

// handleListSessions implements the list_sessions tool logic
func (gm *GomcpManager) handleListSessions(deps *ToolDependencies, args *SessionListArgs) (*SessionListResult, error) {
	sessions := deps.Server.sessionManager.ListSessionSummaries()
	var sessionData []map[string]interface{}

	for _, session := range sessions {
		sessionInfo := map[string]interface{}{
			"session_id":    session.SessionID,
			"created_at":    session.CreatedAt,
			"last_accessed": session.LastAccessed,
			"status":        session.Status,
			"disk_usage":    session.DiskUsage,
			"active_jobs":   session.ActiveJobs,
		}

		// Include additional details if available
		if session.RepoURL != "" {
			sessionInfo["repo_url"] = session.RepoURL
		}

		sessionData = append(sessionData, sessionInfo)

		// Apply limit if specified
		if args.Limit > 0 && len(sessionData) >= args.Limit {
			break
		}
	}

	return &SessionListResult{
		Sessions: sessionData,
		Total:    len(sessions),
	}, nil
}

// handleDeleteSession implements the delete_session tool logic
func (gm *GomcpManager) handleDeleteSession(deps *ToolDependencies, args *SessionDeleteArgs) (*SessionDeleteResult, error) {
	if args.SessionID == "" {
		return &SessionDeleteResult{
			Success: false,
			Message: "session_id is required",
		}, nil
	}

	err := deps.Server.sessionManager.DeleteSession(context.Background(), args.SessionID)
	if err != nil {
		return &SessionDeleteResult{
			Success:   false,
			SessionID: args.SessionID,
			Message:   fmt.Sprintf("Failed to delete session: %v", err),
		}, nil
	}

	return &SessionDeleteResult{
		Success:   true,
		SessionID: args.SessionID,
		Message:   "Session deleted successfully",
	}, nil
}

// handleJobStatus implements the get_job_status tool logic
func (gm *GomcpManager) handleJobStatus(deps *ToolDependencies, args *JobStatusArgs) (*JobStatusResult, error) {
	if args.JobID == "" {
		return &JobStatusResult{
			JobID:  "",
			Status: "error",
			Details: map[string]interface{}{
				"error": "job_id is required",
			},
		}, nil
	}

	// Get job status from the job manager
	if deps.Server.jobManager != nil {
		job, err := deps.Server.jobManager.GetJob(args.JobID)
		if err != nil {
			return &JobStatusResult{
				JobID:  args.JobID,
				Status: "not_found",
				Details: map[string]interface{}{
					"error": fmt.Sprintf("Job not found: %v", err),
				},
			}, nil
		}

		// Convert AsyncJobInfo to JobStatusResult format
		details := map[string]interface{}{
			"type":       string(job.Type),
			"session_id": job.SessionID,
			"created_at": job.CreatedAt.Format(time.RFC3339),
			"progress":   job.Progress,
			"message":    job.Message,
		}

		if job.StartedAt != nil {
			details["started_at"] = job.StartedAt.Format(time.RFC3339)
		}
		if job.CompletedAt != nil {
			details["completed_at"] = job.CompletedAt.Format(time.RFC3339)
		}
		if job.Duration != nil {
			details["duration"] = job.Duration.String()
		}
		if job.Error != "" {
			details["error"] = job.Error
		}
		if job.Result != nil {
			details["result"] = job.Result
		}
		if len(job.Logs) > 0 {
			details["logs"] = job.Logs
		}
		if job.Metadata != nil {
			details["metadata"] = job.Metadata
		}

		return &JobStatusResult{
			JobID:   args.JobID,
			Status:  string(job.Status),
			Details: details,
		}, nil
	}

	return &JobStatusResult{
		JobID:  args.JobID,
		Status: "not_found",
		Details: map[string]interface{}{
			"message": "Job manager not available",
		},
	}, nil
}

// handleChat implements the chat tool logic
func (gm *GomcpManager) handleChat(deps *ToolDependencies, args *ChatArgs) (*ChatResult, error) {
	if args.Message == "" {
		return &ChatResult{
			Response: "Please provide a message to continue the conversation.",
		}, nil
	}

	if deps.Server.conversationComponents == nil || deps.Server.conversationComponents.Handler == nil {
		return &ChatResult{
			Response: "Conversation mode is not enabled on this server.",
		}, nil
	}

	// Use the concrete conversation handler directly
	handler := deps.Server.conversationComponents.Handler

	// Convert ChatArgs to tools.ChatToolArgs
	toolArgs := tools.ChatToolArgs{
		Message:   args.Message,
		SessionID: args.SessionID,
	}

	// Create context with timeout for conversation processing
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Call the conversation handler
	result, err := handler.HandleConversation(ctx, toolArgs)
	if err != nil {
		return &ChatResult{
			Response:  fmt.Sprintf("Failed to process conversation: %v", err),
			SessionID: args.SessionID,
		}, nil
	}

	// Convert tools.ChatToolResult back to ChatResult
	response := result.Message
	if !result.Success {
		response = fmt.Sprintf("Conversation processing failed: %s", result.Message)
	}

	// Add additional context if available
	if result.Stage != "" || result.Status != "" {
		additionalInfo := ""
		if result.Stage != "" {
			additionalInfo += fmt.Sprintf(" [Stage: %s]", result.Stage)
		}
		if result.Status != "" {
			additionalInfo += fmt.Sprintf(" [Status: %s]", result.Status)
		}
		if additionalInfo != "" {
			response += additionalInfo
		}
	}

	// Include next steps if available
	if len(result.NextSteps) > 0 {
		response += "\n\nNext steps:\n"
		for i, step := range result.NextSteps {
			response += fmt.Sprintf("%d. %s\n", i+1, step)
		}
	}

	return &ChatResult{
		Response:  response,
		SessionID: result.SessionID,
	}, nil
}
