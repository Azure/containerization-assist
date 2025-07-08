package session

import (
	"context"
	"fmt"
	"time"

	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/errors"
	"github.com/Azure/container-kit/pkg/mcp/errors/codes"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
)

// DeleteSessionArgs represents the arguments for deleting a session
type DeleteSessionArgs struct {
	core.BaseToolArgs
	Force           bool `json:"force,omitempty" jsonschema:"description=Force deletion even if jobs are running"`
	DeleteWorkspace bool `json:"delete_workspace,omitempty" jsonschema:"description=Also delete the workspace directory"`
}

// GetSessionID implements the api.ToolParams interface
func (args DeleteSessionArgs) GetSessionID() string {
	return args.SessionID
}

// DeleteSessionResult represents the result of deleting a session
type DeleteSessionResult struct {
	core.BaseToolResponse
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
	GetSession(sessionID string) (*SessionState, error)
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
	logger           *slog.Logger
	sessionManager   SessionDeleter
	workspaceManager WorkspaceDeleter
}

// NewDeleteSessionTool creates a new delete session tool
func NewDeleteSessionTool(logger *slog.Logger, sessionManager SessionDeleter, workspaceManager WorkspaceDeleter) *DeleteSessionTool {
	return &DeleteSessionTool{
		logger:           logger,
		sessionManager:   sessionManager,
		workspaceManager: workspaceManager,
	}
}

// Execute implements the unified Tool interface
func (t *DeleteSessionTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	// Type assertion to get proper args
	deleteArgs, ok := args.(DeleteSessionArgs)
	if !ok {
		return nil, errors.NewError().
			Code(codes.VALIDATION_FAILED).
			Messagef("Invalid arguments type: expected DeleteSessionArgs, got %T", args).
			Context("expected", "DeleteSessionArgs").
			Context("received", fmt.Sprintf("%T", args)).
			Build()
	}

	return t.ExecuteTyped(ctx, deleteArgs)
}

// ExecuteTyped provides typed execution for backward compatibility
func (t *DeleteSessionTool) ExecuteTyped(ctx context.Context, args DeleteSessionArgs) (*DeleteSessionResult, error) {
	t.logger.Info("Deleting session",
		"session_id", args.SessionID,
		"force", args.Force,
		"delete_workspace", args.DeleteWorkspace)

	// Validate session ID
	if args.SessionID == "" {
		return nil, errors.NewError().
			Code(codes.VALIDATION_REQUIRED_MISSING).
			Message("Session ID is required").
			Context("field", "session_id").
			Build()
	}

	// Check if session exists
	session, err := t.sessionManager.GetSession(args.SessionID)
	if err != nil {
		systemErr := errors.SystemError(
			codes.SYSTEM_ERROR,
			"Failed to get session",
			err,
		)
		systemErr.Context["session_id"] = args.SessionID
		return nil, systemErr
	}
	if session == nil {
		return &DeleteSessionResult{
			BaseToolResponse: core.NewToolResponse("delete_session", args.SessionID, args.DryRun),
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
				BaseToolResponse: core.NewToolResponse("delete_session", args.SessionID, args.DryRun),
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
			t.logger.Warn("Failed to cancel some jobs", "error", err)
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
		systemErr := errors.SystemError(
			codes.SYSTEM_ERROR,
			"Failed to delete session",
			err,
		)
		systemErr.Context["session_id"] = args.SessionID
		return nil, systemErr
	}

	// Delete workspace if requested
	workspaceDeleted := false
	if args.DeleteWorkspace {
		if err := t.workspaceManager.DeleteWorkspace(args.SessionID); err != nil {
			t.logger.Warn("Failed to delete workspace",
				"error", err,
				"session_id", args.SessionID)
		} else {
			workspaceDeleted = true
		}
	}

	result := &DeleteSessionResult{
		BaseToolResponse: core.NewToolResponse("delete_session", args.SessionID, args.DryRun),
		SessionID:        args.SessionID,
		Deleted:          true,
		WorkspaceDeleted: workspaceDeleted,
		JobsCancelled:    cancelledJobs,
		DiskReclaimed:    diskReclaimed,
		Message:          fmt.Sprintf("Session %s deleted successfully", args.SessionID),
	}

	t.logger.Info("Session deleted successfully",
		"session_id", args.SessionID,
		"workspace_deleted", workspaceDeleted,
		"disk_reclaimed", diskReclaimed,
		"jobs_cancelled", len(cancelledJobs))

	return result, nil
}

// GetMetadata returns comprehensive metadata about the delete session tool
func (t *DeleteSessionTool) GetMetadata() api.ToolMetadata {
	return api.ToolMetadata{
		Name:         "delete_session",
		Description:  "Delete a session and optionally its workspace with safety checks",
		Version:      "1.0.0",
		Category:     api.ToolCategory("Session Management"),
		Status:       api.ToolStatus("active"),
		Tags:         []string{"session", "cleanup", "management"},
		RegisteredAt: time.Now(),
		LastModified: time.Now(),
		Dependencies: []string{
			"Session Manager",
			"Workspace Manager",
			"Job Manager",
		},
		Capabilities: []string{
			"Session deletion",
			"Workspace cleanup",
			"Job cancellation",
			"Force deletion",
			"Disk space reclamation",
			"Safety validation",
		},
		Requirements: []string{
			"Valid session ID",
			"Session manager access",
			"Workspace manager access",
		},
	}
}

// Validate checks if the provided arguments are valid for the delete session tool
func (t *DeleteSessionTool) Validate(ctx context.Context, args interface{}) error {
	deleteArgs, ok := args.(DeleteSessionArgs)
	if !ok {
		return errors.NewError().
			Code(codes.VALIDATION_FAILED).
			Messagef("Invalid arguments type: expected DeleteSessionArgs, got %T", args).
			Context("expected", "DeleteSessionArgs").
			Context("received", fmt.Sprintf("%T", args)).
			Build()
	}

	// Validate required fields
	if deleteArgs.SessionID == "" {
		return errors.NewError().
			Code(codes.VALIDATION_REQUIRED_MISSING).
			Message("Session ID is required and cannot be empty").
			Context("field", "session_id").
			Build()
	}

	// Validate session ID format
	if len(deleteArgs.SessionID) < 3 || len(deleteArgs.SessionID) > 100 {
		return errors.NewError().
			Code(codes.VALIDATION_RANGE_INVALID).
			Message("Session ID must be between 3 and 100 characters").
			Context("field", "session_id").
			Context("min_length", 3).
			Context("max_length", 100).
			Build()
	}

	// Validate managers are available
	if t.sessionManager == nil {
		systemErr := errors.SystemError(
			codes.SYSTEM_ERROR,
			"Session manager is not configured",
			nil,
		)
		systemErr.Suggestions = append(systemErr.Suggestions, "Initialize session manager before use")
		return systemErr
	}

	if t.workspaceManager == nil && deleteArgs.DeleteWorkspace {
		return errors.NewError().
			Code(codes.CONFIG_INVALID).
			Message("Workspace manager is not configured but delete_workspace is requested").
			Context("delete_workspace", true).
			Suggestion("Either configure workspace manager or set delete_workspace to false").
			Build()
	}

	return nil
}
