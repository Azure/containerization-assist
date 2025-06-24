package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	"github.com/rs/zerolog"
)

// DeleteSessionArgs represents the arguments for deleting a session
type DeleteSessionArgs struct {
	types.BaseToolArgs
	SessionID       string `json:"session_id" jsonschema:"required,description=The session ID to delete"`
	Force           bool   `json:"force,omitempty" jsonschema:"description=Force deletion even if jobs are running"`
	DeleteWorkspace bool   `json:"delete_workspace,omitempty" jsonschema:"description=Also delete the workspace directory"`
}

// DeleteSessionResult represents the result of deleting a session
type DeleteSessionResult struct {
	types.BaseToolResponse
	SessionID        string           `json:"session_id"`
	Deleted          bool             `json:"deleted"`
	WorkspaceDeleted bool             `json:"workspace_deleted"`
	JobsCancelled    []string         `json:"jobs_cancelled,omitempty"`
	DiskReclaimed    int64            `json:"disk_reclaimed_bytes"`
	Message          string           `json:"message"`
	Error            *types.ToolError `json:"error,omitempty"`
}

// SessionDeleter interface for session deletion operations
type SessionDeleter interface {
	GetSession(sessionID string) (*SessionData, error)
	DeleteSession(sessionID string) error
	CancelSessionJobs(sessionID string) ([]string, error)
}

// WorkspaceDeleter interface for workspace deletion
type WorkspaceDeleter interface {
	GetWorkspacePath(sessionID string) string
	DeleteWorkspace(sessionID string) error
	GetWorkspaceSize(sessionID string) (int64, error)
}

// DeleteSessionTool implements the delete_session MCP tool
type DeleteSessionTool struct {
	logger           zerolog.Logger
	sessionManager   SessionDeleter
	workspaceManager WorkspaceDeleter
}

// NewDeleteSessionTool creates a new delete session tool
func NewDeleteSessionTool(logger zerolog.Logger, sessionManager SessionDeleter, workspaceManager WorkspaceDeleter) *DeleteSessionTool {
	return &DeleteSessionTool{
		logger:           logger,
		sessionManager:   sessionManager,
		workspaceManager: workspaceManager,
	}
}

// Execute deletes a session and optionally its workspace
func (t *DeleteSessionTool) Execute(ctx context.Context, args DeleteSessionArgs) (*DeleteSessionResult, error) {
	t.logger.Info().
		Str("session_id", args.SessionID).
		Bool("force", args.Force).
		Bool("delete_workspace", args.DeleteWorkspace).
		Msg("Deleting session")

	// Validate session ID
	if args.SessionID == "" {
		return nil, types.NewRichError("INVALID_ARGUMENTS", "session_id is required", "validation_error")
	}

	// Check if session exists
	session, err := t.sessionManager.GetSession(args.SessionID)
	if err != nil {
		return nil, types.NewRichError("INTERNAL_SERVER_ERROR", "failed to get session: "+err.Error(), "execution_error")
	}
	if session == nil {
		return &DeleteSessionResult{
			BaseToolResponse: types.NewBaseResponse("delete_session", args.SessionID, args.DryRun),
			SessionID:        args.SessionID,
			Deleted:          false,
			Message:          "Session not found",
			Error: &types.ToolError{
				Type:      "SESSION_NOT_FOUND",
				Message:   "Session " + args.SessionID + " not found",
				Retryable: false,
				Timestamp: time.Now(),
			},
		}, nil
	}

	// Check for active jobs
	cancelledJobs := []string{}
	if len(session.ActiveJobs) > 0 {
		if !args.Force {
			return &DeleteSessionResult{
				BaseToolResponse: types.NewBaseResponse("delete_session", args.SessionID, args.DryRun),
				SessionID:        args.SessionID,
				Deleted:          false,
				Message:          fmt.Sprintf("Session has %d active jobs", len(session.ActiveJobs)),
				Error: &types.ToolError{
					Type:        "ACTIVE_JOBS",
					Message:     fmt.Sprintf("Session has %d active jobs. Use force=true to delete anyway", len(session.ActiveJobs)),
					Retryable:   true,
					Timestamp:   time.Now(),
					Suggestions: []string{"Use force=true to delete anyway", "Wait for jobs to complete"},
				},
			}, nil
		}

		// Cancel active jobs
		cancelled, err := t.sessionManager.CancelSessionJobs(args.SessionID)
		if err != nil {
			t.logger.Warn().Err(err).Msg("Failed to cancel some jobs")
		}
		cancelledJobs = cancelled
	}

	// Get workspace size before deletion
	var diskReclaimed int64
	if args.DeleteWorkspace {
		size, err := t.workspaceManager.GetWorkspaceSize(args.SessionID)
		if err == nil {
			diskReclaimed = size
		}
	}

	// Delete the session from persistence
	if err := t.sessionManager.DeleteSession(args.SessionID); err != nil {
		return nil, types.NewRichError("INTERNAL_SERVER_ERROR", "failed to delete session: "+err.Error(), "execution_error")
	}

	// Delete workspace if requested
	workspaceDeleted := false
	if args.DeleteWorkspace {
		if err := t.workspaceManager.DeleteWorkspace(args.SessionID); err != nil {
			t.logger.Warn().
				Err(err).
				Str("session_id", args.SessionID).
				Msg("Failed to delete workspace")
		} else {
			workspaceDeleted = true
		}
	}

	result := &DeleteSessionResult{
		BaseToolResponse: types.NewBaseResponse("delete_session", args.SessionID, args.DryRun),
		SessionID:        args.SessionID,
		Deleted:          true,
		WorkspaceDeleted: workspaceDeleted,
		JobsCancelled:    cancelledJobs,
		DiskReclaimed:    diskReclaimed,
		Message:          fmt.Sprintf("Session %s deleted successfully", args.SessionID),
	}

	t.logger.Info().
		Str("session_id", args.SessionID).
		Bool("workspace_deleted", workspaceDeleted).
		Int64("disk_reclaimed", diskReclaimed).
		Int("jobs_cancelled", len(cancelledJobs)).
		Msg("Session deleted successfully")

	return result, nil
}
